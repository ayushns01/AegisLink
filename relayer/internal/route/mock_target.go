package route

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type MockTargetMode string

const (
	MockTargetModeSuccess MockTargetMode = "success"
	MockTargetModeFail    MockTargetMode = "fail"
	MockTargetModeTimeout MockTargetMode = "timeout"
)

type MockTarget struct {
	mode      MockTargetMode
	delay     time.Duration
	statePath string

	mu       sync.Mutex
	received []Transfer
}

func NewMockTargetHandler(mode string, statePath string, delay time.Duration) http.Handler {
	target := &MockTarget{
		mode:      normalizeMockTargetMode(mode),
		delay:     delay,
		statePath: strings.TrimSpace(statePath),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/transfers", target.handleTransfers)
	return mux
}

func (t *MockTarget) handleTransfers(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		t.mu.Lock()
		defer t.mu.Unlock()
		_ = json.NewEncoder(w).Encode(map[string]any{"transfers": t.received})
	case http.MethodPost:
		var transfer Transfer
		if err := json.NewDecoder(r.Body).Decode(&transfer); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		t.mu.Lock()
		t.received = append(t.received, transfer)
		_ = persistMockTargetState(t.statePath, t.received)
		t.mu.Unlock()

		if t.delay > 0 {
			select {
			case <-r.Context().Done():
				return
			case <-time.After(t.delay):
			}
		}

		switch t.mode {
		case MockTargetModeFail:
			_ = json.NewEncoder(w).Encode(Ack{
				Status: AckStatusFailed,
				Reason: "mock ack failed",
			})
		case MockTargetModeTimeout:
			select {
			case <-r.Context().Done():
			case <-time.After(5 * time.Second):
			}
		default:
			_ = json.NewEncoder(w).Encode(Ack{Status: AckStatusCompleted})
		}
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func normalizeMockTargetMode(mode string) MockTargetMode {
	switch MockTargetMode(strings.TrimSpace(mode)) {
	case MockTargetModeFail:
		return MockTargetModeFail
	case MockTargetModeTimeout:
		return MockTargetModeTimeout
	default:
		return MockTargetModeSuccess
	}
}

func persistMockTargetState(path string, transfers []Transfer) error {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(map[string]any{"transfers": transfers}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
