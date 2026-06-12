package worker

import (
	"context"
	"time"

	log "github.com/rdumanski/gophish/logger"
	"github.com/rdumanski/gophish/mailer"
	"github.com/rdumanski/gophish/models"
	"github.com/rdumanski/gophish/sms"
	"github.com/sirupsen/logrus"
)

// Worker is an interface that defines the operations needed for a background worker
type Worker interface {
	Start()
	LaunchCampaign(c models.Campaign)
	SendTestEmail(s *models.EmailRequest) error
}

// DefaultWorker is the background worker that handles watching for new campaigns and sending emails appropriately.
type DefaultWorker struct {
	mailer    mailer.Mailer
	smsWorker sms.Worker
}

// New creates a new worker object to handle the creation of campaigns
func New(options ...func(Worker) error) (Worker, error) {
	w := &DefaultWorker{
		mailer:    mailer.NewMailWorker(),
		smsWorker: sms.NewSMSWorker(),
	}
	for _, opt := range options {
		if err := opt(w); err != nil {
			return nil, err
		}
	}
	return w, nil
}

// WithMailer sets the mailer for a given worker.
// By default, workers use a standard, default mailworker.
func WithMailer(m mailer.Mailer) func(*DefaultWorker) error {
	return func(w *DefaultWorker) error {
		w.mailer = m
		return nil
	}
}

// WithSMSWorker overrides the default SMS dispatcher (mostly for tests
// — production wiring uses sms.NewSMSWorker via New).
func WithSMSWorker(s sms.Worker) func(*DefaultWorker) error {
	return func(w *DefaultWorker) error {
		w.smsWorker = s
		return nil
	}
}

// processCampaigns loads queued mail and SMS logs scheduled to be sent
// before the provided time and dispatches them to the right worker.
func (w *DefaultWorker) processCampaigns(t time.Time) error {
	if err := w.processMailCampaigns(t); err != nil {
		return err
	}
	return w.processSMSCampaigns(t)
}

// processMailCampaigns is the original email path, factored out of
// processCampaigns so the SMS path can sit alongside without nesting.
func (w *DefaultWorker) processMailCampaigns(t time.Time) error {
	ms, err := models.GetQueuedMailLogs(t.UTC())
	if err != nil {
		log.Error(err)
		return err
	}
	// Lock the MailLogs (they will be unlocked after processing)
	err = models.LockMailLogs(ms, true)
	if err != nil {
		return err
	}
	campaignCache := make(map[int64]models.Campaign)
	// We'll group the maillogs by campaign ID to (roughly) group
	// them by sending profile. This lets the mailer re-use the Sender
	// instead of having to re-connect to the SMTP server for every
	// email.
	msg := make(map[int64][]mailer.Mail)
	for _, m := range ms {
		// We cache the campaign here to greatly reduce the time it takes to
		// generate the message (ref #1726)
		c, ok := campaignCache[m.CampaignID]
		if !ok {
			c, err = models.GetCampaignMailContext(m.CampaignID, m.UserID)
			if err != nil {
				return err
			}
			campaignCache[c.Id] = c
		}
		m.CacheCampaign(&c)
		msg[m.CampaignID] = append(msg[m.CampaignID], m)
	}

	// Next, we process each group of maillogs in parallel
	for cid, msc := range msg {
		go func(cid int64, msc []mailer.Mail) {
			c := campaignCache[cid]
			if c.Status == models.CampaignQueued {
				err := c.UpdateStatus(models.CampaignInProgress)
				if err != nil {
					log.Error(err)
					return
				}
			}
			log.WithFields(logrus.Fields{
				"num_emails": len(msc),
			}).Info("Sending emails to mailer for processing")
			w.mailer.Queue(msc)
		}(cid, msc)
	}
	return nil
}

// processSMSCampaigns is the SMS-channel parallel of
// processMailCampaigns. Same shape — load due rows, lock, group by
// campaign, dispatch — just hitting the SMS queue and worker.
func (w *DefaultWorker) processSMSCampaigns(t time.Time) error {
	ss, err := models.GetQueuedSMSLogs(t.UTC())
	if err != nil {
		log.Error(err)
		return err
	}
	if len(ss) == 0 {
		return nil
	}
	if err := models.LockSMSLogs(ss, true); err != nil {
		return err
	}
	campaignCache := make(map[int64]models.Campaign)
	jobs := make(map[int64][]sms.Job)
	for _, s := range ss {
		c, ok := campaignCache[s.CampaignID]
		if !ok {
			c, err = models.GetCampaignSMSContext(s.CampaignID, s.UserID)
			if err != nil {
				return err
			}
			campaignCache[c.Id] = c
		}
		if cerr := s.CacheCampaign(&c); cerr != nil {
			log.Error(cerr)
			continue
		}
		jobs[s.CampaignID] = append(jobs[s.CampaignID], s)
	}
	for cid, batch := range jobs {
		go func(cid int64, batch []sms.Job) {
			c := campaignCache[cid]
			if c.Status == models.CampaignQueued {
				if uerr := c.UpdateStatus(models.CampaignInProgress); uerr != nil {
					log.Error(uerr)
					return
				}
			}
			log.WithFields(logrus.Fields{
				"num_messages": len(batch),
				"campaign_id":  cid,
			}).Info("Sending SMS batch to dispatcher for processing")
			w.smsWorker.Queue(batch)
		}(cid, batch)
	}
	return nil
}

// Start launches the worker to poll the database every minute for any pending maillogs
// that need to be processed.
func (w *DefaultWorker) Start() {
	log.Info("Background Worker Started Successfully - Waiting for Campaigns")
	ctx := context.Background()
	go w.mailer.Start(ctx)
	go w.smsWorker.Start(ctx)
	for t := range time.Tick(1 * time.Minute) {
		err := w.processCampaigns(t)
		if err != nil {
			log.Error(err)
			continue
		}
	}
}

// LaunchCampaign starts a campaign — picks the right channel-specific
// path based on c.Channel.
func (w *DefaultWorker) LaunchCampaign(c models.Campaign) {
	if c.Channel == "sms" {
		w.launchSMSCampaign(c)
		return
	}
	w.launchMailCampaign(c)
}

func (w *DefaultWorker) launchMailCampaign(c models.Campaign) {
	ms, err := models.GetMailLogsByCampaign(c.Id)
	if err != nil {
		log.Error(err)
		return
	}
	models.LockMailLogs(ms, true)
	// This is required since you cannot pass a slice of values
	// that implements an interface as a slice of that interface.
	mailEntries := []mailer.Mail{}
	currentTime := time.Now().UTC()
	campaignMailCtx, err := models.GetCampaignMailContext(c.Id, c.UserID)
	if err != nil {
		log.Error(err)
		return
	}
	for _, m := range ms {
		// Only send the emails scheduled to be sent for the past minute to
		// respect the campaign scheduling options
		if m.SendDate.After(currentTime) {
			m.Unlock()
			continue
		}
		err = m.CacheCampaign(&campaignMailCtx)
		if err != nil {
			log.Error(err)
			return
		}
		mailEntries = append(mailEntries, m)
	}
	w.mailer.Queue(mailEntries)
}

func (w *DefaultWorker) launchSMSCampaign(c models.Campaign) {
	ss, err := models.GetSMSLogsByCampaign(c.Id)
	if err != nil {
		log.Error(err)
		return
	}
	if err := models.LockSMSLogs(ss, true); err != nil {
		log.Error(err)
		return
	}
	currentTime := time.Now().UTC()
	smsCtx, err := models.GetCampaignSMSContext(c.Id, c.UserID)
	if err != nil {
		log.Error(err)
		return
	}
	jobs := []sms.Job{}
	for _, s := range ss {
		if s.SendDate.After(currentTime) {
			if uerr := s.Unlock(); uerr != nil {
				log.Error(uerr)
			}
			continue
		}
		if cerr := s.CacheCampaign(&smsCtx); cerr != nil {
			log.Error(cerr)
			return
		}
		jobs = append(jobs, s)
	}
	w.smsWorker.Queue(jobs)
}

// SendTestEmail sends a test email
func (w *DefaultWorker) SendTestEmail(s *models.EmailRequest) error {
	go func() {
		ms := []mailer.Mail{s}
		w.mailer.Queue(ms)
	}()
	return <-s.ErrorChan
}
