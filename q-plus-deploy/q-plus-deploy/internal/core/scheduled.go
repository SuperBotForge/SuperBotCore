package core

import (
	"context"
	"github.com/rs/zerolog"
	"q+/internal/generated/ent"
	"strconv"
	"time"
)

type StartAllSignUpParams struct {
}

var StartAllSignUp = wrapTx(startAllSignUp)

func startAllSignUp(ctx UseCaseContext[StartAllSignUpParams]) (int, error) { // return number of executed queue sign-ups
	queues, err := ctx.ent().Queue.Query().All(ctx.Ctx)
	if err != nil {
		return 0, err
	}

	var count = 0
	for _, queue := range queues {
		if queue.SignUpStarted {
			continue
		}
		if queue.StartTime == nil {
			continue
		}
		signUpTime := queue.StartTime.Add(-queue.SignUpLeadTime)
		if signUpTime.Before(time.Now()) {
			count++
			err = _startSignUp(ctx.Ctx, ctx.Core, queue)
			if err != nil {
				return 0, err
			}
		}
	}
	return count, nil
}

type StartAllQueueParams struct {
}

var StartAllQueue = wrapTx(startAllQueue)

func startAllQueue(ctx UseCaseContext[StartAllQueueParams]) (int, error) { // return number of executed queues
	queues, err := ctx.ent().Queue.Query().All(ctx.Ctx)
	if err != nil {
		return 0, err
	}

	var count = 0
	for _, q := range queues {
		if q.QueueStarted { // TODO duplication with startAllSignUp
			continue
		}
		if q.StartTime == nil {
			continue
		}
		if q.StartTime.Before(time.Now()) {
			count++
			err = _startQueue(ctx.Ctx, ctx.Core, q)
			if err != nil {
				return 0, err
			}
		}
	}
	return count, nil
}

func _startSignUp(ctx context.Context, core *Core, q *ent.Queue) error {
	logger := zerolog.Ctx(ctx)

	course, err := q.
		QueryQueueTemplate().
		QueryCourseInstance().
		Only(ctx)
	if err != nil {
		return err
	}
	channels, err := course.
		QueryChannelsForCourse().
		Only(ctx)
	if err != nil {
		return err
	}

	if q.SheetID == nil {
		sheetId, err := core.sheetsService.CreateSheet(ctx, course.QueuesSpreadsheetID, q.Name)
		if err != nil {
			return err
		}
		q, err = q.Update().
			SetSheetID(sheetId).
			Save(ctx)
		if err != nil {
			return err
		}
	}

	err = core.discordSender.SendStudentGuide(ctx, q, course, channels)
	if err != nil {
		return err
	}
	err = q.Update().
		SetSignUpStarted(true).
		Exec(ctx)
	if err != nil {
		return err
	}
	logger.Info().
		Str("event", "sign_up_started").
		Str("queue_id", strconv.FormatInt(q.ID, 10)).
		Msg("Sign-up started")
	return nil
}

func _startQueue(ctx context.Context, core *Core, q *ent.Queue) error {
	logger := zerolog.Ctx(ctx)

	course, err := q.
		QueryQueueTemplate().
		QueryCourseInstance().
		Only(ctx)
	if err != nil {
		return err
	}
	channels, err := course.
		QueryChannelsForCourse().
		Only(ctx)
	if err != nil {
		return err
	}

	if q.SheetID == nil {
		sheetId, err := core.sheetsService.CreateSheet(ctx, course.QueuesSpreadsheetID, q.Name)
		if err != nil {
			return err
		}
		q, err = q.Update().
			SetSheetID(sheetId).
			Save(ctx)
		if err != nil {
			return err
		}
	}

	err = core.discordSender.SendMessage(ctx, channels.QueueChannelID, "## Началась очередь '"+q.Name+"'")
	if err != nil {
		return err
	}

	markTableTab, err := q.QueryMarkTableTab().
		WithMarkTable().
		Only(ctx)
	if err != nil {
		return err
	}

	if !q.SignUpStarted {
		err = core.discordSender.SendStudentGuide(ctx, q, course, channels)
		if err != nil {
			return err
		}
	}
	err = core.discordSender.SendTeacherGuide(ctx, q, course, markTableTab, channels)
	if err != nil {
		return err
	}

	err = q.Update().
		SetSignUpStarted(true).
		SetQueueStarted(true).
		SetQueueEnded(false).
		Exec(ctx)
	if err != nil {
		return err
	}
	logger.Info().
		Str("event", "queue_started").
		Str("queue_id", strconv.FormatInt(q.ID, 10)).
		Msg("Queue started")
	return nil
}
