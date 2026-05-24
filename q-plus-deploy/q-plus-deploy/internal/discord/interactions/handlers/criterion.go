package handlers

import (
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/pkg/errors"
	"github.com/samber/lo"
	"q+/internal/core"
	"q+/internal/utils"
	"strconv"
)

func CourseCriterionWizard(ctx InteractionContext, options OptionMap) error {

	course, err := core.GetCourseWithCriteriaAtChannel(useCaseContext(ctx, core.GetCourseAtChannelParams{
		ServerCommandParams: ctx.serverCommandParams(),
	}))
	if err != nil {
		return err
	}

	return ctx.interactionResponseEdit(criteriaWizardDeferredResponse(course))
}

func EditCriteria(ctx InteractionContext, entity string, entityId int64) error {
	criteriaList, err := utils.CatchPanicAsError(func() string {
		// TODO declarative way to get value
		// structure from components.FillCriteriaEditModal
		return ctx.I.ModalSubmitData().Components[0].(*discordgo.ActionsRow).Components[0].(*discordgo.TextInput).Value
	})
	if err != nil {
		return err
	}
	switch entity {
	case "course":
		editCriteriaResponse, err := core.EditCriteriaFromTextList(useCaseContext(ctx, core.EditCriteriaParams{
			CourseId:     entityId,
			CriteriaList: criteriaList,
		}))
		if err != nil {
			return err
		}

		return ctx.interactionRespond(criteriaWizardAfterEditingResponse(editCriteriaResponse))
	default:
		return errors.Errorf("unknown entity %s", entity)
	}
}

func ChooseCriteria(ctx InteractionContext, entity string, entityId int64) error {
	values := ctx.I.MessageComponentData().Values

	criteriaIds := lo.Map(values, func(value string, i int) int64 {
		criterionId, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return -1
		}
		return criterionId
	})

	switch entity {
	case "examiner":
		examiner, err := core.ExaminerChooseCriterion(useCaseContext(ctx, core.ExaminerChooseCriterionParams{
			ServerCommandParams: ctx.serverCommandParams(),
			CriteriaIds:         criteriaIds,
			ExaminerId:          entityId,
		}))
		if err != nil {
			return err
		}

		note := ""
		if len(examiner.Note) > 0 {
			note = "с заметкой '" + examiner.Note + "' "
		}
		response := fmt.Sprintf("Теперь <@%s> - принимающий в очереди '%v' %sпо критериям: %s", examiner.Edges.Teacher.DiscordID, examiner.Edges.Queue.Name, note, utils.JoinCriteria(examiner.Edges.Criteria))
		return ctx.interactionRespond(basicResponse(response))
	case "queue-place":
		queuePlace, err := core.QueuePlaceChooseCriterion(useCaseContext(ctx, core.QueuePlaceChooseCriterionParams{
			ServerCommandParams: ctx.serverCommandParams(), // TODO redundant?
			QueuePlaceId:        entityId,
			CriteriaIds:         criteriaIds,
		}))
		if err != nil {
			return err
		}

		response := fmt.Sprintf("%s записаны в очередь '%s' с критериями: %s",
			utils.JoinUserPings(queuePlace.Edges.Team),
			queuePlace.Edges.Queue.Name,
			utils.JoinCriteria(queuePlace.Edges.Criteria))
		return ctx.interactionRespond(basicResponse(response))
	case "queue-template":
		queueTemplate, err := core.QueueTemplateChooseCriterion(useCaseContext(ctx, core.QueueTemplateChooseCriterionParams{
			QueueTemplateId: entityId,
			CriteriaIds:     criteriaIds,
		}))
		if err != nil {
			return err
		}
		response := fmt.Sprintf(
			"Для шаблона очереди '%s' изменены критерии на: %s",
			queueTemplate.Name,
			utils.JoinCriteria(queueTemplate.Edges.Criteria),
		)
		return ctx.interactionRespond(basicResponse(response))
	case "queue":
		queue, err := core.QueueChooseCriterion(useCaseContext(ctx, core.QueueChooseCriterionParams{
			QueueId:     entityId,
			CriteriaIds: criteriaIds,
		}))
		if err != nil {
			return err
		}
		response := fmt.Sprintf(
			"Для очереди '%s' изменены критерии на: %s",
			queue.Name,
			utils.JoinCriteria(queue.Edges.Criteria),
		)
		return ctx.interactionRespond(basicResponse(response))
	default:
		return errors.Errorf("unknown entity %s", entity)
	}

}
