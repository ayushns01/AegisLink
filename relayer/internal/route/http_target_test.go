package route

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestHTTPTargetSubmitsTransferAndParsesAck(t *testing.T) {
	t.Parallel()

	target := newHTTPTargetWithClient("http://mock-osmosis", &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.Path != "/transfers" {
				t.Fatalf("expected /transfers, got %s", req.URL.Path)
			}
			payload, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("read body: %v", err)
			}
			if string(payload) != `{"transfer_id":"ibc/eth.usdc/1","asset_id":"","amount":"","receiver":"","destination_chain_id":"","channel_id":"","destination_denom":"","timeout_height":0,"memo":"","status":"","failure_reason":""}` {
				t.Fatalf("unexpected payload: %s", payload)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"status":"completed"}`)),
				Header:     make(http.Header),
			}, nil
		}),
	})
	ack, err := target.SubmitTransfer(context.Background(), Transfer{TransferID: "ibc/eth.usdc/1"})
	if err != nil {
		t.Fatalf("submit transfer: %v", err)
	}
	if ack.Status != AckStatusCompleted {
		t.Fatalf("expected completed ack, got %q", ack.Status)
	}
}

func TestHTTPTargetReturnsTimeoutError(t *testing.T) {
	t.Parallel()

	target := newHTTPTargetWithClient("http://mock-osmosis", &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			return nil, context.DeadlineExceeded
		}),
	})
	_, err := target.SubmitTransfer(context.Background(), Transfer{TransferID: "ibc/eth.usdc/1"})
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if _, ok := err.(TimeoutError); !ok {
		t.Fatalf("expected TimeoutError, got %T", err)
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
