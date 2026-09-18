package state

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"SuperBotGo/internal/metrics"
	"SuperBotGo/internal/model"
	"SuperBotGo/internal/state/storage"
)

type Manager struct {
	storage  storage.DialogStorage
	commands map[string]*CommandDefinition
	handlers map[string]StateHandler
	mu       sync.RWMutex
	metrics  *metrics.Metrics
}

func NewManager(store storage.DialogStorage) *Manager {
	return &Manager{
		storage:  store,
		commands: make(map[string]*CommandDefinition),
		handlers: make(map[string]StateHandler),
	}
}

func (m *Manager) SetMetrics(metricSet *metrics.Metrics) {
	m.metrics = metricSet
}

// fqKey builds a fully-qualified map key: "pluginID.commandName".
func fqKey(pluginID, name string) string {
	return pluginID + "." + name
}

func (m *Manager) RegisterCommand(pluginID string, def *CommandDefinition) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := fqKey(pluginID, def.Name)
	m.commands[key] = def
	m.handlers[key] = NewDslStateHandler(def)
}

func (m *Manager) UnregisterCommand(pluginID, name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := fqKey(pluginID, name)
	delete(m.commands, key)
	delete(m.handlers, key)
}

// UnregisterAllCommands removes every command registered under the given
// plugin ID. Used when a plugin is deleted or disabled and the caller no
// longer has access to the plugin instance to enumerate its commands.
func (m *Manager) UnregisterAllCommands(pluginID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	prefix := pluginID + "."
	for key := range m.commands {
		if strings.HasPrefix(key, prefix) {
			delete(m.commands, key)
			delete(m.handlers, key)
		}
	}
}

func (m *Manager) RegisterHandler(name string, handler StateHandler) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.handlers[name] = handler
}

func (m *Manager) IsCommandImmediate(pluginID, commandName string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	def, ok := m.commands[fqKey(pluginID, commandName)]
	if !ok {
		return false
	}
	return def.IsComplete(StepContext{})
}

func (m *Manager) StartCommand(ctx context.Context, userID model.GlobalUserID, chatID string, pluginID string, commandName string, locale string) (model.Message, error) {
	key := fqKey(pluginID, commandName)
	m.mu.RLock()
	handler, ok := m.handlers[key]
	m.mu.RUnlock()
	if !ok {
		return model.Message{}, fmt.Errorf("%w: %s", ErrCommandNotFound, key)
	}

	state, err := handler.CreateNewState(commandName)
	if err != nil {
		return model.Message{}, fmt.Errorf("creating state for %s: %w", key, err)
	}

	msg := handler.BuildStepMessage(ctx, userID, state, locale)

	ds := handler.PersistState(state)
	ds.ChatID = chatID
	ds.PluginID = pluginID
	if err := m.storage.Save(ctx, userID, ds); err != nil {
		return model.Message{}, fmt.Errorf("saving state: %w", err)
	}
	m.incDialogTransition(pluginID, commandName, "started")

	return msg, nil
}

func (m *Manager) ProcessInput(ctx context.Context, userID model.GlobalUserID, chatID string, input model.UserInput, locale string) (model.Message, *model.CommandRequest, error) {
	ds, err := m.storage.Load(ctx, userID)
	if err != nil {
		return model.Message{}, nil, fmt.Errorf("loading state: %w", err)
	}
	if ds == nil {
		return model.Message{}, nil, ErrNoActiveDialog
	}
	if ds.ChatID != "" && ds.ChatID != chatID {
		return model.Message{}, nil, nil
	}

	key := fqKey(ds.PluginID, ds.CommandName)
	m.mu.RLock()
	handler, ok := m.handlers[key]
	m.mu.RUnlock()
	if !ok {
		return model.Message{}, nil, fmt.Errorf("%w: %s", ErrCommandNotFound, key)
	}

	state, err := handler.RestoreState(*ds)
	if err != nil {
		return model.Message{}, nil, fmt.Errorf("restoring state: %w", err)
	}

	nextState, outcome, err := handler.ProcessInput(ctx, userID, state, input, locale)
	if err != nil {
		return model.Message{}, nil, fmt.Errorf("processing input: %w", err)
	}
	if outcome.IsCancelled {
		if err := m.CancelCommand(ctx, userID); err != nil {
			return model.Message{}, nil, err
		}
		if _, err := m.StartCommand(ctx, userID, chatID, "core", "plugins", locale); err != nil {
			return model.Message{}, nil, err
		}
		return m.ProcessInput(ctx, userID, chatID, model.CallbackInput{Data: ds.PluginID}, locale)
	}

	msg := handler.BuildStepMessage(ctx, userID, nextState, locale)
	outcome.Message = msg

	if outcome.IsComplete {
		if delErr := m.storage.Delete(ctx, userID); delErr != nil {
			return model.Message{}, nil, fmt.Errorf("deleting state: %w", delErr)
		}
		m.incDialogTransition(ds.PluginID, ds.CommandName, "completed")
		cmdReq := &model.CommandRequest{
			UserID:      userID,
			PluginID:    ds.PluginID,
			CommandName: outcome.CommandName,
			Params:      outcome.Params,
			Locale:      locale,
		}
		return msg, cmdReq, nil
	}

	persistedDS := handler.PersistState(nextState)
	persistedDS.ChatID = ds.ChatID
	persistedDS.PluginID = ds.PluginID
	if err := m.storage.Save(ctx, userID, persistedDS); err != nil {
		return model.Message{}, nil, fmt.Errorf("saving state: %w", err)
	}
	m.incDialogTransition(ds.PluginID, ds.CommandName, "continued")

	return msg, nil, nil
}

func (m *Manager) HasActiveDialog(ctx context.Context, userID model.GlobalUserID) bool {
	ds, err := m.storage.Load(ctx, userID)
	return err == nil && ds != nil
}

func (m *Manager) CancelCommand(ctx context.Context, userID model.GlobalUserID) error {
	var (
		pluginID string
		command  string
	)

	ds, err := m.storage.Load(ctx, userID)
	if err != nil {
		return err
	}
	if ds != nil {
		pluginID = ds.PluginID
		command = ds.CommandName
	}
	if err := m.storage.Delete(ctx, userID); err != nil {
		return err
	}
	if ds != nil {
		m.incDialogTransition(pluginID, command, "cancelled")
	}
	return nil
}

func (m *Manager) RelocateDialog(ctx context.Context, userID model.GlobalUserID, chatID string) error {
	ds, err := m.storage.Load(ctx, userID)
	if err != nil {
		return fmt.Errorf("loading state: %w", err)
	}
	if ds == nil {
		return nil
	}
	ds.ChatID = chatID
	return m.storage.Save(ctx, userID, *ds)
}

func (m *Manager) IsPreservesDialog(pluginID, commandName string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	def, ok := m.commands[fqKey(pluginID, commandName)]
	if !ok {
		return false
	}
	return def.PreservesDialog
}

func (m *Manager) GetCurrentStepMessage(ctx context.Context, userID model.GlobalUserID, locale string) (*model.Message, string, error) {
	ds, err := m.storage.Load(ctx, userID)
	if err != nil {
		return nil, "", fmt.Errorf("loading state: %w", err)
	}
	if ds == nil {
		return nil, "", nil
	}

	key := fqKey(ds.PluginID, ds.CommandName)
	m.mu.RLock()
	handler, ok := m.handlers[key]
	m.mu.RUnlock()
	if !ok {
		return nil, "", fmt.Errorf("%w: %s", ErrCommandNotFound, key)
	}

	state, err := handler.RestoreState(*ds)
	if err != nil {
		return nil, "", fmt.Errorf("restoring state: %w", err)
	}

	msg := handler.BuildStepMessage(ctx, userID, state, locale)
	return &msg, ds.CommandName, nil
}

func (m *Manager) incDialogTransition(pluginID, commandName, event string) {
	if m.metrics == nil {
		return
	}
	m.metrics.DialogTransitionsTotal.WithLabelValues(pluginID, commandName, event).Inc()
}
