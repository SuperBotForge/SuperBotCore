package hostapi

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	wasmrt "SuperBotGo/internal/wasm/runtime"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// pluginSchemaName returns the PostgreSQL schema name for a plugin: "plugin_{id}".
func pluginSchemaName(pluginID string) string {
	s := strings.ToLower(strings.TrimSpace(pluginID))
	s = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
			return r
		}
		return '_'
	}, s)
	s = strings.Trim(s, "_")
	if s == "" {
		s = "plugin"
	}
	return "plugin_" + s
}

// injectSearchPath appends search_path=schema to a PostgreSQL DSN.
// Supports both URL format (postgres://...) and key=value format.
func injectSearchPath(dsn, schema string) string {
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		if strings.Contains(dsn, "?") {
			return dsn + "&search_path=" + schema
		}
		return dsn + "?search_path=" + schema
	}
	return dsn + " search_path=" + schema
}

type handleKind uint8

const (
	handleConn handleKind = iota
	handleTx
	handleRows
)

type sqlHandle struct {
	kind       handleKind
	conn       *pgxpool.Conn
	tx         pgx.Tx
	rows       pgx.Rows
	cols       []string
	connHandle uint32 // parent conn handle (for tx and rows)
}

type executionHandles struct {
	mu      sync.Mutex
	nextID  uint32
	handles map[uint32]*sqlHandle
}

type pluginSQLState struct {
	mu         sync.Mutex
	dsns       map[string]string        // database name → DSN
	pools      map[string]*pgxpool.Pool // database name → pool (lazily created)
	executions map[string]*executionHandles
}

// SQLHandleStore manages per-plugin SQL connection pools and per-execution handles.
type SQLHandleStore struct {
	mu      sync.RWMutex
	plugins map[string]*pluginSQLState
}

func NewSQLHandleStore() *SQLHandleStore {
	return &SQLHandleStore{
		plugins: make(map[string]*pluginSQLState),
	}
}

// RegisterDSN stores a named DSN for a plugin. Called during plugin load.
// name is the logical database name (e.g. "default", "analytics").
// The DSN is automatically scoped to the plugin's schema (plugin_{pluginID}).
func (s *SQLHandleStore) RegisterDSN(pluginID, name, dsn string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ps, ok := s.plugins[pluginID]
	if !ok {
		ps = &pluginSQLState{
			dsns:       make(map[string]string),
			pools:      make(map[string]*pgxpool.Pool),
			executions: make(map[string]*executionHandles),
		}
		s.plugins[pluginID] = ps
	}
	ps.dsns[name] = injectSearchPath(dsn, pluginSchemaName(pluginID))
}

// UnregisterPlugin closes the pool and removes all state for a plugin.
func (s *SQLHandleStore) UnregisterPlugin(pluginID string) {
	s.mu.Lock()
	ps, ok := s.plugins[pluginID]
	if !ok {
		s.mu.Unlock()
		return
	}
	delete(s.plugins, pluginID)
	s.mu.Unlock()

	ps.mu.Lock()
	defer ps.mu.Unlock()

	for execID := range ps.executions {
		cleanupExecutionLocked(ps, execID)
	}

	for name, pool := range ps.pools {
		pool.Close()
		delete(ps.pools, name)
	}
}

func (s *SQLHandleStore) getPluginState(pluginID string) (*pluginSQLState, error) {
	s.mu.RLock()
	ps, ok := s.plugins[pluginID]
	s.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("no SQL DSN configured for plugin %q", pluginID)
	}
	return ps, nil
}

// getOrCreatePool lazily creates the pgxpool.Pool for the named database.
func (s *SQLHandleStore) getOrCreatePool(ctx context.Context, pluginID, dbName string) (*pgxpool.Pool, error) {
	ps, err := s.getPluginState(pluginID)
	if err != nil {
		return nil, err
	}

	ps.mu.Lock()
	defer ps.mu.Unlock()

	if pool, ok := ps.pools[dbName]; ok {
		return pool, nil
	}

	dsn, ok := ps.dsns[dbName]
	if !ok || dsn == "" {
		return nil, fmt.Errorf("no SQL DSN configured for plugin %q database %q", pluginID, dbName)
	}

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("create pool for plugin %q database %q: %w", pluginID, dbName, err)
	}

	schema := pgx.Identifier{pluginSchemaName(pluginID)}.Sanitize()
	if _, err := pool.Exec(ctx, "CREATE SCHEMA IF NOT EXISTS "+schema); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ensure plugin schema for %q: %w", pluginID, err)
	}

	ps.pools[dbName] = pool
	return pool, nil
}

func (s *SQLHandleStore) getExecHandles(pluginID, execID string) (*pluginSQLState, *executionHandles, error) {
	ps, err := s.getPluginState(pluginID)
	if err != nil {
		return nil, nil, err
	}

	ps.mu.Lock()
	eh, ok := ps.executions[execID]
	if !ok {
		eh = &executionHandles{
			nextID:  1,
			handles: make(map[uint32]*sqlHandle),
		}
		ps.executions[execID] = eh
	}
	ps.mu.Unlock()

	return ps, eh, nil
}

// Alloc allocates a new handle for the given execution. Returns the handle ID.
func (s *SQLHandleStore) Alloc(pluginID, execID string, h *sqlHandle) (uint32, error) {
	_, eh, err := s.getExecHandles(pluginID, execID)
	if err != nil {
		return 0, err
	}

	eh.mu.Lock()
	defer eh.mu.Unlock()

	if len(eh.handles) >= wasmrt.SQLMaxHandlesPerExecution {
		return 0, fmt.Errorf("too many open SQL handles: max %d per execution", wasmrt.SQLMaxHandlesPerExecution)
	}

	id := eh.nextID
	eh.nextID++
	eh.handles[id] = h
	return id, nil
}

// Get retrieves a handle by ID.
func (s *SQLHandleStore) Get(pluginID, execID string, id uint32) (*sqlHandle, error) {
	_, eh, err := s.getExecHandles(pluginID, execID)
	if err != nil {
		return nil, err
	}

	eh.mu.Lock()
	defer eh.mu.Unlock()

	h, ok := eh.handles[id]
	if !ok {
		return nil, fmt.Errorf("SQL handle %d not found", id)
	}
	return h, nil
}

// Remove removes a handle by ID and returns it for the caller to close resources.
func (s *SQLHandleStore) Remove(pluginID, execID string, id uint32) (*sqlHandle, error) {
	_, eh, err := s.getExecHandles(pluginID, execID)
	if err != nil {
		return nil, err
	}

	eh.mu.Lock()
	defer eh.mu.Unlock()

	h, ok := eh.handles[id]
	if !ok {
		return nil, fmt.Errorf("SQL handle %d not found", id)
	}
	delete(eh.handles, id)
	return h, nil
}

// CleanupExecution releases all resources for a given execution.
// Rolls back transactions, closes rows, releases connections.
func (s *SQLHandleStore) CleanupExecution(pluginID, execID string) {
	s.mu.RLock()
	ps, ok := s.plugins[pluginID]
	s.mu.RUnlock()
	if !ok {
		return
	}

	ps.mu.Lock()
	defer ps.mu.Unlock()
	cleanupExecutionLocked(ps, execID)
}

func cleanupExecutionLocked(ps *pluginSQLState, execID string) {
	eh, ok := ps.executions[execID]
	if !ok {
		return
	}
	delete(ps.executions, execID)

	eh.mu.Lock()
	defer eh.mu.Unlock()

	// Close in dependency order: rows → transactions → connections.
	for _, h := range eh.handles {
		if h.kind == handleRows && h.rows != nil {
			h.rows.Close()
		}
	}
	for id, h := range eh.handles {
		if h.kind == handleTx && h.tx != nil {
			if err := h.tx.Rollback(context.Background()); err != nil {
				slog.Debug("sql cleanup: rollback tx", "exec", execID, "handle", id, "error", err)
			}
		}
	}
	for _, h := range eh.handles {
		if h.kind == handleConn && h.conn != nil {
			h.conn.Release()
		}
	}
	clear(eh.handles)
}

// HasDSN checks whether a named database DSN is registered for a plugin.
func (s *SQLHandleStore) HasDSN(pluginID, dbName string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ps, ok := s.plugins[pluginID]
	if !ok {
		return false
	}
	dsn, ok := ps.dsns[dbName]
	return ok && dsn != ""
}

// DSN returns the DSN for a named database, or empty string if not found.
func (s *SQLHandleStore) DSN(pluginID, dbName string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ps, ok := s.plugins[pluginID]
	if !ok {
		return ""
	}
	return ps.dsns[dbName]
}
