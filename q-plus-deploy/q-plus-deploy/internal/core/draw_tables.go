package core

import (
	"context"
	"github.com/rs/zerolog"
	"github.com/samber/lo"
	"q+/internal/generated/ent"
	"q+/internal/generated/ent/courseinstance"
	"q+/internal/generated/ent/criterion"
	"q+/internal/generated/ent/examiner"
	"q+/internal/generated/ent/queue"
	"q+/internal/generated/ent/queueplace"
	"time"
)

func GoRedrawQueue[T any](ctx UseCaseContext[T], queueId int64) {
	go func(ctx context.Context, c *Core, logger *zerolog.Logger, queueId int64) {
		time.Sleep(500 * time.Millisecond) // wait for previous db transaction is over
		q, err := c.originalClient.Queue.
			Query().
			Where(queue.ID(queueId)).
			WithExaminers(func(q *ent.ExaminerQuery) {
				q.Order(examiner.ByID())
				q.WithCriteria()
				q.WithTeacher()
				q.WithCurrentQueuePlace()
			}).
			WithPlaces(func(q *ent.QueuePlaceQuery) {
				q.Order(queueplace.ByPosition())
				q.WithTeam()
				q.WithQueuePlaceCriteria(func(q *ent.QueuePlaceCriterionQuery) {
					q.WithCriterion()
				})
				q.WithCurrentExaminer()
			}).
			Only(ctx)
		if err != nil {
			logger.Error().
				Str("event", "redraw_queue").
				Int64("queue_id", queueId).
				Err(err).
				Msg("failed to get queue")
			return
		}
		spreadsheetId, err := q.
			QueryQueueTemplate().
			QueryCourseInstance().
			Select(courseinstance.FieldQueuesSpreadsheetID).
			String(ctx)
		if err != nil {
			logger.Error().
				Str("event", "redraw_queue").
				Int64("queue_id", queueId).
				Err(err).
				Msg("failed to get spreadsheet ID")
			return
		}
		err = c.sheetsService.RedrawQueue(ctx, spreadsheetId, lo.FromPtr(q.SheetID), q)
		if err != nil {
			logger.Error().
				Str("event", "redraw_queue").
				Int64("queue_id", queueId).
				Err(err).
				Msg("failed to redraw queue")
			return
		}
	}(ctx.Ctx, ctx.Core, ctx.log(), queueId)
}

func GoRedrawMarkTable[T any](ctx UseCaseContext[T], queueId int64) {
	go func(ctx context.Context, c *Core, logger *zerolog.Logger, queueId int64) {
		time.Sleep(500 * time.Millisecond) // wait for previous db transaction is over

		// TODO find and redraw all changed tabs by changed criterion

		tab, err := c.originalClient.Queue.
			Query().
			Where(queue.ID(queueId)).
			QueryMarkTableTab().
			WithMarkTable().
			Only(ctx)

		criteria, err := tab.QueryQueues().
			QueryCriteria().
			Order(criterion.ByID()).
			All(ctx)
		if err != nil {
			logger.Error().
				Str("event", "redraw_mark_table").
				Int64("queue_id", queueId).
				Err(err).
				Msg("failed to get criteria")
			return
		}

		students, err := tab.QueryQueues().
			QueryPlaces().
			QueryTeam().
			All(ctx)
		if err != nil {
			logger.Error().
				Str("event", "redraw_mark_table").
				Int64("queue_id", queueId).
				Err(err).
				Msg("failed to get students")
			return
		}

		marks, err := tab.QueryQueues().
			QueryPlaces().
			QueryCriteria().
			QueryMarks().
			All(ctx)
		if err != nil {
			logger.Error().
				Str("event", "redraw_mark_table").
				Int64("queue_id", queueId).
				Err(err).
				Msg("failed to get marks")
			return
		}

		err = c.sheetsService.RedrawMarkTableTab(ctx, tab.Edges.MarkTable.SpreadsheetID, tab.SheetID, tab, criteria, students, marks)
		if err != nil {
			logger.Error().
				Str("event", "redraw_mark_table").
				Int64("queue_id", queueId).
				Err(err).
				Msg("failed to redraw mark table")
			return
		}
	}(ctx.Ctx, ctx.Core, ctx.log(), queueId)
}
