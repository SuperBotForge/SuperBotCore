package handlers

import (
	"fmt"
	"github.com/samber/lo"
	"q+/internal/core"
	"q+/internal/generated/ent"
	"q+/internal/utils"
	"strings"
)

func QueueCreate(ctx InteractionContext, options OptionMap) error {
	ctx.log().Trace().Msg("Queue create command")

	queue, err := core.CreateQueue(useCaseContext(ctx, core.CreateQueueParams{
		QueueTemplateId: options.Int("template"),
		Name:            options.OptString("name"),
		StartTime:       options.OptTime("start"),
		EndTime:         options.OptTime("end"),
		SignUpLeadTime:  options.OptDuration("sign_up_lead_time"),
	}))
	if err != nil {
		return err
	}

	return ctx.interactionCommandRespond(fmt.Sprintf("Очередь '%s' создана", queue.Name))
}

func QueueList(ctx InteractionContext, options OptionMap) error {
	ctx.log().Trace().Msg("Queue list command")
	templateId := options.Int("template")

	queues, err := core.ListQueueTemplateQueues(useCaseContext(ctx, core.ListQueueTemplateQueuesParams{
		QueueTemplateId: templateId,
	}))
	if err != nil {
		return err
	}
	url, err := createQueueListLink(ctx, templateId)
	if err != nil {
		return err
	}

	response := strings.Join(lo.Map(queues, func(queue *ent.Queue, _ int) string {
		return fmt.Sprintf(
			"**%s**\n- с %s\n- по %s\n- старт записи за %s\n- критериев: %d\n- принимающих: %d\n- записей: %d",
			queue.Name,
			utils.FormatNilTime(queue.StartTime, "2006-01-02 15:04"), // TODO time zone (discord render timestamp)
			utils.FormatNilTime(queue.EndTime, "2006-01-02 15:04"),
			utils.PrintDuration(queue.SignUpLeadTime),
			len(queue.Edges.Criteria),
			len(queue.Edges.Examiners),
			len(queue.Edges.Places),
		)
	}), "\n")

	return ctx.interactionCommandRespond(fmt.Sprintf(
		"### Очереди на шаблон '%d':\n(%s)\n%s",
		templateId, // TODO get template name
		url,
		response,
	))
}

func QueueTeacherSet(ctx InteractionContext, options OptionMap) error {
	ctx.log().Trace().Msg("Queue teacher set command")

	teacherMember := options.OptUser("teacher", ctx)
	if teacherMember == nil {
		teacherMember = ctx.I.Member
	}

	setTeacherResponse, err := core.SetTeacher(useCaseContext(ctx, core.SetTeacherParams{
		ServerCommandParams: ctx.serverCommandParams(),
		QueueId:             options.Int("queue"),
		Teacher:             memberToUser(teacherMember),
		Note:                options.OptString("note"),
	}))
	if err != nil {
		return err
	}

	if len(setTeacherResponse.Criteria) > 25 {
		return ctx.interactionCommandRespondEphemeral("❗ В предмете больше 25 критериев, этот способ выбора пока не может работать")
	}
	if len(setTeacherResponse.Criteria) == 0 {
		note := ""
		if len(setTeacherResponse.Examiner.Note) > 0 {
			note = " с заметкой '" + setTeacherResponse.Examiner.Note + "' "
		}
		response := fmt.Sprintf("Теперь <@%s> - принимающий в очереди '%v'%s", setTeacherResponse.Examiner.Edges.Teacher.DiscordID, setTeacherResponse.Examiner.Edges.Queue.Name, note)
		return ctx.interactionCommandRespond(response)
	}

	return ctx.interactionCommandRespondCustom(queueTeacherSetSelectMenuResponse(setTeacherResponse))
}

func QueueTeacherDelete(ctx InteractionContext, options OptionMap) error {
	ctx.log().Trace().Msg("Queue teacher delete command")

	teacherMember := options.OptUser("teacher", ctx)
	if teacherMember == nil {
		teacherMember = ctx.I.Member
	}

	res, err := core.DeleteTeacher(useCaseContext(ctx, core.DeleteTeacherParams{
		ServerCommandParams: ctx.serverCommandParams(),
		QueueId:             options.Int("queue"),
		Teacher:             memberToUser(teacherMember),
	}))
	if err != nil {
		return err
	}

	return ctx.interactionCommandRespond(fmt.Sprintf(
		"Принимающий <@%s> удален из очереди '%s'",
		res.Examiner.DiscordID,
		res.Queue.Name,
	))
}

func QueueTeacherGet(ctx InteractionContext, options OptionMap) error {
	ctx.log().Trace().Msg("Queue teacher get command")
	panic("not implemented")
}

func QueueEdit(ctx InteractionContext, options OptionMap) error {
	ctx.log().Trace().Msg("Queue edit command")

	queue, err := core.EditQueue(useCaseContext(ctx, core.EditQueueParams{
		QueueId:        options.Int("queue"),
		Name:           options.OptString("name"),
		StartTime:      options.OptTime("start"),
		EndTime:        options.OptTime("end"),
		SignUpLeadTime: options.OptDuration("sign_up_lead_time"),
	}))
	if err != nil {
		return err
	}

	return ctx.interactionCommandRespond(fmt.Sprintf("Очередь '%s' отредактирована", queue.Name))
}

func QueueSelectCriteria(ctx InteractionContext, options OptionMap) error {
	ctx.log().Trace().Msg("Queue select criteria command")

	response, err := core.SelectCriteriaForQueue(useCaseContext(ctx, core.SelectCriteriaForQueueParams{
		QueueId: options.Int("queue"),
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

	return ctx.interactionCommandRespondCustom(queueCriteriaSelectMenuResponse(response))
}
