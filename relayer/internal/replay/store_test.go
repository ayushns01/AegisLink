package replay

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStorePersistsCheckpointsAndProcessedKeys(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "store.json")
	store := NewStoreAt(path)

	if got := store.Checkpoint("evm-deposits"); got != 0 {
		t.Fatalf("expected empty checkpoint to default to 0, got %d", got)
	}
	if store.IsProcessed("message-1") {
		t.Fatalf("expected empty store to report no processed keys")
	}

	if err := store.SaveCheckpoint("evm-deposits", 14); err != nil {
		t.Fatalf("save checkpoint: %v", err)
	}
	if err := store.MarkProcessed("message-1"); err != nil {
		t.Fatalf("mark processed: %v", err)
	}

	reopened := NewStoreAt(path)
	if got := reopened.Checkpoint("evm-deposits"); got != 14 {
		t.Fatalf("expected checkpoint 14 after reopen, got %d", got)
	}
	if !reopened.IsProcessed("message-1") {
		t.Fatalf("expected processed key to persist across reopen")
	}
}

func TestStoreSaveCheckpointReturnsPersistError(t *testing.T) {
	t.Parallel()

	blocked := filepath.Join(t.TempDir(), "blocked")
	if err := os.WriteFile(blocked, []byte("not-a-directory"), 0o644); err != nil {
		t.Fatalf("write blocker file: %v", err)
	}

	store := NewStoreAt(filepath.Join(blocked, "store.json"))
	if err := store.SaveCheckpoint("evm-deposits", 14); err == nil {
		t.Fatalf("expected checkpoint persistence error")
	}
}

func TestStoreMarkProcessedReturnsPersistError(t *testing.T) {
	t.Parallel()

	blocked := filepath.Join(t.TempDir(), "blocked")
	if err := os.WriteFile(blocked, []byte("not-a-directory"), 0o644); err != nil {
		t.Fatalf("write blocker file: %v", err)
	}

	store := NewStoreAt(filepath.Join(blocked, "store.json"))
	if err := store.MarkProcessed("message-1"); err == nil {
		t.Fatalf("expected processed-key persistence error")
	}
}

func TestNewStoreAtTracksLoadErrorForInvalidJSON(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "store.json")
	if err := os.WriteFile(path, []byte("{invalid"), 0o644); err != nil {
		t.Fatalf("write invalid replay state: %v", err)
	}

	store := NewStoreAt(path)
	if err := store.Err(); err == nil {
		t.Fatalf("expected replay load error for invalid json")
	}
}
