package sms

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

// stubSender lets tests inject either a fixed receipt or a fixed error.
type stubSender struct {
	receipt Receipt
	err     error
	calls   atomic.Int32
}

func (s *stubSender) Send(_ context.Context, _ Message) (Receipt, error) {
	s.calls.Add(1)
	return s.receipt, s.err
}

// stubJob captures which lifecycle method the dispatcher invoked, so
// the test can assert the success/backoff/error routing.
type stubJob struct {
	msg     Message
	sender  Sender
	gsErr   error
	rErr    error
	receipt Receipt
	state   string // "success" | "backoff" | "error"
	bErr    error
}

func (j *stubJob) Render() (Message, error)        { return j.msg, j.rErr }
func (j *stubJob) GetSender() (Sender, error)      { return j.sender, j.gsErr }
func (j *stubJob) Success(r Receipt) error         { j.state = "success"; j.receipt = r; return nil }
func (j *stubJob) Backoff(err error) error         { j.state = "backoff"; j.bErr = err; return nil }
func (j *stubJob) Error(err error) error           { j.state = "error"; j.bErr = err; return nil }

func TestProcessBatchSuccess(t *testing.T) {
	s := &stubSender{receipt: Receipt{ProviderID: "SM1", Status: "queued"}}
	j := &stubJob{msg: Message{To: "+15555551212", Body: "hi"}, sender: s}
	processBatch(context.Background(), []Job{j})
	if j.state != "success" {
		t.Errorf("state = %q, want success", j.state)
	}
	if j.receipt.ProviderID != "SM1" {
		t.Errorf("receipt = %+v", j.receipt)
	}
	if s.calls.Load() != 1 {
		t.Errorf("sender called %d times, want 1", s.calls.Load())
	}
}

func TestProcessBatchPermanentError(t *testing.T) {
	s := &stubSender{err: ErrInvalidRecipient}
	j := &stubJob{msg: Message{To: "bad", Body: "hi"}, sender: s}
	processBatch(context.Background(), []Job{j})
	if j.state != "error" {
		t.Errorf("state = %q, want error", j.state)
	}
}

func TestProcessBatchRetryableError(t *testing.T) {
	s := &stubSender{err: errors.New("network blip")}
	j := &stubJob{msg: Message{To: "+15555551212", Body: "hi"}, sender: s}
	processBatch(context.Background(), []Job{j})
	if j.state != "backoff" {
		t.Errorf("state = %q, want backoff", j.state)
	}
}

func TestProcessBatchSenderBuildFailure(t *testing.T) {
	// First job's GetSender errors → entire batch goes through Error.
	bad := &stubJob{gsErr: errors.New("bad config")}
	good := &stubJob{msg: Message{To: "+15555551212", Body: "hi"}}
	processBatch(context.Background(), []Job{bad, good})
	if bad.state != "error" || good.state != "error" {
		t.Errorf("expected both error, got bad=%q good=%q", bad.state, good.state)
	}
}

func TestProcessBatchRenderFailure(t *testing.T) {
	s := &stubSender{}
	j := &stubJob{rErr: errors.New("template explosion"), sender: s}
	processBatch(context.Background(), []Job{j})
	if j.state != "error" {
		t.Errorf("state = %q, want error", j.state)
	}
	if s.calls.Load() != 0 {
		t.Errorf("sender should not be called when render fails, got %d", s.calls.Load())
	}
}

func TestSMSWorkerStartProcessesQueue(t *testing.T) {
	w := NewSMSWorker()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go w.Start(ctx)

	s := &stubSender{receipt: Receipt{ProviderID: "SM"}}
	j := &stubJob{msg: Message{To: "+15555551212", Body: "hi"}, sender: s}
	w.Queue([]Job{j})
	// processBatch is fired in its own goroutine; spin briefly.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if j.state != "" {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if j.state != "success" {
		t.Errorf("state = %q, want success (sender hit %d times)", j.state, s.calls.Load())
	}
}
