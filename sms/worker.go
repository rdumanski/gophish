package sms

import (
	"context"
	"errors"

	log "github.com/rdumanski/gophish/logger"
)

// Worker is the dispatcher half of the package — analogous to
// mailer.Mailer. The default implementation (SMSWorker) consumes
// batches from a channel and processes each Job through that
// campaign's Sender.
type Worker interface {
	Start(ctx context.Context)
	Queue([]Job)
}

// Job is implemented by anything the dispatcher knows how to send. In
// production this is *models.SMSLog, but the interface keeps the
// dispatcher free of a models dependency for testability (sms_test.go
// uses an in-package fake).
//
// Render builds the per-recipient Message (To/From/Body resolved
// against the template context). GetSender returns the configured
// provider client for this Job's campaign — the dispatcher reuses the
// returned Sender across every Job in the same batch (same campaign
// = same SMSProfile).
//
// Backoff/Error/Success follow the same lifecycle as mailer.Mail:
// retryable failures call Backoff, permanent failures call Error,
// successful sends call Success with the provider receipt.
type Job interface {
	Render() (Message, error)
	GetSender() (Sender, error)
	Backoff(reason error) error
	Error(err error) error
	Success(receipt Receipt) error
}

// SMSWorker is a channel-based dispatcher. Mirror of mailer.MailWorker.
type SMSWorker struct {
	queue chan []Job
}

// NewSMSWorker returns a ready-to-use SMSWorker.
func NewSMSWorker() *SMSWorker {
	return &SMSWorker{queue: make(chan []Job)}
}

// Start consumes batches from the queue. Each batch is processed in
// its own goroutine so a slow provider on one campaign doesn't block
// the dispatcher loop. The shared ctx lets a graceful shutdown
// short-circuit any in-flight sends.
func (w *SMSWorker) Start(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case js := <-w.queue:
			go processBatch(ctx, js)
		}
	}
}

// Queue submits a batch for dispatch. Blocks until the worker accepts
// it; same shape as mailer.Mailer.Queue.
func (w *SMSWorker) Queue(js []Job) {
	w.queue <- js
}

// processBatch fetches the campaign-shared Sender once and then drives
// each Job individually. Provider construction errors flunk the whole
// batch (every Job calls Error) — a misconfigured profile is permanent
// for every recipient in the batch.
func processBatch(ctx context.Context, js []Job) {
	if len(js) == 0 {
		return
	}
	sender, err := js[0].GetSender()
	if err != nil {
		log.Errorf("sms: building sender failed for batch of %d: %s", len(js), err)
		for _, j := range js {
			if eerr := j.Error(err); eerr != nil {
				log.Errorf("sms: error finalising job after sender-build failure: %s", eerr)
			}
		}
		return
	}
	for _, j := range js {
		processOne(ctx, sender, j)
	}
}

// processOne renders, sends, and routes the outcome through the right
// lifecycle method on the Job. Sentinel errors from the sender package
// are treated as permanent (Error); everything else is treated as
// retryable (Backoff).
func processOne(ctx context.Context, sender Sender, j Job) {
	msg, err := j.Render()
	if err != nil {
		// Render failure is permanent — the template can't be made
		// to produce a valid message for this recipient.
		log.Errorf("sms: render failed: %s", err)
		if eerr := j.Error(err); eerr != nil {
			log.Errorf("sms: error finalising render-failed job: %s", eerr)
		}
		return
	}
	receipt, err := sender.Send(ctx, msg)
	if err == nil {
		if serr := j.Success(receipt); serr != nil {
			log.Errorf("sms: success finalisation failed: %s", serr)
		}
		return
	}
	if isPermanent(err) {
		if eerr := j.Error(err); eerr != nil {
			log.Errorf("sms: error finalisation failed: %s", eerr)
		}
		return
	}
	if berr := j.Backoff(err); berr != nil {
		log.Errorf("sms: backoff failed: %s", berr)
	}
}

// isPermanent reports whether the sender returned one of the package
// sentinel errors. Unknown errors are treated as retryable so transient
// network blips don't burn a recipient.
func isPermanent(err error) bool {
	for _, sentinel := range []error{
		ErrInvalidRecipient,
		ErrUnregisteredFromNumber,
		ErrCarrierBlocked,
		ErrRefused,
	} {
		if errors.Is(err, sentinel) {
			return true
		}
	}
	return false
}
