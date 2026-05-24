package core

import (
	"q+/internal/generated/ent"
	"q+/internal/generated/ent/discordserver"
)

type CreateChannelsForCourseParams struct {
	ServerCommandParams
	Name             string
	TeacherChannelId string
	StudentChannelId string
	QueueChannelId   string
}

var CreateChannelsForCourse = wrapTx(createChannelsForCourse)

func createChannelsForCourse(ctx UseCaseContext[CreateChannelsForCourseParams]) (*ent.ChannelsForCourse, error) {
	err := ctx.createDiscordServer(ctx.Params.DiscordServerID)
	if err != nil {
		return nil, err
	}

	// TODO unique channels on server?

	channelsForCourse, err := ctx.ent().ChannelsForCourse.
		Create().
		SetName(ctx.Params.Name).
		SetDiscordServerID(ctx.Params.DiscordServerID).
		SetTeacherChannelID(ctx.Params.TeacherChannelId).
		SetStudentChannelID(ctx.Params.StudentChannelId).
		SetQueueChannelID(ctx.Params.QueueChannelId).
		Save(ctx.Ctx)
	if err != nil {
		return nil, err
	}

	ctx.log().Debug().
		Str("event", "channels_for_course_create").
		Int64("channels_for_course_id", channelsForCourse.ID).
		Str("name", channelsForCourse.Name).
		Msg("Channels for course created")

	return channelsForCourse, nil
}

type ListChannelsForCourseParams struct {
	ServerCommandParams
}

func ListChannelsForCourse(ctx UseCaseContext[ListChannelsForCourseParams]) ([]*ent.ChannelsForCourse, error) {
	return ctx.ent().DiscordServer.
		Query().
		Where(discordserver.ID(ctx.Params.DiscordServerID)).
		QueryChannelsForCourses().
		All(ctx.Ctx)
}

type GetChannelsForCourseParams struct {
	ServerCommandParams
}

var GetChannelsForCourse = wrapTx(getChannelsForCourse)

func getChannelsForCourse(ctx UseCaseContext[GetChannelsForCourseParams]) (*ent.ChannelsForCourse, error) {
	return ctx.getChannelsForCourse(ctx.Params.DiscordChannelId)
}

type GetTypeOfChannelParams struct {
	ServerCommandParams
}

var GetTypeOfChannel = wrapTx(getTypeOfChannel)

func getTypeOfChannel(ctx UseCaseContext[GetTypeOfChannelParams]) (DiscordChannelType, error) {
	channelsForCourse, err := ctx.getChannelsForCourse(ctx.Params.DiscordChannelId)
	if err != nil {
		return 0, err
	}

	switch ctx.Params.DiscordChannelId {
	case channelsForCourse.TeacherChannelID:
		return ChannelTeacher, nil
	case channelsForCourse.StudentChannelID:
		return ChannelStudent, nil
	case channelsForCourse.QueueChannelID:
		return ChannelQueue, nil
	default:
		return 0, nil
	}
}
