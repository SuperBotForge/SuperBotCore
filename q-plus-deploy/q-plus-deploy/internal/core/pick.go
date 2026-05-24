package core

import (
	"github.com/samber/lo"
	"q+/internal/core/discord"
	"q+/internal/generated/ent"
	"q+/internal/generated/ent/examiner"
	"q+/internal/generated/ent/queueplace"
)

type PickParams struct {
	ServerCommandParams
	User         *discord.User
	QueuePlaceId int64
}

type PickResponse struct {
	NextQueuePlace *ent.QueuePlace
	Criteria       []*ent.Criterion
	Examiner       *ent.Examiner
}

var Pick = wrapTx(pick)

func pick(ctx UseCaseContext[PickParams]) (*PickResponse, error) {
	q, err := ctx.getActiveQueue(ctx.Params.DiscordChannelId)
	if err != nil {
		return nil, err
	}
	if q == nil {
		return nil, ErrActiveQueueNotFound
	}

	examUser, err := ctx.getUser(ctx.Params.User)
	if err != nil {
		return nil, err
	}

	exam, err := ctx.ent().Examiner.
		Query().
		Where(
			examiner.TeacherID(examUser.ID),
			examiner.QueueID(q.ID),
		).
		WithCriteria().
		WithCurrentQueuePlace(func(q *ent.QueuePlaceQuery) {
			q.WithTeam()
		}).
		WithTeacher().
		Only(ctx.Ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, ErrExaminerNotFound
		}
		return nil, err
	}

	defer GoRedrawQueue(ctx, q.ID)

	nextPlace, err := ctx.ent().QueuePlace.
		Query().
		Where(queueplace.ID(ctx.Params.QueuePlaceId)).
		WithTeam().
		WithQueuePlaceCriteria().
		Only(ctx.Ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, ErrQueuePlaceNotFound
		}
		return nil, err
	} // пасхалОчка

	if exam.Edges.CurrentQueuePlace != nil {
		err = unlockUsers(UseCaseContext[ServerCommandParams]{
			Ctx:    ctx.Ctx,
			Core:   ctx.Core,
			Params: ctx.Params.ServerCommandParams,
		}, q, exam)
		if err != nil {
			return nil, err
		}
	}

	if nextPlace == nil {
		return &PickResponse{
			NextQueuePlace: nil,
			Criteria:       nil,
			Examiner:       exam,
		}, nil
	}

	err = exam.Update().
		SetCurrentQueuePlace(nextPlace).
		Exec(ctx.Ctx)
	if err != nil {
		return nil, err
	}

	err = lockUsers(ctx, nextPlace)
	if err != nil {
		return nil, err
	}

	notPassedCriteria := lo.FilterMap(nextPlace.Edges.QueuePlaceCriteria, func(c *ent.QueuePlaceCriterion, _ int) (int64, bool) {
		return c.CriterionID, !c.Passed
	})

	return &PickResponse{
		NextQueuePlace: nextPlace,
		Criteria:       intersectIds(exam.Edges.Criteria, notPassedCriteria),
		Examiner:       exam,
	}, nil
}
