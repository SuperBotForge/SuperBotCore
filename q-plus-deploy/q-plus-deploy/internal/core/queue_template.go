package core

import (
	"q+/internal/generated/ent"
	"q+/internal/generated/ent/criterion"
	"q+/internal/generated/ent/queuetemplate"
	"time"
)

type CreateQueueTemplateParams struct {
	ServerCommandParams
	Name           string
	SignUpLeadTime time.Duration
}

var CreateQueueTemplate = wrapTx(createQueueTemplate)

func createQueueTemplate(ctx UseCaseContext[CreateQueueTemplateParams]) (*ent.QueueTemplate, error) {
	course, err := ctx.getCourseAt(ctx.Params.DiscordChannelId)
	if err != nil {
		return nil, err
	}

	markTable, err := course.QueryMarkTable().First(ctx.Ctx)
	if err != nil {
		return nil, err
	}

	markTableTab, err := createMarkTableTab(useCaseContext(ctx, CreateMarkTableTabParams{
		MarkTableId: markTable.ID,
		Name:        ctx.Params.Name,
	}))
	if err != nil {
		return nil, err
	}

	criteria, err := course.QueryCriteria().All(ctx.Ctx)
	if err != nil {
		return nil, err
	}

	template, err := ctx.ent().QueueTemplate.
		Create().
		SetName(ctx.Params.Name).
		SetCourseInstance(course).
		SetSignUpLeadTime(ctx.Params.SignUpLeadTime).
		SetMarkTableTab(markTableTab).
		AddCriteria(criteria...).
		Save(ctx.Ctx)
	if err != nil {
		return nil, err
	}

	markTableTab.Edges.MarkTable = markTable
	template.Edges.MarkTableTab = markTableTab

	return template, nil
}

type ListQueueTemplatesParams struct {
	ServerCommandParams
}

var ListQueueTemplates = wrapTx(listQueueTemplates)

func listQueueTemplates(ctx UseCaseContext[ListQueueTemplatesParams]) ([]*ent.QueueTemplate, error) {
	course, err := ctx.getCourseAt(ctx.Params.DiscordChannelId)
	if err != nil {
		return nil, err
	}

	return course.QueryQueueTemplates().All(ctx.Ctx)
}

type EditQueueTemplateParams struct {
	TemplateId     int64
	Name           *string
	SignUpLeadTime *time.Duration
}

var EditQueueTemplate = wrapTx(editQueueTemplate)

func editQueueTemplate(ctx UseCaseContext[EditQueueTemplateParams]) (*ent.QueueTemplate, error) {
	template, err := ctx.ent().QueueTemplate.Get(ctx.Ctx, ctx.Params.TemplateId)
	if ent.IsNotFound(err) {
		return nil, ErrQueueTemplateNotFound
	}
	if err != nil {
		return nil, err
	}

	updateQuery := template.Update().
		SetNillableSignUpLeadTime(ctx.Params.SignUpLeadTime)
	if ctx.Params.Name != nil {
		updateQuery = updateQuery.SetName(*ctx.Params.Name)
		// TODO update spreadsheet name
	}

	err = updateQuery.Exec(ctx.Ctx)
	if err != nil {
		return nil, err
	}

	return ctx.ent().QueueTemplate.
		Query().
		Where(queuetemplate.ID(template.ID)).
		WithMarkTableTab(
			func(q *ent.MarkTableTabQuery) {
				q.WithMarkTable()
			},
		).
		Only(ctx.Ctx)
}

type SelectCriteriaForQueueTemplateParams struct {
	TemplateId int64
}

type SelectCriteriaForQueueTemplateResponse struct {
	QueueTemplate    *ent.QueueTemplate
	Criteria         []*ent.Criterion
	SelectedCriteria []int64
}

var SelectCriteriaForQueueTemplate = wrapTx(selectCriteriaForQueueTemplate)

func selectCriteriaForQueueTemplate(ctx UseCaseContext[SelectCriteriaForQueueTemplateParams]) (*SelectCriteriaForQueueTemplateResponse, error) {
	template, err := ctx.ent().QueueTemplate.Get(ctx.Ctx, ctx.Params.TemplateId)
	if ent.IsNotFound(err) {
		return nil, ErrQueueTemplateNotFound
	}
	if err != nil {
		return nil, err
	}

	criteria, err := template.QueryCourseInstance().QueryCriteria().All(ctx.Ctx)
	if err != nil {
		return nil, err
	}

	var selectedCriteria []int64
	err = template.QueryCriteria().Select(criterion.FieldID).Scan(ctx.Ctx, &selectedCriteria)
	if err != nil {
		return nil, err
	}

	return &SelectCriteriaForQueueTemplateResponse{
		QueueTemplate:    template,
		Criteria:         criteria,
		SelectedCriteria: selectedCriteria,
	}, nil
}

type QueueTemplateChooseCriterionParams struct {
	QueueTemplateId int64
	CriteriaIds     []int64
}

var QueueTemplateChooseCriterion = wrapTx(queueTemplateChooseCriterion)

func queueTemplateChooseCriterion(ctx UseCaseContext[QueueTemplateChooseCriterionParams]) (*ent.QueueTemplate, error) {
	err := ctx.ent().QueueTemplate.UpdateOneID(ctx.Params.QueueTemplateId).
		ClearCriteria().
		AddCriteriumIDs(ctx.Params.CriteriaIds...).
		Exec(ctx.Ctx)
	if err != nil {
		return nil, err
	}

	return ctx.ent().QueueTemplate.
		Query().
		Where(queuetemplate.ID(ctx.Params.QueueTemplateId)).
		WithCriteria().
		Only(ctx.Ctx)
}
