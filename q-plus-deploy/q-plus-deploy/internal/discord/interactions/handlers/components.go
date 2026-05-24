package handlers

import (
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/pkg/errors"
	"github.com/samber/lo"
	"q+/internal/core"
	"q+/internal/generated/ent"
	"strings"
)

type ComponentMaker interface {
	Components(entity string, entityId int64) []discordgo.MessageComponent
}

func EditCriteriaModal(ctx InteractionContext, entity string, entityId int64) error {
	switch entity { // TODO enum
	case "course":
		course, err := core.GetCourse(useCaseContext(ctx, core.GetCourseParams{CourseId: entityId}))
		if err != nil {
			return err
		}
		criteriaList := strings.Join(lo.Map(course.Edges.Criteria, func(c *ent.Criterion, i int) string {
			return fmt.Sprintf("%d. %s", i+1, c.Name)
		}), "\n")
		ctx.log().Debug().Msgf("criteria list: %s", criteriaList)

		return ctx.interactionRespond(editCriteriaModal(course, criteriaList))
	default:
		return errors.Errorf("unknown entity %s", entity)
	}
}
