package core

import (
	"q+/internal/generated/ent"
	"q+/internal/generated/ent/discordserver"
)

type CreateCourseInstanceParams struct {
	ServerCommandParams
	Name string
}

var CreateCourseInstance = wrapTx(createCourseInstance)

func createCourseInstance(ctx UseCaseContext[CreateCourseInstanceParams]) (*ent.CourseInstance, error) {
	err := ctx.createDiscordServer(ctx.Params.DiscordServerID)
	if err != nil {
		return nil, err
	}

	channelsForCourse, err := ctx.getChannelsForCourse(ctx.Params.DiscordChannelId)
	if err != nil {
		return nil, err
	}

	queueTableName := ctx.Params.Name + " Очереди"

	queuesSpreadsheetId, _, err := ctx.Core.sheetsService.CreateSpreadsheet(ctx.Ctx, queueTableName)
	if err != nil {
		return nil, err
	}

	course, err := ctx.ent().CourseInstance.
		Create().
		SetName(ctx.Params.Name).
		SetDiscordServerID(ctx.Params.DiscordServerID).
		SetChannelsForCourse(channelsForCourse).
		SetQueuesSpreadsheetID(queuesSpreadsheetId).
		Save(ctx.Ctx)
	if err != nil {
		return nil, err
	}

	markTableName := ctx.Params.Name + " Оценки"

	spreadsheetId, _, err := ctx.Core.sheetsService.CreateSpreadsheet(ctx.Ctx, markTableName)
	if err != nil {
		return nil, err
	}

	markTable, err := ctx.ent().MarkTable.
		Create().
		SetName(ctx.Params.Name + " Оценки").
		SetCourseInstance(course).
		SetSpreadsheetID(spreadsheetId).
		Save(ctx.Ctx)
	if err != nil {
		return nil, err
	}

	course.Edges.ChannelsForCourse = channelsForCourse // TODO wtf???
	course.Edges.MarkTable = markTable

	ctx.log().Debug().
		Str("event", "course_instance_create").
		Int64("course_instance_id", course.ID).
		Str("name", course.Name).
		Msg("Course instance created")

	return course, nil
}

type ListCourseInstancesParams struct {
	ServerCommandParams
}

var ListCourseInstances = wrapTx(listCourseInstances)

func listCourseInstances(ctx UseCaseContext[ListCourseInstancesParams]) ([]*ent.CourseInstance, error) {
	return ctx.ent().DiscordServer.
		Query().
		Where(discordserver.ID(ctx.Params.DiscordServerID)).
		QueryCourseInstances().
		All(ctx.Ctx)
}

var GetCourseAtChannel = wrapTx(getCourseAtChannel)

func getCourseAtChannel(ctx UseCaseContext[GetCourseAtChannelParams]) (*ent.CourseInstance, error) {
	return ctx.getCourseAt(ctx.Params.DiscordChannelId)
}
