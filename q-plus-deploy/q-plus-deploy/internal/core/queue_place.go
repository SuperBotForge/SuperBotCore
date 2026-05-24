package core

import (
	"github.com/samber/lo"
	"q+/internal/generated/ent"
	"q+/internal/generated/ent/criterion"
	"q+/internal/generated/ent/examiner"
	"q+/internal/generated/ent/queueplace"
	"q+/internal/generated/ent/queueplacecriterion"
)

type QueuePlaceChooseCriterionParams struct {
	ServerCommandParams
	QueuePlaceId int64
	CriteriaIds  []int64
}

var QueuePlaceChooseCriterion = wrapTx(queuePlaceChooseCriterion)

func queuePlaceChooseCriterion(ctx UseCaseContext[QueuePlaceChooseCriterionParams]) (*ent.QueuePlace, error) {
	q, err := ctx.ent().QueuePlace.
		Query().
		Where(queueplace.ID(ctx.Params.QueuePlaceId)).
		QueryQueue().
		Only(ctx.Ctx)
	if err != nil {
		return nil, err
	}

	channels, err := ctx.getChannelsForCourse(ctx.Params.DiscordChannelId)
	if err != nil {
		return nil, err
	}

	var idleExams []*ent.Examiner
	if q.QueueStarted && !q.QueueEnded {
		idleExams, err = getIdleExams(ctx, q, ctx.Params.CriteriaIds, []int64{})
		if err != nil {
			return nil, err
		}
	}

	_, err = ctx.ent().QueuePlaceCriterion.
		Delete().
		Where(
			queueplacecriterion.Passed(false),
			queueplacecriterion.QueuePlaceID(ctx.Params.QueuePlaceId),
		).
		Exec(ctx.Ctx)
	if err != nil {
		return nil, err
	}
	_, err = ctx.ent().QueuePlaceCriterion.MapCreateBulk(ctx.Params.CriteriaIds, func(c *ent.QueuePlaceCriterionCreate, i int) {
		c.SetQueuePlaceID(ctx.Params.QueuePlaceId).
			SetCriterionID(ctx.Params.CriteriaIds[i]).
			SetPassed(false)
	}).Save(ctx.Ctx)
	if err != nil {
		return nil, err
	}

	queuePlace, err := ctx.ent().QueuePlace.
		Query().
		Where(queueplace.ID(ctx.Params.QueuePlaceId)).
		WithCriteria().
		WithQueue().
		WithTeam().
		Only(ctx.Ctx)
	if err != nil {
		return nil, err
	}

	queuePlaces := filterNotBusyPlaces([]*ent.QueuePlace{queuePlace})

	if q.QueueStarted && !q.QueueEnded && len(idleExams) > 0 && len(queuePlaces) > 0 {
		users := lo.Map(idleExams, func(e *ent.Examiner, _ int) *ent.User {
			return e.Edges.Teacher
		})
		err = ctx.Core.discordSender.SendNewStudentInQueueNotify(ctx.Ctx, users, queuePlace.Edges.Team, channels)
		if err != nil {
			return nil, err
		}
	}

	GoRedrawQueue(ctx, q.ID)

	return queuePlace, nil
}

func getIdleExams[T any](ctx UseCaseContext[T], q *ent.Queue, forCriteria []int64, ignoreExamIds []int64) ([]*ent.Examiner, error) {
	places, err := q.QueryPlaces().
		WithTeam().
		WithQueuePlaceCriteria().
		WithCurrentExaminer().
		All(ctx.Ctx)

	if err != nil {
		return nil, err
	}

	potentialExams, err := ctx.ent().Criterion.
		Query().
		Where(criterion.IDIn(forCriteria...)).
		QueryExaminers().
		Where(
			examiner.QueueID(q.ID),
			examiner.Not(examiner.HasCurrentQueuePlace()),
			examiner.IDNotIn(ignoreExamIds...),
		).
		WithCriteria().
		WithTeacher().
		All(ctx.Ctx)
	if err != nil {
		return nil, err
	}

	idleExams := lo.Filter(potentialExams, func(e *ent.Examiner, _ int) bool {
		placesForExaminer := lo.Filter(places, func(p *ent.QueuePlace, _ int) bool {
			notPassedCriteriaIds := lo.FilterMap(p.Edges.QueuePlaceCriteria, func(c *ent.QueuePlaceCriterion, _ int) (int64, bool) {
				return c.CriterionID, !c.Passed
			})

			sameCriteria := intersectIds(e.Edges.Criteria, notPassedCriteriaIds)
			return len(sameCriteria) > 0
		})
		notBusyPlaces := filterNotBusyPlaces(placesForExaminer)
		return len(notBusyPlaces) == 0
	})
	return idleExams, nil
}
