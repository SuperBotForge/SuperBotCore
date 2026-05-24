package handlers

import (
	"fmt"
	"github.com/samber/lo"
	"q+/internal/core"
	"q+/internal/core/discord"
	"q+/internal/generated/ent"
	"q+/internal/utils"
)

func Next(ctx InteractionContext, options OptionMap) error {
	ctx.log().Trace().Msg("Next command")

	user := getUser(ctx.I)

	nextResponse, err := core.Next(useCaseContext(ctx, core.NextParams{
		ServerCommandParams: ctx.serverCommandParams(),
		User:                user,
	}))
	if err != nil {
		return err
	}

	return nextResp(ctx, nextResponse, user)
}

func nextResp(ctx InteractionContext, nextResponse *core.NextResponse, user *discord.User) error {
	if nextResponse.NextQueuePlace == nil {
		response := ""
		if nextResponse.BusyCount+nextResponse.NotBusyCount == 0 {
			response = "Кажется, к вам никого больше нет"
		} else if nextResponse.NotBusyCount == 0 && nextResponse.BusyCount > 0 {
			response = fmt.Sprintf("К вам записаны еще %v человек/команд, но они все заняты", nextResponse.BusyCount)
		} else {
			response = fmt.Sprintf("К вам записаны еще %v человек/команд (%v из них заняты), "+
				"но next не подобрался, попробуйте еще раз", nextResponse.BusyCount+nextResponse.NotBusyCount, nextResponse.BusyCount)
		}
		return ctx.interactionCommandRespondEphemeral(response)
	}

	return callStudentTeam(
		ctx,
		user.DiscordId,
		nextResponse.Examiner,
		nextResponse.NextQueuePlace,
		nextResponse.NextQueuePlace.Edges.Team,
		nextResponse.Criteria,
	)
}

func callStudentTeam(
	ctx InteractionContext,
	examDiscordId string,
	exam *ent.Examiner,
	queuePlace *ent.QueuePlace,
	studentTeam []*ent.User,
	criteria []*ent.Criterion,
) error {
	voiceState, err := ctx.S.State.VoiceState(ctx.I.GuildID, examDiscordId)
	voiceChannelMsg := ""
	if err == nil && voiceState != nil && voiceState.ChannelID != "" {
		voiceChannelMsg = fmt.Sprintf(", пожалуйста зайдите в канал <#%s>", voiceState.ChannelID)
	}
	examNote := ""
	if len(exam.Note) > 0 {
		examNote = "\nЗаметка принимающего: '" + exam.Note + "'"
	}
	queuePlaceNote := ""
	if len(queuePlace.Note) > 0 {
		queuePlaceNote = "\nЗаметка сдающего: '" + queuePlace.Note + "'"
	}

	return ctx.interactionCommandRespondFollowup(
		fmt.Sprintf(
			"Принимающий <@%s> вызывает %s на сдачу '%s'%s%s%s",
			examDiscordId,
			utils.JoinUserPings(studentTeam),
			utils.JoinCriteria(criteria),
			voiceChannelMsg,
			examNote,
			queuePlaceNote,
		),
		true,
	)
}

func StartSignUp(ctx InteractionContext, options OptionMap) error {
	ctx.log().Trace().Msg("Start sign-up command")

	queue, err := core.StartSignUp(useCaseContext(ctx, core.StartSignUpParams{
		ServerCommandParams: ctx.serverCommandParams(),
		QueueId:             options.Int("queue"),
	}))
	if err != nil {
		return err
	}

	return ctx.interactionCommandRespond(fmt.Sprintf("Запись в очередь '%s' открыта", queue.Name))
}

func StartQueue(ctx InteractionContext, options OptionMap) error {
	ctx.log().Trace().Msg("Start queue command")

	queue, err := core.StartQueue(useCaseContext(ctx, core.StartQueueParams{
		ServerCommandParams: ctx.serverCommandParams(),
		QueueId:             options.Int("queue"),
	}))
	if err != nil {
		return err
	}

	return ctx.interactionCommandRespond(fmt.Sprintf("Очередь '%s' запущена", queue.Name))
}

func EndQueue(ctx InteractionContext, _ OptionMap) error {
	ctx.log().Trace().Msg("End queue command")

	queue, err := core.EndQueue(useCaseContext(ctx, core.EndQueueParams{
		ServerCommandParams: ctx.serverCommandParams(),
	}))
	if err != nil {
		return err
	}

	if queue == nil {
		return ctx.interactionCommandRespondEphemeral("❗ Нет активной очереди")
	}

	return ctx.interactionCommandRespond(fmt.Sprintf("Очередь '%s' завершена", queue.Name))
}

func SignUp(ctx InteractionContext, options OptionMap) error {
	ctx.log().Trace().Msg("Sign up command")

	signUpResponse, err := core.SignUp(useCaseContext(ctx, core.SignUpParams{
		ServerCommandParams: ctx.serverCommandParams(),
		QueueId:             options.Int("queue"),
		Note:                lo.FromPtr(options.OptString("note")),
		Team: lo.Compact([]*discord.User{
			memberToUser(ctx.I.Member),
			memberToUser(options.OptUser("teammate_2", ctx)),
			memberToUser(options.OptUser("teammate_3", ctx)),
			memberToUser(options.OptUser("teammate_4", ctx)),
			memberToUser(options.OptUser("teammate_5", ctx)),
			memberToUser(options.OptUser("teammate_6", ctx)),
		}),
	}))
	if err != nil {
		return err
	}

	if len(signUpResponse.Criteria) > 25 {
		return ctx.interactionCommandRespondEphemeral("❗ В предмете больше 25 критериев, этот способ выбора пока не может работать")
	}
	if len(signUpResponse.Criteria) == 0 {
		response := fmt.Sprintf("%s записаны в очередь",
			utils.JoinUserPings(signUpResponse.QueuePlace.Edges.Team))
		return ctx.interactionCommandRespond(response)
	}

	return ctx.interactionCommandRespondCustom(signUpSelectMenuResponse(signUpResponse))
}

func Leave(ctx InteractionContext, options OptionMap) error {
	ctx.log().Trace().Msg("Leave command")

	queuePlace, err := core.Leave(useCaseContext(ctx, core.LeaveParams{
		ServerCommandParams: ctx.serverCommandParams(),
		User:                getUser(ctx.I),
		QueuePlaceId:        options.Int("place"),
	}))
	if err != nil {
		return err
	}
	response := fmt.Sprintf("Сдающие %s (%s) удалены из очереди",
		utils.JoinUserPings(queuePlace.Edges.Team),
		utils.JoinCriteria(lo.Map(queuePlace.Edges.QueuePlaceCriteria, func(c *ent.QueuePlaceCriterion, _ int) *ent.Criterion {
			return c.Edges.Criterion
		})))

	return ctx.interactionCommandRespond(response)
}

func Pause(ctx InteractionContext, options OptionMap) error {
	ctx.log().Trace().Msg("Pause command")

	exam, err := core.Pause(useCaseContext(ctx, core.PauseParams{
		ServerCommandParams: ctx.serverCommandParams(),
		User:                getUser(ctx.I),
	}))
	if err != nil {
		return err
	}

	resp := fmt.Sprintf("<@%s> ушел/ушла на перерыв, но обещал(а) вернуться! (через %d мин)", exam.Edges.Teacher.DiscordID, options.Int("duration"))
	return ctx.interactionCommandRespond(resp)
}

func Pick(ctx InteractionContext, options OptionMap) error {
	ctx.log().Trace().Msg("Pick command")

	pickResponse, err := core.Pick(useCaseContext(ctx, core.PickParams{
		ServerCommandParams: ctx.serverCommandParams(),
		User:                getUser(ctx.I),
		QueuePlaceId:        options.Int("place"),
	}))
	if err != nil {
		return err
	}

	return callStudentTeam(
		ctx,
		pickResponse.Examiner.Edges.Teacher.DiscordID,
		pickResponse.Examiner,
		pickResponse.NextQueuePlace,
		pickResponse.NextQueuePlace.Edges.Team,
		pickResponse.Criteria,
	)
}

func Reping(ctx InteractionContext, options OptionMap) error {
	ctx.log().Trace().Msg("Reping command")

	user := getUser(ctx.I)

	repingResponse, err := core.Reping(useCaseContext(ctx, core.RepingParams{
		ServerCommandParams: ctx.serverCommandParams(),
		User:                user,
	}))
	if err != nil {
		return err
	}

	if repingResponse.NextQueuePlace == nil {
		response := "У вас сейчас нет вызванного студента/команды"
		return ctx.interactionCommandRespondEphemeral(response)
	}

	return callStudentTeam(
		ctx,
		user.DiscordId,
		repingResponse.Examiner,
		repingResponse.NextQueuePlace,
		repingResponse.NextQueuePlace.Edges.Team,
		repingResponse.Criteria,
	)
}
