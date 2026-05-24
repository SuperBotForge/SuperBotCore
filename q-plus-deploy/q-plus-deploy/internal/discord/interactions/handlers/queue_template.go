package handlers

import (
	"fmt"
	"github.com/kr/pretty"
	"github.com/samber/lo"
	"q+/internal/core"
	"q+/internal/generated/ent"
	"q+/internal/utils"
	"strings"
)

func QueueTemplateCreate(ctx InteractionContext, options OptionMap) error {
	ctx.log().Trace().Msg("Queue template create command")

	template, err := core.CreateQueueTemplate(useCaseContext(ctx, core.CreateQueueTemplateParams{
		ServerCommandParams: ctx.serverCommandParams(),
		Name:                options.String("name"),
		SignUpLeadTime:      options.Duration("sign_up_lead_time"),
	}))
	if err != nil {
		return err
	}

	templates, err := core.ListQueueTemplates(useCaseContext(ctx, core.ListQueueTemplatesParams{
		ServerCommandParams: ctx.serverCommandParams(),
	}))

	//output all templates
	templatesList := strings.Join(lo.Map(templates, func(template *ent.QueueTemplate, _ int) string {
		return pretty.Sprintf("'%s', обновлен %s", template.Name, template.UpdatedAt.Format("2006-01-02 15:04"))
	}), "\n")

	googleSheetsLink := utils.CreateGoogleSheetsSheetLink(template.Edges.MarkTableTab.Edges.MarkTable.SpreadsheetID, template.Edges.MarkTableTab.SheetID)
	return ctx.interactionCommandRespond(fmt.Sprintf("### Шаблон очереди '%s' создан\n(вкладка для оценок тоже создана: %s)\nШаблоны на текущем предмете:\n%s", template.Name, googleSheetsLink, templatesList))
}

func QueueTemplateList(ctx InteractionContext, options OptionMap) error {
	ctx.log().Trace().Msg("Queue template list command")

	templates, err := core.ListQueueTemplates(useCaseContext(ctx, core.ListQueueTemplatesParams{
		ServerCommandParams: ctx.serverCommandParams(),
	}))
	if err != nil {
		return err
	}

	//output all templates
	templatesList := strings.Join(lo.Map(templates, func(template *ent.QueueTemplate, _ int) string {
		return fmt.Sprintf(
			"**%s**, обновлен %s:\n"+
				"- За сколько времени до начала очереди открывать запись: %s\n",
			template.Name,
			template.UpdatedAt.Format("2006-01-02 15:04"),
			utils.PrintDuration(template.SignUpLeadTime),
		)
	}), "\n")

	return ctx.interactionCommandRespond(fmt.Sprintf("### Список шаблонов очередей:\n%s", templatesList))
}

func QueueTemplateEdit(ctx InteractionContext, options OptionMap) error {
	ctx.log().Trace().Msg("Queue template edit command")

	template, err := core.EditQueueTemplate(useCaseContext(ctx, core.EditQueueTemplateParams{
		TemplateId:     options.Int("template"),
		Name:           options.OptString("name"),
		SignUpLeadTime: options.OptDuration("sign_up_lead_time"),
	}))
	if err != nil {
		return err
	}

	googleSheetsLink := utils.CreateGoogleSheetsSheetLink(template.Edges.MarkTableTab.Edges.MarkTable.SpreadsheetID, template.Edges.MarkTableTab.SheetID)
	return ctx.interactionCommandRespond(fmt.Sprintf(
		"### Шаблон очереди '%s' отредактирован\n(вкладка для оценок: %s)\n- За сколько времени до начала очереди открывать запись: %s",
		template.Name,
		googleSheetsLink,
		utils.PrintDuration(template.SignUpLeadTime),
	))
}

func QueueTemplateSelectCriteria(ctx InteractionContext, options OptionMap) error {
	ctx.log().Trace().Msg("Queue template select criteria command")

	response, err := core.SelectCriteriaForQueueTemplate(useCaseContext(ctx, core.SelectCriteriaForQueueTemplateParams{
		TemplateId: options.Int("template"),
	}))
	if err != nil {
		return err
	}

	if len(response.Criteria) > 25 {
		return ctx.interactionCommandRespondEphemeral("❗ В предмете больше 25 критериев, этот способ выбора пока не может работать")
	}

	if len(response.Criteria) == 0 {
		return ctx.interactionCommandRespondEphemeral("❗ В предмете нет критериев")
	}

	return ctx.interactionCommandRespondCustom(queueTemplateCriteriaSelectMenuResponse(response))
}
