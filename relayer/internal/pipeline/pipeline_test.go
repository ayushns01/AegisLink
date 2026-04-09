package pipeline

import (
	"context"
	"errors"
	"math/big"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	bridgetypes "github.com/ayushns01/aegislink/chain/aegislink/x/bridge/types"
	"github.com/ayushns01/aegislink/relayer/internal/config"
	"github.com/ayushns01/aegislink/relayer/internal/cosmos"
	"github.com/ayushns01/aegislink/relayer/internal/evm"
	"github.com/ayushns01/aegislink/relayer/internal/replay"
)

func TestCoordinatorRunOnceRetriesTransientDepositSubmissionAndPersistsCheckpoint(t *testing.T) {
	t.Parallel()

	store := replay.NewStore()
	deposit := newPipelineDepositEvent()
	depositWatcher := &stubDepositWatcher{
		events:     []evm.DepositEvent{deposit},
		nextCursor: 11,
	}
	withdrawalWatcher := &stubWithdrawalWatcher{nextCursor: 0}
	collector := &stubCollector{}
	submitter := &stubCosmosSubmitter{
		errs: []error{
			cosmos.TemporaryError{Err: errors.New("rpc timeout")},
			nil,
		},
	}
	releaser := &stubEVMReleaser{}

	coordinator := New(
		config.Config{
			CosmosChainID:        "aegislink-1",
			AttestationThreshold: 2,
			SubmissionRetryLimit: 2,
		},
		store,
		depositWatcher,
		collector,
		submitter,
		withdrawalWatcher,
		releaser,
	)

	summary, err := coordinator.RunOnceWithSummary(context.Background())
	if err != nil {
		t.Fatalf("expected run to succeed, got error: %v", err)
	}
	if summary.DepositsObserved != 1 || summary.DepositsSubmitted != 1 {
		t.Fatalf("unexpected deposit summary: %+v", summary)
	}
	if summary.DepositSubmitAttempts != 2 {
		t.Fatalf("expected two deposit submit attempts, got %+v", summary)
	}
	if summary.WithdrawalsObserved != 0 || summary.WithdrawalsReleased != 0 {
		t.Fatalf("unexpected withdrawal summary: %+v", summary)
	}

	if len(collector.calls) != 1 {
		t.Fatalf("expected one attestation collection, got %d", len(collector.calls))
	}
	if len(submitter.claims) != 2 {
		t.Fatalf("expected two submit attempts, got %d", len(submitter.claims))
	}

	claim := submitter.claims[0]
	if err := claim.ValidateBasic(); err != nil {
		t.Fatalf("expected submitted claim to be valid, got %v", err)
	}
	if claim.DestinationChainID != "aegislink-1" {
		t.Fatalf("expected destination chain id to come from config, got %q", claim.DestinationChainID)
	}
	if collector.calls[0].payloadHash != claim.Digest() {
		t.Fatalf("expected collector to receive claim digest %q, got %q", claim.Digest(), collector.calls[0].payloadHash)
	}

	if !store.IsProcessed(deposit.ReplayKey()) {
		t.Fatalf("expected successful deposit to be marked processed")
	}
	if got := store.Checkpoint(depositCheckpointKey); got != 11 {
		t.Fatalf("expected deposit checkpoint 11, got %d", got)
	}
}

func TestCoordinatorRunOnceSuppressesDuplicateDepositAfterRestart(t *testing.T) {
	t.Parallel()

	store := replay.NewStore()
	deposit := newPipelineDepositEvent()
	if err := store.SaveCheckpoint(depositCheckpointKey, 5); err != nil {
		t.Fatalf("save checkpoint: %v", err)
	}
	if err := store.MarkProcessed(deposit.ReplayKey()); err != nil {
		t.Fatalf("mark processed: %v", err)
	}

	depositWatcher := &stubDepositWatcher{
		events:     []evm.DepositEvent{deposit, deposit},
		nextCursor: 11,
	}
	collector := &stubCollector{}
	submitter := &stubCosmosSubmitter{}
	coordinator := New(
		config.Config{
			CosmosChainID:        "aegislink-1",
			AttestationThreshold: 2,
		},
		store,
		depositWatcher,
		collector,
		submitter,
		&stubWithdrawalWatcher{},
		&stubEVMReleaser{},
	)

	if err := coordinator.RunOnce(context.Background()); err != nil {
		t.Fatalf("expected duplicate run to succeed, got error: %v", err)
	}

	if len(depositWatcher.calls) != 1 || depositWatcher.calls[0] != 5 {
		t.Fatalf("expected watcher to resume from persisted checkpoint 5, got %v", depositWatcher.calls)
	}
	if len(collector.calls) != 0 {
		t.Fatalf("expected duplicate deposit to skip attestation collection, got %d calls", len(collector.calls))
	}
	if len(submitter.claims) != 0 {
		t.Fatalf("expected duplicate deposit to skip submission, got %d attempts", len(submitter.claims))
	}
	if got := store.Checkpoint(depositCheckpointKey); got != 11 {
		t.Fatalf("expected checkpoint to advance to 11 after duplicate scan, got %d", got)
	}
}

func TestCoordinatorRunOnceSubmitsEthereumReleaseForObservedWithdrawal(t *testing.T) {
	t.Parallel()

	store := replay.NewStore()
	withdrawal := newPipelineWithdrawal()
	coordinator := New(
		config.Config{
			CosmosChainID:        "aegislink-1",
			SubmissionRetryLimit: 2,
		},
		store,
		&stubDepositWatcher{},
		&stubCollector{},
		&stubCosmosSubmitter{},
		&stubWithdrawalWatcher{
			withdrawals: []cosmos.Withdrawal{withdrawal},
			nextCursor:  22,
		},
		&stubEVMReleaser{},
	)

	releaser := coordinator.evmRelease.(*stubEVMReleaser)

	if err := coordinator.RunOnce(context.Background()); err != nil {
		t.Fatalf("expected withdrawal run to succeed, got error: %v", err)
	}

	if len(releaser.requests) != 1 {
		t.Fatalf("expected one release submission, got %d", len(releaser.requests))
	}
	request := releaser.requests[0]
	if request.MessageID != withdrawal.Identity.MessageID {
		t.Fatalf("expected message id %q, got %q", withdrawal.Identity.MessageID, request.MessageID)
	}
	if request.AssetAddress != withdrawal.AssetAddress {
		t.Fatalf("expected asset address %q, got %q", withdrawal.AssetAddress, request.AssetAddress)
	}
	if request.Amount.Cmp(withdrawal.Amount) != 0 {
		t.Fatalf("expected amount %s, got %s", withdrawal.Amount, request.Amount)
	}
	if string(request.Signature) != string(withdrawal.Signature) {
		t.Fatalf("expected signature %q, got %q", withdrawal.Signature, request.Signature)
	}
	if !store.IsProcessed(withdrawal.ReplayKey()) {
		t.Fatalf("expected withdrawal to be marked processed")
	}
	if got := store.Checkpoint(withdrawalCheckpointKey); got != 22 {
		t.Fatalf("expected withdrawal checkpoint 22, got %d", got)
	}
}

func TestCoordinatorRunOnceRetriesTransientWithdrawalReleaseAndPersistsCheckpoint(t *testing.T) {
	t.Parallel()

	store := replay.NewStoreAt(filepath.Join(t.TempDir(), "store.json"))
	withdrawal := newPipelineWithdrawal()
	coordinator := New(
		config.Config{
			CosmosChainID:        "aegislink-1",
			SubmissionRetryLimit: 2,
		},
		store,
		&stubDepositWatcher{},
		&stubCollector{},
		&stubCosmosSubmitter{},
		&stubWithdrawalWatcher{
			withdrawals: []cosmos.Withdrawal{withdrawal},
			nextCursor:  22,
		},
		&stubEVMReleaser{
			errs: []error{
				evm.TemporaryError{Err: errors.New("rpc timeout")},
				nil,
			},
		},
	)

	releaser := coordinator.evmRelease.(*stubEVMReleaser)

	if err := coordinator.RunOnce(context.Background()); err != nil {
		t.Fatalf("expected withdrawal retry run to succeed, got error: %v", err)
	}
	if len(releaser.requests) != 2 {
		t.Fatalf("expected two release attempts, got %d", len(releaser.requests))
	}
	if got := store.Checkpoint(withdrawalCheckpointKey); got != 22 {
		t.Fatalf("expected withdrawal checkpoint 22, got %d", got)
	}
	if !store.IsProcessed(withdrawal.ReplayKey()) {
		t.Fatalf("expected withdrawal to be marked processed after retry")
	}
}

func TestCoordinatorRunOnceFailsWhenReplayPersistenceFails(t *testing.T) {
	t.Parallel()

	blocked := filepath.Join(t.TempDir(), "blocked")
	if err := os.WriteFile(blocked, []byte("not-a-directory"), 0o644); err != nil {
		t.Fatalf("write blocker file: %v", err)
	}

	store := replay.NewStoreAt(filepath.Join(blocked, "store.json"))
	deposit := newPipelineDepositEvent()
	coordinator := New(
		config.Config{
			CosmosChainID:        "aegislink-1",
			AttestationThreshold: 2,
			SubmissionRetryLimit: 2,
		},
		store,
		&stubDepositWatcher{
			events:     []evm.DepositEvent{deposit},
			nextCursor: 11,
		},
		&stubCollector{},
		&stubCosmosSubmitter{},
		&stubWithdrawalWatcher{},
		&stubEVMReleaser{},
	)

	err := coordinator.RunOnce(context.Background())
	if err == nil {
		t.Fatalf("expected replay persistence failure")
	}
}

func TestCoordinatorRunOnceFailsWhenReplayLoadFails(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "store.json")
	if err := os.WriteFile(path, []byte("{invalid"), 0o644); err != nil {
		t.Fatalf("write invalid replay state: %v", err)
	}

	coordinator := New(
		config.Config{CosmosChainID: "aegislink-1"},
		replay.NewStoreAt(path),
		&stubDepositWatcher{},
		&stubCollector{},
		&stubCosmosSubmitter{},
		&stubWithdrawalWatcher{},
		&stubEVMReleaser{},
	)

	if err := coordinator.RunOnce(context.Background()); err == nil {
		t.Fatalf("expected replay load failure")
	}
}

func TestCoordinatorDaemonRunPollsWithoutDuplicateReprocessing(t *testing.T) {
	t.Parallel()

	store := replay.NewStore()
	deposit := newPipelineDepositEvent()
	depositWatcher := &stubDepositWatcher{
		events:     []evm.DepositEvent{deposit},
		nextCursor: 11,
	}
	submitter := &stubCosmosSubmitter{}

	coordinator := New(
		config.Config{
			CosmosChainID:        "aegislink-1",
			AttestationThreshold: 2,
		},
		store,
		depositWatcher,
		&stubCollector{},
		submitter,
		&stubWithdrawalWatcher{},
		&stubEVMReleaser{},
	)

	daemon := NewDaemon(coordinator, DaemonConfig{PollInterval: time.Millisecond})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- daemon.Run(ctx)
	}()

	deadline := time.Now().Add(250 * time.Millisecond)
	for time.Now().Before(deadline) {
		if len(submitter.claims) == 1 && len(depositWatcher.calls) >= 2 {
			break
		}
		time.Sleep(time.Millisecond)
	}
	cancel()

	if err := <-done; err != nil {
		t.Fatalf("daemon run: %v", err)
	}
	if len(submitter.claims) != 1 {
		t.Fatalf("expected one deposit submission across polling loop, got %d", len(submitter.claims))
	}
	if len(depositWatcher.calls) < 2 {
		t.Fatalf("expected repeated polling calls, got %v", depositWatcher.calls)
	}
	if got := store.Checkpoint(depositCheckpointKey); got != 11 {
		t.Fatalf("expected checkpoint 11, got %d", got)
	}
}

func TestCoordinatorDaemonRunBacksOffAfterTemporaryFailure(t *testing.T) {
	t.Parallel()

	depositWatcher := &countingDepositWatcher{
		errs: []error{
			evm.TemporaryError{Err: errors.New("rpc timeout")},
			nil,
			nil,
		},
		nextCursor: 22,
	}

	coordinator := New(
		config.Config{CosmosChainID: "aegislink-1"},
		replay.NewStore(),
		depositWatcher,
		&stubCollector{},
		&stubCosmosSubmitter{},
		&stubWithdrawalWatcher{},
		&stubEVMReleaser{},
	)

	daemon := NewDaemon(coordinator, DaemonConfig{
		PollInterval:      time.Millisecond,
		FailureBackoff:    40 * time.Millisecond,
		MaxConsecutiveRuns: 3,
	})

	start := time.Now()
	if err := daemon.Run(context.Background()); err != nil {
		t.Fatalf("daemon run: %v", err)
	}
	if got := depositWatcher.calls.Load(); got < 2 {
		t.Fatalf("expected retry after temporary error, got %d observe calls", got)
	}
	if elapsed := time.Since(start); elapsed < 35*time.Millisecond {
		t.Fatalf("expected backoff delay, run finished too quickly: %s", elapsed)
	}
}

func TestCoordinatorDaemonRunStopsGracefullyOnContextCancel(t *testing.T) {
	t.Parallel()

	coordinator := New(
		config.Config{CosmosChainID: "aegislink-1"},
		replay.NewStore(),
		&blockingDepositWatcher{},
		&stubCollector{},
		&stubCosmosSubmitter{},
		&stubWithdrawalWatcher{},
		&stubEVMReleaser{},
	)

	daemon := NewDaemon(coordinator, DaemonConfig{PollInterval: 5 * time.Millisecond})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- daemon.Run(ctx)
	}()

	time.Sleep(10 * time.Millisecond)
	cancel()

	if err := <-done; err != nil {
		t.Fatalf("expected graceful shutdown, got %v", err)
	}
}

type stubDepositWatcher struct {
	events     []evm.DepositEvent
	nextCursor uint64
	err        error
	calls      []uint64
}

func (s *stubDepositWatcher) Observe(_ context.Context, fromBlock uint64) ([]evm.DepositEvent, uint64, error) {
	s.calls = append(s.calls, fromBlock)
	return append([]evm.DepositEvent(nil), s.events...), s.nextCursor, s.err
}

type countingDepositWatcher struct {
	errs       []error
	nextCursor uint64
	calls      atomic.Int32
}

func (w *countingDepositWatcher) Observe(_ context.Context, fromBlock uint64) ([]evm.DepositEvent, uint64, error) {
	call := int(w.calls.Add(1))
	if call <= len(w.errs) && w.errs[call-1] != nil {
		return nil, fromBlock, w.errs[call-1]
	}
	return nil, w.nextCursor, nil
}

type blockingDepositWatcher struct {
	once sync.Once
}

func (w *blockingDepositWatcher) Observe(ctx context.Context, fromBlock uint64) ([]evm.DepositEvent, uint64, error) {
	w.once.Do(func() {
		<-ctx.Done()
	})
	return nil, fromBlock, ctx.Err()
}

type collectorCall struct {
	messageID   string
	payloadHash string
}

type stubCollector struct {
	calls []collectorCall
	err   error
}

func (s *stubCollector) Collect(_ context.Context, messageID, payloadHash string) (bridgetypes.Attestation, error) {
	s.calls = append(s.calls, collectorCall{messageID: messageID, payloadHash: payloadHash})
	if s.err != nil {
		return bridgetypes.Attestation{}, s.err
	}
	return bridgetypes.Attestation{
		MessageID:        messageID,
		PayloadHash:      payloadHash,
		Signers:          []string{"signer-1", "signer-2"},
		Proofs: []bridgetypes.AttestationProof{
			{Signer: "signer-1", Signature: []byte{1}},
			{Signer: "signer-2", Signature: []byte{2}},
		},
		Threshold:        2,
		Expiry:           200,
		SignerSetVersion: 1,
	}, nil
}

type stubCosmosSubmitter struct {
	claims       []bridgetypes.DepositClaim
	attestations []bridgetypes.Attestation
	errs         []error
}

func (s *stubCosmosSubmitter) SubmitDepositClaim(_ context.Context, claim bridgetypes.DepositClaim, attestation bridgetypes.Attestation) error {
	s.claims = append(s.claims, claim)
	s.attestations = append(s.attestations, attestation)

	if len(s.errs) == 0 {
		return nil
	}
	err := s.errs[0]
	s.errs = s.errs[1:]
	return err
}

type stubWithdrawalWatcher struct {
	withdrawals []cosmos.Withdrawal
	nextCursor  uint64
	err         error
	calls       []uint64
}

func (s *stubWithdrawalWatcher) Observe(_ context.Context, fromHeight uint64) ([]cosmos.Withdrawal, uint64, error) {
	s.calls = append(s.calls, fromHeight)
	return append([]cosmos.Withdrawal(nil), s.withdrawals...), s.nextCursor, s.err
}

type stubEVMReleaser struct {
	requests []evm.ReleaseRequest
	errs     []error
}

func (s *stubEVMReleaser) ReleaseWithdrawal(_ context.Context, request evm.ReleaseRequest) (string, error) {
	s.requests = append(s.requests, request)

	if len(s.errs) == 0 {
		return "release-id", nil
	}
	err := s.errs[0]
	s.errs = s.errs[1:]
	if err != nil {
		return "", err
	}
	return "release-id", nil
}

func newPipelineDepositEvent() evm.DepositEvent {
	return evm.DepositEvent{
		BlockNumber:    10,
		SourceChainID:  "11155111",
		SourceContract: "0xgateway",
		TxHash:         "0xdeposit-tx",
		LogIndex:       1,
		Nonce:          7,
		DepositID:      "deposit-7",
		MessageID:      "message-7",
		AssetAddress:   "0xasset",
		AssetID:        "uusdc",
		Amount:         big.NewInt(99),
		Recipient:      "aegis1recipient",
		Expiry:         150,
	}
}

func newPipelineWithdrawal() cosmos.Withdrawal {
	identity := bridgetypes.ClaimIdentity{
		Kind:           bridgetypes.ClaimKindWithdrawal,
		SourceChainID:  "aegislink-1",
		SourceContract: "bridge",
		SourceTxHash:   "0xwithdrawal-tx",
		SourceLogIndex: 4,
		Nonce:          9,
	}
	identity.MessageID = identity.DerivedMessageID()

	return cosmos.Withdrawal{
		BlockHeight:  21,
		Identity:     identity,
		AssetID:      "uusdc",
		AssetAddress: "0xasset",
		Amount:       big.NewInt(75),
		Recipient:    "0xrecipient",
		Deadline:     300,
		Signature:    []byte("proof"),
	}
}
