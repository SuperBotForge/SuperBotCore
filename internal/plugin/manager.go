package plugin

import (
	"sort"
	"sync"

	"SuperBotGo/internal/state"
)

type Manager struct {
	mu      sync.RWMutex
	plugins map[string]Plugin
}

func NewManager() *Manager {
	return &Manager{
		plugins: make(map[string]Plugin),
	}
}

func (m *Manager) Load(plugins []Plugin) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, p := range plugins {
		m.plugins[p.ID()] = p
	}
}

func (m *Manager) Register(p Plugin) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.plugins[p.ID()] = p
}

func (m *Manager) Remove(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.plugins, id)
}

func (m *Manager) GetByCommand(commandName string) Plugin {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, p := range m.plugins {
		for _, name := range CommandNames(p) {
			if name == commandName {
				return p
			}
		}
	}
	return nil
}

func (m *Manager) GetCommandDefinition(commandName string) *state.CommandDefinition {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, p := range m.plugins {
		for _, def := range p.Commands() {
			if def.Name == commandName {
				return def
			}
		}
	}
	return nil
}

func (m *Manager) GetPluginIDByCommand(commandName string) string {
	p := m.GetByCommand(commandName)
	if p == nil {
		return ""
	}
	return p.ID()
}

func (m *Manager) Get(id string) (Plugin, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	p, ok := m.plugins[id]
	return p, ok
}

func (m *Manager) All() map[string]Plugin {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make(map[string]Plugin, len(m.plugins))
	for k, v := range m.plugins {
		result[k] = v
	}
	return result
}

// ListUserPlugins returns plugin info for all plugins except those whose IDs
// are in excludeIDs. The result is sorted by plugin name.
func (m *Manager) ListUserPlugins(excludeIDs ...string) []PluginInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	exclude := make(map[string]struct{}, len(excludeIDs))
	for _, id := range excludeIDs {
		exclude[id] = struct{}{}
	}

	var result []PluginInfo
	for _, p := range m.plugins {
		if _, skip := exclude[p.ID()]; skip {
			continue
		}
		cmds := p.Commands()
		commands := make([]PluginCommand, len(cmds))
		for i, c := range cmds {
			commands[i] = PluginCommand{
				Name:         c.Name,
				Descriptions: copyStringMap(c.Descriptions),
				Description:  c.Description,
				Requirements: c.Requirements,
			}
		}
		supportsVis := false
		if vp, ok := p.(interface{ SupportsVisibility() bool }); ok {
			supportsVis = vp.SupportsVisibility()
		}
		result = append(result, PluginInfo{
			ID:                 p.ID(),
			Name:               p.Name(),
			Commands:           commands,
			SupportsVisibility: supportsVis,
		})
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}

func copyStringMap(src map[string]string) map[string]string {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]string, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}
