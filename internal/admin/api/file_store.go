package api

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

type FilePluginStore struct {
	mu       sync.RWMutex
	path     string
	data     map[string]PluginRecord
	metadata map[string]PluginMetadataRecord
	frontend map[string]PluginFrontendRecord
}

func (s *FilePluginStore) SavePlugin(_ context.Context, record PluginRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[record.ID] = record
	return s.flush()
}

func (s *FilePluginStore) GetPlugin(_ context.Context, id string) (PluginRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rec, ok := s.data[id]
	if !ok {
		return PluginRecord{}, fmt.Errorf("plugin %q not found", id)
	}
	return rec, nil
}

func (s *FilePluginStore) ListPlugins(_ context.Context) ([]PluginRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]PluginRecord, 0, len(s.data))
	for _, rec := range s.data {
		result = append(result, rec)
	}
	return result, nil
}

func (s *FilePluginStore) DeletePlugin(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.data[id]; !ok {
		return fmt.Errorf("plugin %q not found", id)
	}
	delete(s.data, id)
	return s.flush()
}

func (s *FilePluginStore) SavePluginMetadata(_ context.Context, record PluginMetadataRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.metadata == nil {
		s.metadata = make(map[string]PluginMetadataRecord)
	}
	s.metadata[record.PluginID] = record
	return s.flush()
}

func (s *FilePluginStore) GetPluginMetadata(_ context.Context, id string) (PluginMetadataRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rec, ok := s.metadata[id]
	if !ok {
		return PluginMetadataRecord{}, fmt.Errorf("plugin metadata %q not found", id)
	}
	return rec, nil
}

func (s *FilePluginStore) DeletePluginMetadata(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.metadata[id]; !ok {
		return fmt.Errorf("plugin metadata %q not found", id)
	}
	delete(s.metadata, id)
	return s.flush()
}

func (s *FilePluginStore) SavePluginFrontend(_ context.Context, record PluginFrontendRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.frontend == nil {
		s.frontend = make(map[string]PluginFrontendRecord)
	}
	s.frontend[record.PluginID] = record
	return s.flush()
}

func (s *FilePluginStore) GetPluginFrontend(_ context.Context, pluginID string) (PluginFrontendRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rec, ok := s.frontend[pluginID]
	if !ok {
		return PluginFrontendRecord{}, fmt.Errorf("plugin frontend %q not found", pluginID)
	}
	return rec, nil
}

func (s *FilePluginStore) DeletePluginFrontend(_ context.Context, pluginID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.frontend, pluginID)
	return s.flush()
}

func (s *FilePluginStore) flush() error {
	var payload struct {
		Plugins  []PluginRecord         `json:"plugins"`
		Metadata []PluginMetadataRecord `json:"metadata,omitempty"`
		Frontend []PluginFrontendRecord `json:"frontend,omitempty"`
	}
	payload.Plugins = make([]PluginRecord, 0, len(s.data))
	for _, r := range s.data {
		payload.Plugins = append(payload.Plugins, r)
	}
	payload.Metadata = make([]PluginMetadataRecord, 0, len(s.metadata))
	for _, r := range s.metadata {
		payload.Metadata = append(payload.Metadata, r)
	}
	payload.Frontend = make([]PluginFrontendRecord, 0, len(s.frontend))
	for _, r := range s.frontend {
		payload.Frontend = append(payload.Frontend, r)
	}
	raw, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal plugins: %w", err)
	}
	if err := os.WriteFile(s.path, raw, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", s.path, err)
	}
	return nil
}

var _ PluginStore = (*FilePluginStore)(nil)
var _ PluginFrontendStore = (*FilePluginStore)(nil)
