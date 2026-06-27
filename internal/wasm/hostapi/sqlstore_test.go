package hostapi

import (
	"fmt"
	"sync"
	"testing"

	wasmrt "SuperBotGo/internal/wasm/runtime"
)

func TestSQLHandleStore_AllocAndGet(t *testing.T) {
	store := NewSQLHandleStore()
	store.RegisterDSN("p1", "default", "postgres://localhost/test")

	id, err := store.Alloc("p1", "exec1", &sqlHandle{kind: handleConn})
	if err != nil {
		t.Fatalf("Alloc: %v", err)
	}
	if id == 0 {
		t.Fatal("expected non-zero handle ID")
	}

	h, err := store.Get("p1", "exec1", id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if h.kind != handleConn {
		t.Fatalf("expected handleConn, got %d", h.kind)
	}
}

func TestSQLHandleStore_HandleNotFound(t *testing.T) {
	store := NewSQLHandleStore()
	store.RegisterDSN("p1", "default", "postgres://localhost/test")

	_, err := store.Get("p1", "exec1", 999)
	if err == nil {
		t.Fatal("expected error for non-existent handle")
	}
}

func TestSQLHandleStore_PluginNotRegistered(t *testing.T) {
	store := NewSQLHandleStore()

	_, err := store.Alloc("unknown", "exec1", &sqlHandle{kind: handleConn})
	if err == nil {
		t.Fatal("expected error for unregistered plugin")
	}
}

func TestSQLHandleStore_PluginIsolation(t *testing.T) {
	store := NewSQLHandleStore()
	store.RegisterDSN("p1", "default", "postgres://localhost/test1")
	store.RegisterDSN("p2", "default", "postgres://localhost/test2")

	id1, _ := store.Alloc("p1", "exec1", &sqlHandle{kind: handleConn})
	id2, _ := store.Alloc("p2", "exec1", &sqlHandle{kind: handleTx})

	h1, _ := store.Get("p1", "exec1", id1)
	h2, _ := store.Get("p2", "exec1", id2)

	if h1.kind != handleConn {
		t.Fatalf("p1 handle: expected handleConn, got %d", h1.kind)
	}
	if h2.kind != handleTx {
		t.Fatalf("p2 handle: expected handleTx, got %d", h2.kind)
	}

	// Handles are scoped per-plugin: same ID in different plugins → different data.
	// Both start at ID 1, but they resolve to different handles.
	if h1.kind == h2.kind {
		t.Fatal("expected different handle kinds for different plugins")
	}
}

func TestSQLHandleStore_ExecutionIsolation(t *testing.T) {
	store := NewSQLHandleStore()
	store.RegisterDSN("p1", "default", "postgres://localhost/test")

	id1, _ := store.Alloc("p1", "exec1", &sqlHandle{kind: handleConn})
	id2, _ := store.Alloc("p1", "exec2", &sqlHandle{kind: handleTx})

	// Both executions start at ID 1, but resolve to different handles.
	h1, _ := store.Get("p1", "exec1", id1)
	h2, _ := store.Get("p1", "exec2", id2)
	if h1.kind != handleConn {
		t.Fatalf("exec1: expected handleConn, got %d", h1.kind)
	}
	if h2.kind != handleTx {
		t.Fatalf("exec2: expected handleTx, got %d", h2.kind)
	}

	// Cleanup exec1 should not affect exec2.
	store.CleanupExecution("p1", "exec1")

	_, err := store.Get("p1", "exec1", id1)
	if err == nil {
		t.Fatal("exec1 handle should be cleaned up")
	}

	h2after, err := store.Get("p1", "exec2", id2)
	if err != nil {
		t.Fatalf("exec2 handle should still exist: %v", err)
	}
	if h2after.kind != handleTx {
		t.Fatalf("exec2 handle should still be handleTx, got %d", h2after.kind)
	}
}

func TestSQLHandleStore_Remove(t *testing.T) {
	store := NewSQLHandleStore()
	store.RegisterDSN("p1", "default", "postgres://localhost/test")

	id, _ := store.Alloc("p1", "exec1", &sqlHandle{kind: handleRows})

	removed, err := store.Remove("p1", "exec1", id)
	if err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if removed.kind != handleRows {
		t.Fatalf("expected handleRows, got %d", removed.kind)
	}

	// Should not be findable anymore.
	_, err = store.Get("p1", "exec1", id)
	if err == nil {
		t.Fatal("expected error after Remove")
	}
}

func TestSQLHandleStore_MaxHandlesLimit(t *testing.T) {
	store := NewSQLHandleStore()
	store.RegisterDSN("p1", "default", "postgres://localhost/test")

	for i := 0; i < wasmrt.SQLMaxHandlesPerExecution; i++ {
		_, err := store.Alloc("p1", "exec1", &sqlHandle{kind: handleConn})
		if err != nil {
			t.Fatalf("Alloc %d: %v", i, err)
		}
	}

	// One more should fail.
	_, err := store.Alloc("p1", "exec1", &sqlHandle{kind: handleConn})
	if err == nil {
		t.Fatal("expected error for exceeding max handles")
	}
}

func TestSQLHandleStore_CleanupExecution(t *testing.T) {
	store := NewSQLHandleStore()
	store.RegisterDSN("p1", "default", "postgres://localhost/test")

	// Alloc handles with nil resources (safe for cleanup).
	store.Alloc("p1", "exec1", &sqlHandle{kind: handleConn})
	store.Alloc("p1", "exec1", &sqlHandle{kind: handleTx})
	store.Alloc("p1", "exec1", &sqlHandle{kind: handleRows})

	store.CleanupExecution("p1", "exec1")

	// All handles should be gone.
	_, err := store.Get("p1", "exec1", 1)
	if err == nil {
		t.Fatal("expected handles to be cleaned up")
	}
}

func TestSQLHandleStore_CleanupNonexistent(t *testing.T) {
	store := NewSQLHandleStore()

	// Should not panic.
	store.CleanupExecution("unknown", "exec1")
}

func TestSQLHandleStore_UnregisterPlugin(t *testing.T) {
	store := NewSQLHandleStore()
	store.RegisterDSN("p1", "default", "postgres://localhost/test")

	store.Alloc("p1", "exec1", &sqlHandle{kind: handleConn})

	store.UnregisterPlugin("p1")

	// Plugin should be gone.
	_, err := store.Alloc("p1", "exec1", &sqlHandle{kind: handleConn})
	if err == nil {
		t.Fatal("expected error after UnregisterPlugin")
	}
}

func TestSQLHandleStore_HasDSN(t *testing.T) {
	store := NewSQLHandleStore()

	if store.HasDSN("p1", "default") {
		t.Fatal("expected no DSN before registration")
	}

	store.RegisterDSN("p1", "default", "postgres://localhost/test")

	if !store.HasDSN("p1", "default") {
		t.Fatal("expected DSN after registration")
	}

	if store.HasDSN("p1", "analytics") {
		t.Fatal("expected no DSN for unregistered database name")
	}

	store.UnregisterPlugin("p1")

	if store.HasDSN("p1", "default") {
		t.Fatal("expected no DSN after unregister")
	}
}

func TestSQLHandleStore_NamedDatabases(t *testing.T) {
	store := NewSQLHandleStore()
	store.RegisterDSN("p1", "default", "postgres://localhost/main")
	store.RegisterDSN("p1", "analytics", "postgres://localhost/analytics")

	if !store.HasDSN("p1", "default") {
		t.Fatal("expected default DSN")
	}
	if !store.HasDSN("p1", "analytics") {
		t.Fatal("expected analytics DSN")
	}
	if store.DSN("p1", "default") != "postgres://localhost/main?search_path=plugin_p1" {
		t.Fatalf("unexpected default DSN: %s", store.DSN("p1", "default"))
	}
	if store.DSN("p1", "analytics") != "postgres://localhost/analytics?search_path=plugin_p1" {
		t.Fatalf("unexpected analytics DSN: %s", store.DSN("p1", "analytics"))
	}
}

func TestSQLHandleStore_HandleIDsIncrement(t *testing.T) {
	store := NewSQLHandleStore()
	store.RegisterDSN("p1", "default", "postgres://localhost/test")

	id1, _ := store.Alloc("p1", "exec1", &sqlHandle{kind: handleConn})
	id2, _ := store.Alloc("p1", "exec1", &sqlHandle{kind: handleTx})
	id3, _ := store.Alloc("p1", "exec1", &sqlHandle{kind: handleRows})

	if id1 != 1 || id2 != 2 || id3 != 3 {
		t.Fatalf("expected sequential IDs (1,2,3), got (%d,%d,%d)", id1, id2, id3)
	}
}

func TestSQLHandleStore_ConcurrentAccess(t *testing.T) {
	store := NewSQLHandleStore()

	for i := 0; i < 10; i++ {
		store.RegisterDSN(fmt.Sprintf("p%d", i), "default", "postgres://localhost/test")
	}

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			pluginID := fmt.Sprintf("p%d", i%10)
			execID := fmt.Sprintf("exec%d", i%5)

			id, err := store.Alloc(pluginID, execID, &sqlHandle{kind: handleConn})
			if err != nil {
				return // max handles reached, ok
			}
			store.Get(pluginID, execID, id)
			store.Remove(pluginID, execID, id)
		}(i)
	}
	wg.Wait()

	// If we got here without panics, concurrency is safe.
}
