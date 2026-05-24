package core

import (
	"q+/internal/generated/ent"
	"q+/internal/generated/ent/examiner"
)

type ExaminerChooseCriterionParams struct {
	ServerCommandParams
	ExaminerId  int64
	CriteriaIds []int64
}

var ExaminerChooseCriterion = wrapTx(examinerChooseCriterion)

func examinerChooseCriterion(ctx UseCaseContext[ExaminerChooseCriterionParams]) (*ent.Examiner, error) {
	err := ctx.ent().Examiner.UpdateOneID(ctx.Params.ExaminerId).
		ClearCriteria().
		AddCriteriumIDs(ctx.Params.CriteriaIds...).
		Exec(ctx.Ctx)
	if err != nil {
		return nil, err
	}

	exam, err := ctx.ent().Examiner.
		Query().
		Where(examiner.ID(ctx.Params.ExaminerId)).
		WithCriteria().
		WithQueue().
		WithTeacher().
		Only(ctx.Ctx)
	if err != nil {
		return nil, err
	}

	GoRedrawQueue(ctx, exam.Edges.Queue.ID)

	return exam, nil
}
