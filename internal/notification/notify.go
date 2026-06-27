package notification

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"SuperBotGo/internal/channel"
	"SuperBotGo/internal/model"
)

type UserService interface {
	GetUser(ctx context.Context, id model.GlobalUserID) (*model.GlobalUser, error)
}

type StudentResolver interface {
	ResolveStudentUsers(ctx context.Context, scope string, targetID int64) ([]model.GlobalUserID, error)
}

type TeacherResolver interface {
	ResolveTeacherUser(ctx context.Context, ref model.TeacherRef) (model.GlobalUserID, error)
}

type NotifyAPI struct {
	adapters *channel.AdapterRegistry
	users    UserService
	prefs    PrefsRepository
	students StudentResolver
	teachers TeacherResolver

	scheduled      ScheduledMessageStore
	workerInterval time.Duration
	claimLimit     int
	claimLease     time.Duration

	// menuCommand, if set, is appended as a fallback button to notifications
	// that have no interactive options. Typically "/start".
	menuCommand string
}

func NewNotifyAPI(
	adapters *channel.AdapterRegistry,
	users UserService,
	prefs PrefsRepository,
	students StudentResolver,
	stores ...ScheduledMessageStore,
) *NotifyAPI {
	scheduled := ScheduledMessageStore(NewMemoryScheduledStore())
	if len(stores) > 0 && stores[0] != nil {
		scheduled = stores[0]
	}

	api := &NotifyAPI{
		adapters:       adapters,
		users:          users,
		prefs:          prefs,
		students:       students,
		scheduled:      scheduled,
		workerInterval: time.Minute,
		claimLimit:     100,
		claimLease:     5 * time.Minute,
	}

	go api.startWorker(context.Background())

	return api
}

func (n *NotifyAPI) SetTeacherResolver(resolver TeacherResolver) {
	n.teachers = resolver
}

// WithMenuCommand sets a fallback navigation button that is automatically
// injected into any notification message that has no interactive options.
// This ensures users always have a way back to the main menu.
func (n *NotifyAPI) WithMenuCommand(cmd string) {
	n.menuCommand = cmd
}

// injectMenuButton appends a "main menu" button to msg when the message has
// no OptionsBlock yet and a menuCommand is configured.
func (n *NotifyAPI) injectMenuButton(msg model.Message) model.Message {
	if n.menuCommand == "" {
		return msg
	}
	for _, b := range msg.Blocks {
		if _, ok := b.(model.OptionsBlock); ok {
			return msg
		}
	}
	blocks := make([]model.ContentBlock, len(msg.Blocks)+1)
	copy(blocks, msg.Blocks)
	blocks[len(msg.Blocks)] = model.OptionsBlock{
		Options: []model.Option{{Label: "🏠 Главное меню", Value: n.menuCommand}},
	}
	return model.Message{Blocks: blocks}
}

func (n *NotifyAPI) NotifyUser(
	ctx context.Context,
	userID model.GlobalUserID,
	msg model.Message,
	priority model.NotifyPriority,
) error {
	return n.deliverUser(ctx, userID, msg, priority, nil, true)
}

func (n *NotifyAPI) deliverUser(
	ctx context.Context,
	userID model.GlobalUserID,
	msg model.Message,
	priority model.NotifyPriority,
	scheduledAt *time.Time,
	allowDelay bool,
) error {

	user, err := n.users.GetUser(ctx, userID)
	if err != nil || user == nil {
		return fmt.Errorf("user not found: %w", err)
	}

	prefs, err := n.prefs.GetPrefs(ctx, userID)
	if err != nil {
		return err
	}
	if prefs == nil {
		prefs = defaultPrefs(userID, user.PrimaryChannel)
	}

	if allowDelay && shouldDelay(priority, prefs) {
		return n.schedule(ctx, userID, msg, priority, prefs)
	}

	opts := n.buildSendOptions(prefs, priority)
	if scheduledAt != nil {
		msg = addScheduledNotice(msg, *scheduledAt, prefs)
	}

	return n.sendToUser(ctx, user, msg, priority, prefs, opts)
}

func (n *NotifyAPI) sendToUser(
	ctx context.Context,
	user *model.GlobalUser,
	msg model.Message,
	priority model.NotifyPriority,
	prefs *model.NotificationPrefs,
	opts model.SendOptions,
) error {

	msg = n.injectMenuButton(msg)

	if priority == model.PriorityCritical {
		for _, acc := range user.Accounts {
			targetMsg := n.maybeInjectMention(msg, acc.ChannelUserID, prefs, priority)
			_ = n.adapters.SendToUserWithOpts(
				ctx,
				acc.ChannelType,
				acc.ChannelUserID,
				targetMsg,
				opts,
			)
		}
		return nil
	}

	ch, id := n.resolveChannel(user, prefs)
	msg = n.maybeInjectMention(msg, id, prefs, priority)

	return n.adapters.SendToUserWithOpts(ctx, ch, id, msg, opts)
}

// NotifyUsers sends one notification to each listed user.
func (n *NotifyAPI) NotifyUsers(ctx context.Context, userIDs []model.GlobalUserID, msg model.Message, priority model.NotifyPriority) error {
	var sendErrs []error
	for _, userID := range userIDs {
		if err := n.NotifyUser(ctx, userID, msg, priority); err != nil {
			slog.Error("notify: partial failure sending to user list",
				slog.Int64("user_id", int64(userID)),
				slog.Any("error", err))
			sendErrs = append(sendErrs, fmt.Errorf("user %d: %w", userID, err))
		}
	}
	if len(sendErrs) > 0 {
		return fmt.Errorf("notify: direct user broadcast failed on %d/%d users: %w",
			len(sendErrs), len(userIDs), errors.Join(sendErrs...))
	}
	return nil
}

// NotifyChat sends a notification to a specific chat with priority-aware delivery.
func (n *NotifyAPI) NotifyChat(ctx context.Context, channelType model.ChannelType, chatID string, msg model.Message, priority model.NotifyPriority) error {
	return n.adapters.SendToChatWithOpts(ctx, channelType, chatID, msg, model.SendOptions{})
}

// NotifyStudents sends a priority-aware notification to all students within
// the given university hierarchy scope.
func (n *NotifyAPI) NotifyStudents(ctx context.Context, scope string, targetID int64, msg model.Message, priority model.NotifyPriority) error {
	userIDs, err := n.students.ResolveStudentUsers(ctx, scope, targetID)
	if err != nil {
		return fmt.Errorf("notify: resolve students for %s/%d: %w", scope, targetID, err)
	}

	if err := n.NotifyUsers(ctx, userIDs, msg, priority); err != nil {
		return fmt.Errorf("notify: students %s/%d broadcast failed: %w", scope, targetID, err)
	}
	return nil
}

// NotifyTeacher resolves a university teacher/person reference to a linked
// global user and sends a priority-aware notification to that user.
func (n *NotifyAPI) NotifyTeacher(ctx context.Context, ref model.TeacherRef, msg model.Message, priority model.NotifyPriority) error {
	if n.teachers == nil {
		return fmt.Errorf("notify: teacher resolver is not configured")
	}

	userID, err := n.teachers.ResolveTeacherUser(ctx, ref)
	if err != nil {
		return fmt.Errorf("notify: resolve teacher recipient: %w", err)
	}
	return n.NotifyUser(ctx, userID, msg, priority)
}

func (n *NotifyAPI) schedule(
	ctx context.Context,
	userID model.GlobalUserID,
	msg model.Message,
	priority model.NotifyPriority,
	prefs *model.NotificationPrefs,
) error {
	now := time.Now()
	sendAt := nextWorkTime(prefs)

	if err := n.scheduled.Enqueue(ctx, ScheduledMessage{
		UserID:    userID,
		Msg:       msg,
		Priority:  priority,
		SendAt:    sendAt,
		CreatedAt: now,
	}); err != nil {
		return err
	}

	slog.Info("message scheduled",
		slog.Int64("user_id", int64(userID)),
		slog.Time("send_at", sendAt),
		slog.Time("created_at", now),
	)
	return nil
}

func (n *NotifyAPI) startWorker(ctx context.Context) {
	ticker := time.NewTicker(n.workerInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			n.process(ctx)
		case <-ctx.Done():
			return
		}
	}
}

func (n *NotifyAPI) process(ctx context.Context) {
	now := time.Now()

	messages, err := n.scheduled.ClaimDue(ctx, now, n.claimLimit, n.claimLease)
	if err != nil {
		slog.Error("notification: failed to claim scheduled messages", slog.Any("error", err))
		return
	}

	for _, m := range messages {
		n.processScheduled(ctx, m)
	}
}

func (n *NotifyAPI) processScheduled(ctx context.Context, scheduled ScheduledMessage) {
	user, err := n.users.GetUser(ctx, scheduled.UserID)
	if err != nil || user == nil {
		n.rescheduleAfterFailure(ctx, scheduled, fmt.Errorf("user not found: %w", err))
		return
	}

	prefs, err := n.prefs.GetPrefs(ctx, scheduled.UserID)
	if err != nil {
		n.rescheduleAfterFailure(ctx, scheduled, err)
		return
	}
	if prefs == nil {
		prefs = defaultPrefs(scheduled.UserID, user.PrimaryChannel)
	}

	if shouldDelay(scheduled.Priority, prefs) {
		if err := n.scheduled.Reschedule(ctx, scheduled.ID, nextWorkTime(prefs), nil); err != nil {
			slog.Error("notification: failed to reschedule delayed message",
				slog.Int64("scheduled_id", scheduled.ID),
				slog.Any("error", err))
		}
		return
	}

	msg := addScheduledNotice(scheduled.Msg, scheduled.CreatedAt, prefs)
	opts := n.buildSendOptions(prefs, scheduled.Priority)
	if err := n.sendToUser(ctx, user, msg, scheduled.Priority, prefs, opts); err != nil {
		n.rescheduleAfterFailure(ctx, scheduled, err)
		return
	}

	if err := n.scheduled.Complete(ctx, scheduled.ID); err != nil {
		slog.Error("notification: failed to complete scheduled message",
			slog.Int64("scheduled_id", scheduled.ID),
			slog.Any("error", err))
	}
}

func (n *NotifyAPI) rescheduleAfterFailure(ctx context.Context, scheduled ScheduledMessage, cause error) {
	nextAttempt := time.Now().Add(retryDelay(scheduled.Attempts))
	if err := n.scheduled.Reschedule(ctx, scheduled.ID, nextAttempt, cause); err != nil {
		slog.Error("notification: failed to reschedule failed message",
			slog.Int64("scheduled_id", scheduled.ID),
			slog.Any("delivery_error", cause),
			slog.Any("error", err))
		return
	}

	slog.Warn("notification: scheduled message delivery failed",
		slog.Int64("scheduled_id", scheduled.ID),
		slog.Int("attempts", scheduled.Attempts),
		slog.Time("next_attempt", nextAttempt),
		slog.Any("error", cause))
}

func retryDelay(attempts int) time.Duration {
	if attempts <= 1 {
		return time.Minute
	}
	if attempts >= 6 {
		return time.Hour
	}
	return time.Duration(attempts*attempts) * time.Minute
}

func nextWorkTime(prefs *model.NotificationPrefs) time.Time {
	return nextWorkTimeAt(prefs, time.Now())
}

func nextWorkTimeAt(prefs *model.NotificationPrefs, now time.Time) time.Time {
	loc := notificationLocation(prefs)
	localNow := now.In(loc)

	if prefs.WorkHoursStart == nil {
		return localNow.Add(1 * time.Minute)
	}

	start := time.Date(localNow.Year(), localNow.Month(), localNow.Day(),
		*prefs.WorkHoursStart, 0, 0, 0, loc)

	if localNow.Before(start) {
		return start
	}

	return start.Add(24 * time.Hour)
}

func isWithinWorkHours(prefs *model.NotificationPrefs) bool {
	return isWithinWorkHoursAt(prefs, time.Now())
}

func isWithinWorkHoursAt(prefs *model.NotificationPrefs, now time.Time) bool {
	if prefs.WorkHoursStart == nil || prefs.WorkHoursEnd == nil {
		return true
	}

	localNow := now.In(notificationLocation(prefs))
	hour := localNow.Hour()
	start := *prefs.WorkHoursStart
	end := *prefs.WorkHoursEnd

	if start <= end {
		return hour >= start && hour < end
	}
	return hour >= start || hour < end
}

func (n *NotifyAPI) buildSendOptions(
	prefs *model.NotificationPrefs,
	priority model.NotifyPriority,
) model.SendOptions {
	var opts model.SendOptions

	if prefs.MuteMentions && priority < model.PriorityCritical {
		opts.StripMentions = true
	}

	return opts
}

func (n *NotifyAPI) resolveChannel(user *model.GlobalUser, prefs *model.NotificationPrefs) (model.ChannelType, model.PlatformUserID) {
	accountMap := make(map[model.ChannelType]model.PlatformUserID, len(user.Accounts))
	for _, acc := range user.Accounts {
		accountMap[acc.ChannelType] = acc.ChannelUserID
	}

	for _, ch := range prefs.ChannelPriority {
		if pid, ok := accountMap[ch]; ok {
			return ch, pid
		}
	}

	if pid, ok := accountMap[user.PrimaryChannel]; ok {
		return user.PrimaryChannel, pid
	}

	if len(user.Accounts) > 0 {
		acc := user.Accounts[0]
		return acc.ChannelType, acc.ChannelUserID
	}
	return user.PrimaryChannel, ""
}

func (n *NotifyAPI) maybeInjectMention(msg model.Message, platformUserID model.PlatformUserID, prefs *model.NotificationPrefs, priority model.NotifyPriority) model.Message {
	if priority < model.PriorityHigh {
		return msg
	}
	if prefs.MuteMentions && priority < model.PriorityCritical {
		return msg
	}

	for _, block := range msg.Blocks {
		if m, ok := block.(model.MentionBlock); ok && m.UserID == string(platformUserID) {
			return msg
		}
	}

	blocks := make([]model.ContentBlock, 0, len(msg.Blocks)+1)
	blocks = append(blocks, model.MentionBlock{UserID: string(platformUserID)})
	blocks = append(blocks, msg.Blocks...)
	return model.Message{Blocks: blocks}
}

func shouldDelay(priority model.NotifyPriority, prefs *model.NotificationPrefs) bool {
	return shouldDelayAt(priority, prefs, time.Now())
}

func shouldDelayAt(priority model.NotifyPriority, prefs *model.NotificationPrefs, now time.Time) bool {
	return priority < model.PriorityCritical && !isWithinWorkHoursAt(prefs, now)
}

func notificationLocation(prefs *model.NotificationPrefs) *time.Location {
	if prefs != nil && prefs.Timezone != "" {
		if loc, err := time.LoadLocation(prefs.Timezone); err == nil {
			return loc
		}
	}
	return time.UTC
}

func addScheduledNotice(msg model.Message, createdAt time.Time, prefs *model.NotificationPrefs) model.Message {
	loc := notificationLocation(prefs)
	notice := model.TextBlock{
		Text:  fmt.Sprintf("Отложенное сообщение. Отправлено пользователем: %s", createdAt.In(loc).Format("02.01.2006 15:04 MST")),
		Style: model.StyleQuote,
	}

	blocks := make([]model.ContentBlock, 0, len(msg.Blocks)+1)
	blocks = append(blocks, notice)
	blocks = append(blocks, msg.Blocks...)
	return model.Message{Blocks: blocks}
}

func defaultPrefs(userID model.GlobalUserID, primary model.ChannelType) *model.NotificationPrefs {
	return &model.NotificationPrefs{
		GlobalUserID:    userID,
		ChannelPriority: []model.ChannelType{primary},
		Timezone:        "UTC",
	}
}
