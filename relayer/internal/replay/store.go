package replay

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

const defaultStoreFile = "aegislink-relayer-replay.json"

type Store struct {
	mu          sync.Mutex
	path        string
	checkpoints map[string]uint64
	processed   map[string]struct{}
	loadErr     error
}

type persistedState struct {
	Checkpoints map[string]uint64 `json:"checkpoints"`
	Processed   []string          `json:"processed"`
}

func NewStore() *Store {
	return &Store{
		checkpoints: make(map[string]uint64),
		processed:   make(map[string]struct{}),
	}
}

func NewStoreAt(path string) *Store {
	if path == "" {
		path = filepath.Join(os.TempDir(), defaultStoreFile)
	}

	store := &Store{
		path:        path,
		checkpoints: make(map[string]uint64),
		processed:   make(map[string]struct{}),
	}
	store.loadErr = store.load()
	return store
}

func (s *Store) Checkpoint(key string) uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.checkpoints[key]
}

func (s *Store) Err() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.loadErr
}

func (s *Store) SaveCheckpoint(key string, value uint64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.loadErr != nil {
		return fmt.Errorf("replay store unavailable: %w", s.loadErr)
	}
	s.checkpoints[key] = value
	return s.persistLocked()
}

func (s *Store) MarkProcessed(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.loadErr != nil {
		return fmt.Errorf("replay store unavailable: %w", s.loadErr)
	}
	s.processed[key] = struct{}{}
	return s.persistLocked()
}

func (s *Store) IsProcessed(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, ok := s.processed[key]
	return ok
}

func (s *Store) load() error {
	if s.path == "" {
		return nil
	}

	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read replay state: %w", err)
	}

	var state persistedState
	if err := json.Unmarshal(data, &state); err != nil {
		return fmt.Errorf("decode replay state: %w", err)
	}

	for key, value := range state.Checkpoints {
		s.checkpoints[key] = value
	}
	for _, key := range state.Processed {
		s.processed[key] = struct{}{}
	}
	return nil
}

func (s *Store) persistLocked() error {
	if s.path == "" {
		return nil
	}

	state := persistedState{
		Checkpoints: make(map[string]uint64, len(s.checkpoints)),
		Processed:   make([]string, 0, len(s.processed)),
	}

	for key, value := range s.checkpoints {
		state.Checkpoints[key] = value
	}
	for key := range s.processed {
		state.Processed = append(state.Processed, key)
	}

	encoded, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal replay state: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("mkdir replay state dir: %w", err)
	}

	dir := filepath.Dir(s.path)
	tmp, err := os.CreateTemp(dir, "replay-*.json")
	if err != nil {
		return fmt.Errorf("create temp replay state file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err := tmp.Write(encoded); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp replay state file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp replay state file: %w", err)
	}
	if err := os.Rename(tmpName, s.path); err != nil {
		return fmt.Errorf("rename replay state file: %w", err)
	}
	return nil
}
