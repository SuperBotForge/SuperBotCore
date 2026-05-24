package cron

import (
	"context"
	"github.com/reugn/go-quartz/job"
	"github.com/reugn/go-quartz/quartz"
	"github.com/rs/zerolog"
	"github.com/samber/do/v2"
	"q+/internal/core"
)

type Scheduler struct {
	quartz.Scheduler
	core *core.Core
}

func NewScheduler(i do.Injector) (*Scheduler, error) {
	c := do.MustInvoke[*core.Core](i)
	return &Scheduler{
		Scheduler: quartz.NewStdScheduler(),
		core:      c,
	}, nil
}

func (s *Scheduler) Start(ctx context.Context) error {
	logger := zerolog.Ctx(ctx)
	s.Scheduler.Start(ctx)

	// defer s.Stop() // TODO stop in do Shutdown

	cronTrigger, err := quartz.NewCronTrigger("0 * * * * *") // every minute
	if err != nil {
		return err
	}

	queueEventsJob := job.NewFunctionJob(newFunctionJob(s.core, QueueCronEventsHandler))

	err = s.ScheduleJob(quartz.NewJobDetail(queueEventsJob, quartz.NewJobKey("queue_events")), cronTrigger)
	if err != nil {
		return err
	}

	logger.Info().
		Str("event", "cron_scheduler_started").
		Msg("Cron scheduler started")

	err = queueEventsJob.Execute(ctx)
	if err != nil {
		logger.Error().
			Str("event", "cron_job_first_execution_error").
			Err(err).
			Msg("Error executing first cron job")
	}

	<-ctx.Done()
	s.Wait(ctx)

	return nil
}
