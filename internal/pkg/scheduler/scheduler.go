package scheduler

import (
	"github.com/robfig/cron/v3"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/wire"
)

// ProviderSet is scheduler providers.
var ProviderSet = wire.NewSet(NewScheduler)

// Scheduler wraps a cron scheduler for managing scheduled jobs.
type Scheduler struct {
	cron *cron.Cron
	logger *log.Helper
}

// NewScheduler creates a new scheduler.
func NewScheduler(logger log.Logger) *Scheduler {
	return &Scheduler{
		cron:   cron.New(cron.WithSeconds()),
		logger: log.NewHelper(logger),
	}
}

// AddJob adds a cron job.
// spec is a cron expression (e.g., "@every 1h", "0 0 * * *").
func (s *Scheduler) AddJob(spec string, cmd func()) error {
	id, err := s.cron.AddFunc(spec, cmd)
	if err != nil {
		return err
	}
	s.logger.Infof("scheduled job added: %s (id: %d)", spec, id)
	return nil
}

// Start starts the scheduler.
func (s *Scheduler) Start() {
	s.cron.Start()
	s.logger.Info("scheduler started")
}

// Stop stops the scheduler and waits for running jobs to complete.
func (s *Scheduler) Stop() {
	ctx := s.cron.Stop()
	<-ctx.Done()
	s.logger.Info("scheduler stopped")
}

// Entries returns all scheduled entries.
func (s *Scheduler) Entries() []cron.Entry {
	return s.cron.Entries()
}
