package route

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type HTTPTarget struct {
	baseURL string
	client  *http.Client
}

func NewHTTPTarget(baseURL string, timeout time.Duration) *HTTPTarget {
	if timeout <= 0 {
		timeout = time.Second
	}
	return newHTTPTargetWithClient(baseURL, &http.Client{Timeout: timeout})
}

func newHTTPTargetWithClient(baseURL string, client *http.Client) *HTTPTarget {
	if client == nil {
		client = &http.Client{Timeout: time.Second}
	}
	return &HTTPTarget{
		baseURL: strings.TrimRight(baseURL, "/"),
		client:  client,
	}
}

func (t *HTTPTarget) SubmitTransfer(ctx context.Context, transfer Transfer) (Ack, error) {
	envelope, err := buildDeliveryEnvelope(transfer)
	if err != nil {
		return Ack{}, err
	}

	body, err := json.Marshal(envelope)
	if err != nil {
		return Ack{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.baseURL+"/transfers", bytes.NewReader(body))
	if err != nil {
		return Ack{}, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.client.Do(req)
	if err != nil {
		if isTimeout(err) {
			return Ack{}, TimeoutError{Err: err}
		}
		return Ack{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return Ack{}, fmt.Errorf("route target status %d", resp.StatusCode)
	}

	var ack Ack
	if err := json.NewDecoder(resp.Body).Decode(&ack); err != nil {
		return Ack{}, err
	}
	if ack.Status == "" {
		return Ack{}, errors.New("route target returned empty status")
	}
	return ack, nil
}

func (t *HTTPTarget) ReadyAcks(ctx context.Context) ([]AckRecord, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, t.baseURL+"/acks", nil)
	if err != nil {
		return nil, err
	}

	resp, err := t.client.Do(req)
	if err != nil {
		if isTimeout(err) {
			return nil, TimeoutError{Err: err}
		}
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("route target status %d", resp.StatusCode)
	}

	var acks []AckRecord
	if err := json.NewDecoder(resp.Body).Decode(&acks); err != nil {
		return nil, err
	}
	return acks, nil
}

func (t *HTTPTarget) ConfirmAck(ctx context.Context, transferID string) error {
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		t.baseURL+"/acks/confirm?transfer_id="+url.QueryEscape(strings.TrimSpace(transferID)),
		nil,
	)
	if err != nil {
		return err
	}

	resp, err := t.client.Do(req)
	if err != nil {
		if isTimeout(err) {
			return TimeoutError{Err: err}
		}
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		if len(body) == 0 {
			return fmt.Errorf("route target status %d", resp.StatusCode)
		}
		return fmt.Errorf("route target status %d: %s", resp.StatusCode, body)
	}
	return nil
}

func isTimeout(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	var urlErr *url.Error
	return errors.As(err, &urlErr) && urlErr.Timeout()
}
