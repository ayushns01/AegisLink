package networked

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"time"
)

type Status struct {
	ReadyState
	Healthy bool `json:"healthy"`
}

func ReadStatus(ctx context.Context, cfg Config) (Status, error) {
	resolved, _, err := ResolveConfig(cfg)
	if err != nil {
		return Status{}, err
	}

	data, err := os.ReadFile(resolved.ReadyFile)
	if err != nil {
		return Status{}, err
	}

	var ready ReadyState
	if err := json.Unmarshal(data, &ready); err != nil {
		return Status{}, err
	}

	status := Status{
		ReadyState: ready,
	}

	requestCtx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(requestCtx, http.MethodGet, "http://"+ready.RPCAddress+"/healthz", nil)
	if err != nil {
		return Status{}, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err == nil {
		defer resp.Body.Close()
		status.Healthy = resp.StatusCode == http.StatusOK
	}

	return status, nil
}
