package core

import (
	"github.com/samber/lo"
	"q+/internal/core/discord"
	"q+/internal/generated/ent"
	"q+/internal/generated/ent/examiner"
	"q+/internal/generated/ent/mark"
	"q+/internal/generated/ent/queueplacecriterion"
)

type SetMarkParams struct {
	ServerCommandParams
	CriterionId int64
	Mark        string
	Teacher     *discord.User
}

type SetMarkResponse struct {
	Team      []*ent.User
	Mark      string
	Criterion *ent.Criterion
}

var SetMark = wrapTx(setMark)

func setMark(ctx UseCaseContext[SetMarkParams]) (*SetMarkResponse, error) {
	q, err := ctx.getActiveQueue(ctx.Params.DiscordChannelId)
	if err != nil {
		return nil, err
	}
	if q == nil {
		return nil, ErrActiveQueueNotFound
	}

	examUser, err := ctx.getUser(ctx.Params.Teacher)
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
			q.WithQueuePlaceCriteria()
		}).
		Only(ctx.Ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, ErrExaminerNotFound
		}
		return nil, err
	}

	if exam.Edges.CurrentQueuePlace == nil {
		return nil, ErrCurrentQueuePlaceNotFound
	}

	criterion, err := ctx.ent().Criterion.Get(ctx.Ctx, ctx.Params.CriterionId)
	if err != nil {
		return nil, err
	}

	team := exam.Edges.CurrentQueuePlace.Edges.Team

	err = ctx.ent().Mark.MapCreateBulk(team, func(c *ent.MarkCreate, i int) {
		c.SetStudent(team[i]).
			SetCriterion(criterion).
			SetValue(ctx.Params.Mark).
			SetUpdatedBy(examUser)
	}).
		OnConflictColumns(mark.FieldCriterionID, mark.FieldUserID).
		UpdateValue().
		UpdateUpdatedByExamID().
		Exec(ctx.Ctx)
	if err != nil {
		return nil, err
	}

	err = ctx.ent().QueuePlaceCriterion.
		Update().
		Where(
			queueplacecriterion.QueuePlaceID(exam.Edges.CurrentQueuePlace.ID),
			queueplacecriterion.CriterionID(ctx.Params.CriterionId),
		).
		SetPassed(true).
		Exec(ctx.Ctx)
	if err != nil {
		return nil, err
	}

	queuePlaceCriteria, err := exam.Edges.CurrentQueuePlace.QueryQueuePlaceCriteria().All(ctx.Ctx)
	if err != nil {
		return nil, err
	}

	notPassedCriteriaIds := lo.FilterMap(queuePlaceCriteria, func(c *ent.QueuePlaceCriterion, _ int) (int64, bool) {
		return c.CriterionID, !c.Passed
	})

	sameCriteria := intersectIds(exam.Edges.Criteria, notPassedCriteriaIds)

	if len(sameCriteria) == 0 {
		err = unlockUsers(UseCaseContext[ServerCommandParams]{
			Ctx:    ctx.Ctx,
			Core:   ctx.Core,
			Params: ctx.Params.ServerCommandParams,
		}, q, exam)
		if err != nil {
			return nil, err
		}
	}

	defer GoRedrawQueue(ctx, q.ID)
	defer GoRedrawMarkTable(ctx, q.ID)

	return &SetMarkResponse{
		Team:      team,
		Mark:      ctx.Params.Mark,
		Criterion: criterion,
	}, nil
}
