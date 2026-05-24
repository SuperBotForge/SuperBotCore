package cron

import (
	"github.com/rs/zerolog"
	"q+/internal/core"
)

func QueueCronEventsHandler(ctx Context) (int, error) {
	logger := zerolog.Ctx(ctx.ctx)

	logger.Info().
		Str("event", "queue_cron_events").
		Msg("Queueing cron events...")

	signUpResult, err := core.StartAllSignUp(useCaseContext(ctx, core.StartAllSignUpParams{}))
	if err != nil {
		return 0, err
	}

	queueResult, err := core.StartAllQueue(useCaseContext(ctx, core.StartAllQueueParams{}))
	if err != nil {
		return 0, err
	}

	return signUpResult + queueResult, nil
}
