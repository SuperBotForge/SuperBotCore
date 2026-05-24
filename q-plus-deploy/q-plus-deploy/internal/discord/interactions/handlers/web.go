package handlers

import (
	"fmt"
	"q+/internal/core"
)

func WebQueueTemplate(ctx InteractionContext, _ OptionMap) error {
	ctx.log().Trace().Msg("Web queue template command")

	course, err := core.GetCourseAtChannel(useCaseContext(ctx, core.GetCourseAtChannelParams{
		ServerCommandParams: ctx.serverCommandParams(),
	}))
	if err != nil {
		return err
	}

	url, err := createAuthorizedLinkFromCourse(ctx, course.ID, fmt.Sprintf(
		"/create-template/%d",
		course.ID,
	))
	if err != nil {
		return err
	}

	return ctx.interactionCommandRespond(url)
}

func WebScheduleQueues(ctx InteractionContext, options OptionMap) error {
	ctx.log().Trace().Msg("Web schedule queues command")

	url, err := createAuthorizedLink(ctx, fmt.Sprintf(
		"/schedule-queues/%d",
		options.Int("template"),
	))
	if err != nil {
		return err
	}

	return ctx.interactionCommandRespond(url)
}

func WebQueueList(ctx InteractionContext, options OptionMap) error {
	ctx.log().Trace().Msg("Queue list command")

	url, err := createQueueListLink(ctx, options.Int("template"))
	if err != nil {
		return err
	}

	return ctx.interactionCommandRespond(url)
}

func createQueueListLink(ctx InteractionContext, templateId int64) (string, error) {
	return createAuthorizedLink(ctx, fmt.Sprintf(
		"/queues/%d",
		templateId,
	))
}

func createAuthorizedLink(ctx InteractionContext, url string) (string, error) {
	course, err := core.GetCourseAtChannel(useCaseContext(ctx, core.GetCourseAtChannelParams{
		ServerCommandParams: ctx.serverCommandParams(),
	}))
	if err != nil {
		return "", err
	}

	return createAuthorizedLinkFromCourse(ctx, course.ID, url)
}

func createAuthorizedLinkFromCourse(ctx InteractionContext, courseId int64, url string) (string, error) {
	jwt, err := ctx.Coder.CreateJwtTokenWithCourseId(courseId)
	if err != nil {
		return "", err
	}

	return ctx.FrontendBaseUrl + url + "#" + jwt, nil
}
