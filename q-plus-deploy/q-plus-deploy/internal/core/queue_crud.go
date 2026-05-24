package core

import (
	"q+/internal/generated/ent"
	"q+/internal/generated/ent/criterion"
	"q+/internal/generated/ent/queue"
	"q+/internal/generated/ent/queuetemplate"
	"time"
)

type CreateQueueParams struct {
	QueueTemplateId int64
	Name            *string
	StartTime       *time.Time
	EndTime         *time.Time
	SignUpLeadTime  *time.Duration
}

var CreateQueue = wrapTx(createQueue)

func createQueue(ctx UseCaseContext[CreateQueueParams]) (*ent.Queue, error) {
	template, err := ctx.ent().QueueTemplate.
		Query().
		Where(queuetemplate.ID(ctx.Params.QueueTemplateId)).
		WithMarkTableTab().
		WithCriteria().
		Only(ctx.Ctx)
	if err != nil {
		return nil, err
	}

	queueCreate := ctx.ent().Queue.
		Create().
		SetQueueTemplate(template)

	if ctx.Params.Name != nil {
		queueCreate.SetName(*ctx.Params.Name)
	} else {
		queueCreate.SetName(template.Name)
	}

	if ctx.Params.SignUpLeadTime != nil {
		queueCreate.SetSignUpLeadTime(*ctx.Params.SignUpLeadTime)
	} else {
		queueCreate.SetSignUpLeadTime(template.SignUpLeadTime)
	}

	if ctx.Params.StartTime != nil {
		queueCreate.SetStartTime(*ctx.Params.StartTime)
	}

	if ctx.Params.EndTime != nil {
		queueCreate.SetEndTime(*ctx.Params.EndTime)
	}

	q, err := queueCreate.
		SetMarkTableTab(template.Edges.MarkTableTab).
		AddCriteria(template.Edges.Criteria...).
		Save(ctx.Ctx)
	if err != nil {
		return nil, err
	}

	q.Edges.QueueTemplate = template

	return q, nil
}

type ListQueueTemplateQueuesParams struct {
	QueueTemplateId int64
}

var ListQueueTemplateQueues = wrapTx(listQueueTemplateQueues)

func listQueueTemplateQueues(ctx UseCaseContext[ListQueueTemplateQueuesParams]) ([]*ent.Queue, error) {
	return ctx.ent().QueueTemplate.
		Query().
		Where(queuetemplate.ID(ctx.Params.QueueTemplateId)).
		QueryQueues().
		WithCriteria().
		WithMarkTableTab().
		WithExaminers().
		WithPlaces().
		All(ctx.Ctx)
}

type EditQueueParams struct {
	QueueId        int64
	Name           *string
	StartTime      *time.Time
	EndTime        *time.Time
	SignUpLeadTime *time.Duration
}

var EditQueue = wrapTx(editQueue)

func editQueue(ctx UseCaseContext[EditQueueParams]) (*ent.Queue, error) {
	q, err := ctx.ent().Queue.Get(ctx.Ctx, ctx.Params.QueueId)
	if ent.IsNotFound(err) {
		return nil, ErrQueueNotFound
	}
	if err != nil {
		return nil, err
	}

	updateQuery := q.Update().
		SetNillableName(ctx.Params.Name).
		SetNillableSignUpLeadTime(ctx.Params.SignUpLeadTime)

	if ctx.Params.StartTime != nil {
		if ctx.Params.StartTime.IsZero() {
			updateQuery = updateQuery.ClearStartTime()
		} else {
			updateQuery = updateQuery.SetStartTime(*ctx.Params.StartTime)
		}
	}
	if ctx.Params.EndTime != nil {
		if ctx.Params.EndTime.IsZero() {
			updateQuery = updateQuery.ClearEndTime()
		} else {
			updateQuery = updateQuery.SetEndTime(*ctx.Params.EndTime)
		}
	}

	err = updateQuery.Exec(ctx.Ctx)
	if err != nil {
		return nil, err
	}

	return ctx.ent().Queue.
		Query().
		Where(queue.ID(q.ID)).
		Only(ctx.Ctx)
}

type SelectCriteriaForQueueParams struct {
	QueueId int64
}

type SelectCriteriaForQueueResponse struct {
	Queue            *ent.Queue
	Criteria         []*ent.Criterion
	SelectedCriteria []int64
}

var SelectCriteriaForQueue = wrapTx(selectCriteriaForQueue)

func selectCriteriaForQueue(ctx UseCaseContext[SelectCriteriaForQueueParams]) (*SelectCriteriaForQueueResponse, error) {
	q, err := ctx.ent().Queue.Get(ctx.Ctx, ctx.Params.QueueId)
	if ent.IsNotFound(err) {
		return nil, ErrQueueNotFound
	}
	if err != nil {
		return nil, err
	}

	criteria, err := q.QueryQueueTemplate().QueryCourseInstance().QueryCriteria().All(ctx.Ctx)
	if err != nil {
		return nil, err
	}

	var selectedCriteria []int64
	err = q.QueryCriteria().Select(criterion.FieldID).Scan(ctx.Ctx, &selectedCriteria)
	if err != nil {
		return nil, err
	}

	return &SelectCriteriaForQueueResponse{
		Queue:            q,
		Criteria:         criteria,
		SelectedCriteria: selectedCriteria,
	}, nil
}

type QueueChooseCriterionParams struct {
	ServerCommandParams
	QueueId     int64
	CriteriaIds []int64
}

var QueueChooseCriterion = wrapTx(queueChooseCriterion)

func queueChooseCriterion(ctx UseCaseContext[QueueChooseCriterionParams]) (*ent.Queue, error) {
	err := ctx.ent().Queue.UpdateOneID(ctx.Params.QueueId).
		ClearCriteria().
		AddCriteriumIDs(ctx.Params.CriteriaIds...).
		Exec(ctx.Ctx)
	if err != nil {
		return nil, err
	}

	return ctx.ent().Queue.
		Query().
		Where(queue.ID(ctx.Params.QueueId)).
		WithCriteria().
		Only(ctx.Ctx)
}
