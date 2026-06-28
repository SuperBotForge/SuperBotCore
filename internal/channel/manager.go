package channel

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"SuperBotGo/internal/errs"
	"SuperBotGo/internal/i18n"
	"SuperBotGo/internal/locale"
	"SuperBotGo/internal/metrics"
	"SuperBotGo/internal/model"
	"SuperBotGo/internal/plugin/contract"
	"SuperBotGo/internal/state"
)

type UserService interface {
	FindOrCreateUser(ctx context.Context, channelType model.ChannelType, platformUserID model.PlatformUserID, username ...string) (*model.GlobalUser, error)
	GetUser(ctx context.Context, id model.GlobalUserID) (*model.GlobalUser, error)
}

type StateManager interface {
	Register(pluginID string, def *state.CommandDefinition)
	StartCommand(ctx context.Context, userID model.GlobalUserID, chatID string, pluginID string, commandName string, locale string) (*StateResult, error)
	ProcessInput(ctx context.Context, userID model.GlobalUserID, chatID string, input model.UserInput, locale string) (*StateResult, error)
	CancelCommand(ctx context.Context, userID model.GlobalUserID) error
	IsPreservesDialog(pluginID, commandName string) bool
	GetCurrentStepMessage(ctx context.Context, userID model.GlobalUserID, locale string) (*model.Message, string, error)
}

type StateResult struct {
	PluginID    string
	Message     model.Message
	CommandName string
	IsComplete  bool
	Params      model.OptionMap
}

type PluginRegistry interface {
	GetCommandDefinition(commandName string) *state.CommandDefinition
	GetPluginIDByCommand(commandName string) string
	ResolveCommand(input string) (pluginID string, def *state.CommandDefinition, candidates []model.CommandCandidate)
}

type EventRouter interface {
	RouteEvent(ctx context.Context, event contract.Event) (*contract.EventResponse, error)
}

type Authorizer interface {
	CheckCommand(ctx context.Context, userID model.GlobalUserID, pluginID string, commandName string, requirements *model.RoleRequirements) (bool, error)
}

// FocusTracker tracks per-user last-used plugin for disambiguation sorting.
type FocusTracker interface {
	Record(userID model.GlobalUserID, pluginID string)
	LastPlugin(userID model.GlobalUserID) string
}

// ChatGroupResolver resolves cross-messenger group IDs for a given chat.
type ChatGroupResolver interface {
	FindChatGroupID(ctx context.Context, channelType model.ChannelType, platformChatID string) (int64, error)
}

// BlacklistChecker checks whether a user is currently blacklisted.
type BlacklistChecker interface {
	IsBlocked(ctx context.Context, userID int64) (bool, error)
}

type ChannelManager struct {
	userService       UserService
	router            EventRouter
	state             StateManager
	plugins           PluginRegistry
	authorizer        Authorizer
	adapters          *AdapterRegistry
	focus             FocusTracker
	chatGroupResolver ChatGroupResolver
	blacklist         BlacklistChecker
	logger            *slog.Logger
	metrics           *metrics.Metrics
	// lastBotMsgID tracks the most recently sent bot message ID per chatID so
	// stale inline keyboards can be cleared when a new command arrives.
	lastBotMsgID sync.Map // key: chatID string → value: string (platform-native message ID)
}

func NewChannelManager(
	userService UserService,
	router EventRouter,
	stateManager StateManager,
	plugins PluginRegistry,
	authorizer Authorizer,
	adapters *AdapterRegistry,
	focus FocusTracker,
	logger *slog.Logger,
) *ChannelManager {
	if logger == nil {
		logger = slog.Default()
	}
	return &ChannelManager{
		userService: userService,
		router:      router,
		state:       stateManager,
		plugins:     plugins,
		authorizer:  authorizer,
		adapters:    adapters,
		focus:       focus,
		logger:      logger,
	}
}

// SetChatGroupResolver wires up cross-messenger group ID resolution.
func (m *ChannelManager) SetChatGroupResolver(r ChatGroupResolver) {
	m.chatGroupResolver = r
}

// SetBlacklistChecker wires up the cross-plugin user blacklist.
func (m *ChannelManager) SetBlacklistChecker(b BlacklistChecker) {
	m.blacklist = b
}

func (m *ChannelManager) RegisterAdapter(adapter ChannelAdapter) {
	m.adapters.Register(adapter)
}

func (m *ChannelManager) SetMetrics(metricSet *metrics.Metrics) {
	m.metrics = metricSet
}

func (m *ChannelManager) OnUpdate(ctx context.Context, u Update) error {
	start := time.Now()
	result := "ok"
	defer func() {
		if m.metrics == nil {
			return
		}
		m.metrics.ChannelUpdateDuration.WithLabelValues(
			string(u.ChannelType),
			updateInputType(u.Input),
			result,
		).Observe(time.Since(start).Seconds())
	}()

	user, err := m.userService.FindOrCreateUser(ctx, u.ChannelType, u.PlatformUserID, u.Username)
	if err != nil {
		result = "user_lookup_error"
		return err
	}

	if m.blacklist != nil {
		blocked, err := m.blacklist.IsBlocked(ctx, int64(user.ID))
		if err != nil {
			m.logger.Warn("blacklist check failed", slog.Int64("user_id", int64(user.ID)), slog.Any("error", err))
		} else if blocked {
			result = "blacklisted"
			return nil
		}
	}

	loc := user.Locale
	if loc == "" {
		loc = locale.Default()
	}

	chatGroupID := m.resolveChatGroupID(ctx, u.ChannelType, u.ChatID)

	if err := m.processUpdate(ctx, user, u.ChannelType, u.Input, u.ChatID, chatGroupID, loc); err != nil {
		result = classifyUpdateResult(err)
		m.handleError(ctx, u.ChannelType, u.ChatID, user.ID, err)
	}
	return nil
}

func (m *ChannelManager) resolveChatGroupID(ctx context.Context, channelType model.ChannelType, chatID string) string {
	if m.chatGroupResolver == nil {
		return ""
	}
	id, err := m.chatGroupResolver.FindChatGroupID(ctx, channelType, chatID)
	if err != nil {
		m.logger.Warn("channel: resolve chat group id failed", "chat_id", chatID, "error", err)
		return ""
	}
	if id == 0 {
		return ""
	}
	return fmt.Sprintf("cg:%d", id)
}

func (m *ChannelManager) processUpdate(
	ctx context.Context,
	user *model.GlobalUser,
	channelType model.ChannelType,
	input model.UserInput,
	chatID string,
	chatGroupID string,
	locale string,
) error {
	input = m.normalizeInput(channelType, input)

	if input.IsCommand() {
		return m.handleCommand(ctx, user.ID, channelType, input, chatID, chatGroupID, locale)
	}
	return m.handleInput(ctx, user.ID, channelType, input, chatID, chatGroupID, locale)
}

func (m *ChannelManager) normalizeInput(channelType model.ChannelType, input model.UserInput) model.UserInput {
	if channelType != model.ChannelMattermost || input == nil || input.IsCommand() {
		return input
	}

	text, ok := input.(model.TextInput)
	if !ok {
		return input
	}

	trimmed := strings.TrimSpace(text.Text)
	if trimmed == "" || strings.ContainsAny(trimmed, " \t\r\n") {
		return input
	}

	_, def, candidates := m.plugins.ResolveCommand(trimmed)
	if def == nil && len(candidates) == 0 {
		return input
	}

	text.Text = "/" + trimmed
	return text
}

func (m *ChannelManager) handleCommand(
	ctx context.Context,
	userID model.GlobalUserID,
	channelType model.ChannelType,
	input model.UserInput,
	chatID string,
	chatGroupID string,
	loc string,
) error {
	rawName := input.CommandName()

	pluginID, def, candidates := m.plugins.ResolveCommand(rawName)

	// Ambiguous alias — send disambiguation message.
	if len(candidates) > 0 {
		m.incCommandExecution(channelType, pluginID, rawName, "ambiguous")
		msg := m.buildDisambiguationMessage(userID, candidates, loc)
		return m.adapters.SendToChat(ctx, channelType, chatID, msg)
	}

	// Not found.
	if def == nil {
		m.incCommandExecution(channelType, pluginID, rawName, "not_found")
		return errs.NewSilentError(errs.ErrCommandNotFound, rawName)
	}

	commandName := def.Name

	ok, err := m.authorizer.CheckCommand(ctx, userID, pluginID, commandName, def.Requirements)
	if err != nil {
		return err
	}
	if !ok {
		m.incCommandExecution(channelType, pluginID, commandName, "denied")
		return m.adapters.SendToChat(ctx, channelType, chatID,
			model.NewTextMessage(i18n.Get("error.access_denied", loc)))
	}

	if !m.state.IsPreservesDialog(pluginID, commandName) {
		_ = m.state.CancelCommand(ctx, userID)
	}

	result, err := m.state.StartCommand(ctx, userID, chatID, pluginID, commandName, loc)
	if err != nil {
		return err
	}

	if result.IsComplete {
		return m.dispatchCompletedCommand(ctx, newCompletedCommand(
			userID,
			channelType,
			chatID,
			chatGroupID,
			pluginID,
			commandName,
			result.Params,
			loc,
			input,
		))
	}

	var callbackMsgID string
	if cb, ok := input.(model.CallbackInput); ok && cb.MessageID != 0 {
		callbackMsgID = strconv.Itoa(cb.MessageID)
	}

	if callbackMsgID != "" && !result.Message.IsEmpty() {
		if editErr := m.adapters.EditMessageInChat(ctx, channelType, chatID, callbackMsgID, result.Message); editErr != nil {
			return m.adapters.SendToChat(ctx, channelType, chatID, result.Message)
		}
		return nil
	}

	return m.adapters.SendToChat(ctx, channelType, chatID, result.Message)
}

func (m *ChannelManager) handleInput(
	ctx context.Context,
	userID model.GlobalUserID,
	channelType model.ChannelType,
	input model.UserInput,
	chatID string,
	chatGroupID string,
	loc string,
) error {
	// Clear the stale keyboard from the previous bot message when the user
	// sends a fresh text input (not a button callback). This prevents users
	// from accidentally clicking buttons that belong to an earlier flow step.
	_, isCallback := input.(model.CallbackInput)
	if !isCallback {
		if v, ok := m.lastBotMsgID.Load(chatID); ok {
			_ = m.adapters.EditMessageInChat(ctx, channelType, chatID, v.(string), model.Message{})
			m.lastBotMsgID.Delete(chatID)
		}
	}

	result, err := m.state.ProcessInput(ctx, userID, chatID, input, loc)
	if err != nil {
		if m.shouldIgnoreInputError(err, input) {
			return nil
		}
		return err
	}

	var callbackMsgID string
	if cb, ok := input.(model.CallbackInput); ok && cb.MessageID != 0 {
		callbackMsgID = strconv.Itoa(cb.MessageID)
	}

	slog.Info("channel: handleInput", "callback_msg_id", callbackMsgID, "result_empty", result.Message.IsEmpty(), "complete", result.IsComplete)

	if callbackMsgID != "" && !result.Message.IsEmpty() {
		// Edit the message that contained the clicked button in place.
		// On failure (e.g. media message, Telegram API error) fall back to sending a new message.
		if editErr := m.adapters.EditMessageInChat(ctx, channelType, chatID, callbackMsgID, result.Message); editErr != nil {
			if err := m.sendResultMessage(ctx, channelType, chatID, result.Message); err != nil {
				return err
			}
		}
	} else {
		if err := m.sendResultMessage(ctx, channelType, chatID, result.Message); err != nil {
			return err
		}
	}

	if result.IsComplete {
		if callbackMsgID != "" && result.Message.IsEmpty() {
			// Only remove the keyboard if this message belongs to the current flow
			// (tracked in lastBotMsgID). Notification messages must not be deleted.
			if v, ok := m.lastBotMsgID.Load(chatID); ok && v.(string) == callbackMsgID {
				_ = m.adapters.EditMessageInChat(ctx, channelType, chatID, callbackMsgID, model.Message{})
				m.lastBotMsgID.Delete(chatID)
			}
		}
		return m.dispatchCompletedCommand(ctx, newCompletedCommand(
			userID,
			channelType,
			chatID,
			chatGroupID,
			m.resultPluginID(result),
			result.CommandName,
			result.Params,
			loc,
			input,
		))
	}

	return nil
}

type completedCommand struct {
	userID      model.GlobalUserID
	channelType model.ChannelType
	chatID      string
	chatGroupID string
	pluginID    string
	commandName string
	params      model.OptionMap
	locale      string
	files       []model.FileRef
}

func newCompletedCommand(
	userID model.GlobalUserID,
	channelType model.ChannelType,
	chatID string,
	chatGroupID string,
	pluginID string,
	commandName string,
	params model.OptionMap,
	locale string,
	input model.UserInput,
) completedCommand {
	return completedCommand{
		userID:      userID,
		channelType: channelType,
		chatID:      chatID,
		chatGroupID: chatGroupID,
		pluginID:    pluginID,
		commandName: commandName,
		params:      params,
		locale:      locale,
		files:       extractFiles(input),
	}
}

type completedCommandError struct {
	cmd completedCommand
	err error
}

func (e *completedCommandError) Error() string {
	return e.err.Error()
}

func (e *completedCommandError) Unwrap() error {
	return e.err
}

func (m *ChannelManager) dispatchCompletedCommand(ctx context.Context, cmd completedCommand) error {
	result := "ok"
	defer func() {
		m.incCommandExecution(cmd.channelType, cmd.pluginID, cmd.commandName, result)
	}()

	m.recordFocus(cmd.userID, cmd.pluginID)
	if err := m.routeCommand(ctx, cmd.pluginID, model.CommandRequest{
		UserID:      cmd.userID,
		ChannelType: cmd.channelType,
		ChatID:      cmd.chatID,
		ChatGroupID: cmd.chatGroupID,
		PluginID:    cmd.pluginID,
		CommandName: cmd.commandName,
		Params:      cmd.params,
		Locale:      cmd.locale,
		Files:       cmd.files,
	}); err != nil {
		result = "dispatch_error"
		return &completedCommandError{cmd: cmd, err: err}
	}

	m.tryReturnToPluginMenu(ctx, cmd)
	return nil
}

func (m *ChannelManager) resultPluginID(result *StateResult) string {
	if result.PluginID != "" {
		return result.PluginID
	}
	// Fallback for dialogs started before the PluginID field existed.
	return m.plugins.GetPluginIDByCommand(result.CommandName)
}

func (m *ChannelManager) tryReturnToPluginMenu(ctx context.Context, cmd completedCommand) {
	if !m.shouldReturnToPluginMenu(cmd) {
		return
	}

	_, err := m.state.StartCommand(ctx, cmd.userID, cmd.chatID, "core", "plugins", cmd.locale)
	if err != nil {
		m.logger.Warn("channel: auto-return to plugin menu failed",
			"plugin_id", cmd.pluginID,
			"command", cmd.commandName,
			"error", err)
		return
	}

	result, err := m.state.ProcessInput(ctx, cmd.userID, cmd.chatID, model.CallbackInput{Data: cmd.pluginID}, cmd.locale)
	if err != nil {
		m.logger.Warn("channel: auto-return to plugin menu failed",
			"plugin_id", cmd.pluginID,
			"command", cmd.commandName,
			"error", err)
		return
	}
	if result == nil {
		return
	}
	if err := m.sendResultMessage(ctx, cmd.channelType, cmd.chatID, result.Message); err != nil {
		m.logger.Warn("channel: auto-return to plugin menu failed",
			"plugin_id", cmd.pluginID,
			"command", cmd.commandName,
			"error", err)
	}
}

func (m *ChannelManager) shouldReturnToPluginMenu(cmd completedCommand) bool {
	if cmd.pluginID == "" {
		return false
	}
	if m.state.IsPreservesDialog(cmd.pluginID, cmd.commandName) {
		return false
	}

	switch {
	case cmd.pluginID == "core" && cmd.commandName == "start":
		return false
	case cmd.pluginID == "core" && cmd.commandName == "plugins":
		return false
	case cmd.pluginID == "core" && cmd.commandName == "resume":
		return false
	default:
		return true
	}
}

func (m *ChannelManager) shouldIgnoreInputError(err error, input model.UserInput) bool {
	if !errors.Is(err, state.ErrNoActiveDialog) {
		return false
	}
	_, isFile := input.(model.FileInput)
	return isFile
}

func (m *ChannelManager) sendResultMessage(ctx context.Context, channelType model.ChannelType, chatID string, msg model.Message) error {
	if msg.IsEmpty() {
		return nil
	}
	msgID, err := m.adapters.SendToChatGetID(ctx, channelType, chatID, msg)
	if err != nil {
		return err
	}
	if msgID != "" {
		m.lastBotMsgID.Store(chatID, msgID)
	}
	return nil
}

// buildDisambiguationMessage builds an options message listing all candidates.
// The candidate whose plugin matches the user's recent focus is placed first
// and marked as probable; the rest are sorted alphabetically by FQ name.
func (m *ChannelManager) buildDisambiguationMessage(userID model.GlobalUserID, candidates []model.CommandCandidate, loc string) model.Message {
	focusPlugin := ""
	if m.focus != nil {
		focusPlugin = m.focus.LastPlugin(userID)
	}

	// Sort: focused candidate first, then alphabetical by FQ name.
	sorted := make([]model.CommandCandidate, len(candidates))
	copy(sorted, candidates)
	sort.Slice(sorted, func(i, j int) bool {
		iFocused := sorted[i].PluginID == focusPlugin && focusPlugin != ""
		jFocused := sorted[j].PluginID == focusPlugin && focusPlugin != ""
		if iFocused != jFocused {
			return iFocused
		}
		return sorted[i].FQName < sorted[j].FQName
	})

	options := make([]model.Option, len(sorted))
	for i, c := range sorted {
		label := disambiguationLabel(c, loc)
		if c.PluginID == focusPlugin && focusPlugin != "" && i == 0 {
			label = "⟶ " + label
		}
		options[i] = model.Option{
			Label: label,
			Value: "/" + c.FQName,
		}
	}

	return model.Message{
		Blocks: []model.ContentBlock{
			model.TextBlock{
				Text:  i18n.Get("disambiguate.prompt", loc),
				Style: model.StylePlain,
			},
			model.OptionsBlock{
				Options: options,
			},
		},
	}
}

func disambiguationLabel(c model.CommandCandidate, loc string) string {
	if label := locale.ResolveText(c.Descriptions, loc); label != "" {
		return label
	}
	if c.Description != "" {
		return c.Description
	}
	if c.PluginID != "" {
		return c.PluginID
	}
	return c.FQName
}

func (m *ChannelManager) recordFocus(userID model.GlobalUserID, pluginID string) {
	if m.focus != nil && pluginID != "" {
		m.focus.Record(userID, pluginID)
	}
}

func (m *ChannelManager) routeCommand(ctx context.Context, pluginID string, req model.CommandRequest) error {
	event, err := contract.NewMessengerEvent(req, pluginID)
	if err != nil {
		return fmt.Errorf("build messenger event: %w", err)
	}
	resp, err := m.router.RouteEvent(ctx, event)
	if err != nil {
		return err
	}
	if resp != nil && resp.Error != "" {
		return fmt.Errorf("plugin %q command %q: %s", pluginID, req.CommandName, resp.Error)
	}
	return nil
}

func (m *ChannelManager) handleError(ctx context.Context, channelType model.ChannelType, chatID string, userID model.GlobalUserID, err error) {
	defer m.tryReturnToPluginMenuFromError(ctx, err)

	var appErr *errs.AppError
	if errors.As(err, &appErr) {
		switch appErr.Severity {
		case errs.SeverityUser:
			m.logger.Warn("user error",
				slog.String("code", string(appErr.Code)),
				slog.Int64("user_id", int64(userID)),
				slog.String("message", appErr.Message))
			msg := appErr.Message
			if msg == "" {
				msg = i18n.Get("error.generic", locale.Default())
			}
			m.sendErrorReply(ctx, channelType, chatID, userID, msg, err)

		case errs.SeveritySilent:
			m.logger.Debug("silent error",
				slog.String("code", string(appErr.Code)),
				slog.Int64("user_id", int64(userID)),
				slog.String("message", appErr.Message))

		case errs.SeverityInternal:
			m.logger.Error("internal error",
				slog.String("code", string(appErr.Code)),
				slog.Int64("user_id", int64(userID)),
				slog.Any("error", err))
			m.sendErrorReply(ctx, channelType, chatID, userID, i18n.Get("error.internal", locale.Default()), err)
		}
		return
	}

	m.logger.Error("unexpected error processing update",
		slog.Int64("user_id", int64(userID)),
		slog.Any("error", err))
	m.sendErrorReply(ctx, channelType, chatID, userID, "An error occurred. Please try again.", err)
}

func (m *ChannelManager) tryReturnToPluginMenuFromError(ctx context.Context, err error) {
	var cmdErr *completedCommandError
	if !errors.As(err, &cmdErr) {
		return
	}
	m.tryReturnToPluginMenu(ctx, cmdErr.cmd)
}

func (m *ChannelManager) sendErrorReply(ctx context.Context, channelType model.ChannelType, chatID string, userID model.GlobalUserID, msg string, originalErr error) {
	if sendErr := m.adapters.SendToChat(ctx, channelType, chatID, model.NewTextMessage(msg)); sendErr != nil {
		m.logger.Error("failed to send error reply to user",
			slog.Int64("user_id", int64(userID)),
			slog.String("chat_id", chatID),
			slog.Any("send_error", sendErr),
			slog.Any("original_error", originalErr))
	}
}

// extractFiles returns file references from a FileInput, or nil for other input types.
func extractFiles(input model.UserInput) []model.FileRef {
	if fi, ok := input.(model.FileInput); ok {
		return fi.Files
	}
	return nil
}

func (m *ChannelManager) incCommandExecution(channelType model.ChannelType, pluginID, commandName, result string) {
	if m.metrics == nil {
		return
	}
	m.metrics.CommandExecutionsTotal.WithLabelValues(
		string(channelType),
		pluginID,
		commandName,
		result,
	).Inc()
}

func updateInputType(input model.UserInput) string {
	switch input.(type) {
	case model.TextInput:
		return "text"
	case model.CallbackInput:
		return "callback"
	case model.FileInput:
		return "file"
	default:
		return "unknown"
	}
}

func classifyUpdateResult(err error) string {
	var appErr *errs.AppError
	if errors.As(err, &appErr) {
		switch appErr.Severity {
		case errs.SeverityUser:
			return "user_error"
		case errs.SeveritySilent:
			return "silent"
		case errs.SeverityInternal:
			return "internal_error"
		}
	}
	return "internal_error"
}
