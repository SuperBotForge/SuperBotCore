package handlers

import (
	"fmt"
	"github.com/samber/lo"
	"q+/internal/core"
	"q+/internal/generated/ent"
	"q+/internal/utils"
	"strings"
)

func CourseInstanceCreate(ctx InteractionContext, options OptionMap) error {
	ctx.log().Trace().Msg("Course instance create command")
	course, err := core.CreateCourseInstance(useCaseContext(ctx, core.CreateCourseInstanceParams{
		ServerCommandParams: ctx.serverCommandParams(),
		Name:                options.String("name"),
	}))
	if err != nil {
		// TODO function HandleCommonError to handle errors with course channels
		return err
	}

	courses, err := core.ListCourseInstances(useCaseContext(ctx, core.ListCourseInstancesParams{
		ServerCommandParams: ctx.serverCommandParams(),
	}))

	//output all courses
	coursesList := strings.Join(lo.Map(courses, func(course *ent.CourseInstance, _ int) string {
		return fmt.Sprintf("'%s', создан %s", course.Name, course.CreatedAt.Format("2006-01-02 15:04"))
	}), "\n")
	googleSheetsLink := utils.CreateGoogleSheetsLink(course.Edges.MarkTable.SpreadsheetID)
	queuesGoogleSheetsLink := utils.CreateGoogleSheetsLink(course.QueuesSpreadsheetID)
	response := fmt.Sprintf("### Предмет '%s' создан\n"+
		"(набор каналов: '%s')\n"+
		"(таблица оценок тоже создана: %s)\n"+
		"(и таблица для очередей: %s)\n"+
		"Предметы на текущем сервере:\n%s",
		course.Name, course.Edges.ChannelsForCourse.Name, googleSheetsLink, queuesGoogleSheetsLink, coursesList)
	return ctx.interactionCommandRespond(response)
}
