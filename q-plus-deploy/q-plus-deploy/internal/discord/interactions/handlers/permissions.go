package handlers

import (
	"fmt"
	"q+/internal/core"
	"q+/internal/generated/ent"
	"strings"
)

func CheckChannelType(ctx InteractionContext, allowedChannelTypes core.DiscordChannelType) error {
	ctx.log().Trace().Msg("Check channel type")
	if allowedChannelTypes == core.ChannelAny {
		return nil
	}

	channelType, err := core.GetTypeOfChannel(useCaseContext(ctx, core.GetTypeOfChannelParams{
		ServerCommandParams: ctx.serverCommandParams(),
	}))
	if err != nil {
		return err
	}

	// allowedChannelTypes is a bitmask of allowed channel types
	if allowedChannelTypes&channelType != 0 {
		return nil
	}

	channelsForCourse, err := core.GetChannelsForCourse(useCaseContext(ctx, core.GetChannelsForCourseParams{
		ServerCommandParams: ctx.serverCommandParams(),
	}))
	if err != nil {
		return err
	}

	allowedChannelsMessage := getAllowedChannelsMessage(allowedChannelTypes, channelsForCourse)

	errMsg := "❗ Эта команда не может быть выполнена в этом канале. Вы можете выполнить её в " + allowedChannelsMessage + // TODO render mentions in autocomplete error
		"" //"\nНажмите Стрелку вверх, Ctrl+A, Ctrl+X, перейдите в нужный канал, Ctrl+V и Enter."
	return &core.HumanReadableError{
		Err:       fmt.Errorf("channel type not allowed: %w", core.ErrBadRequest),
		UserError: errMsg,
	}
}

func getAllowedChannelsMessage(allowedChannelTypes core.DiscordChannelType, channelsForCourse *ent.ChannelsForCourse) string {
	var message string
	if allowedChannelTypes&core.ChannelTeacher != 0 {
		message += fmt.Sprintf("<#%s>, ", channelsForCourse.TeacherChannelID)
	}
	if allowedChannelTypes&core.ChannelStudent != 0 {
		message += fmt.Sprintf("<#%s>, ", channelsForCourse.StudentChannelID)
	}
	if allowedChannelTypes&core.ChannelQueue != 0 {
		message += fmt.Sprintf("<#%s>, ", channelsForCourse.QueueChannelID)
	}
	return strings.TrimSuffix(message, ", ")
}
