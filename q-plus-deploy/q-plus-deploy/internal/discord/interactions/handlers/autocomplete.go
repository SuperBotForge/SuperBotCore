package handlers

import (
	"github.com/bwmarrin/discordgo"
	"github.com/samber/lo"
	"q+/internal/core"
	"q+/internal/generated/ent"
	"q+/internal/utils"
	"strings"
)

func AutocompleteCourseList(ctx AutocompleteContext, value string) error {
	ctx.log().Trace().Msg("Autocomplete course list")

	courses, err := core.SearchCourseInstances(useCaseContext(ctx.InteractionContext, core.SearchCourseInstancesParams{
		ServerCommandParams: ctx.serverCommandParams(),
		Query:               value,
	}))
	if err != nil {
		return err
	}

	choices := lo.Map(courses, func(course *ent.CourseInstance, _ int) *discordgo.ApplicationCommandOptionChoice {
		return &discordgo.ApplicationCommandOptionChoice{
			Name:  course.Name,
			Value: course.ID,
		}
	})
	ctx.log().Trace().Msg("Choices formed")
	return ctx.interactionAutocompleteRespond(choices)
}

func AutocompleteQueueTemplateList(ctx AutocompleteContext, value string) error {
	ctx.log().Trace().Msg("Autocomplete queue template list")

	templates, err := core.SearchQueueTemplates(useCaseContext(ctx.InteractionContext, core.SearchQueueTemplatesParams{
		ServerCommandParams: ctx.serverCommandParams(),
		Query:               value,
	}))
	if err != nil {
		return err
	}

	choices := lo.Map(templates, func(template *ent.QueueTemplate, _ int) *discordgo.ApplicationCommandOptionChoice {
		return &discordgo.ApplicationCommandOptionChoice{
			Name:  template.Name,
			Value: template.ID,
		}
	})
	ctx.log().Trace().Msg("Choices formed")
	return ctx.interactionAutocompleteRespond(choices)

}

func AutocompleteQueueList(ctx AutocompleteContext, value string) error {
	ctx.log().Trace().Msg("Autocomplete queue list")

	queues, err := core.SearchQueues(useCaseContext(ctx.InteractionContext, core.SearchQueuesParams{
		ServerCommandParams: ctx.serverCommandParams(),
		Query:               value,
	}))
	if err != nil {
		return err
	}

	choices := lo.Map(queues, func(queue *ent.Queue, _ int) *discordgo.ApplicationCommandOptionChoice {
		return &discordgo.ApplicationCommandOptionChoice{
			Name:  queue.Name,
			Value: queue.ID,
		}
	})
	ctx.log().Trace().Msg("Choices formed")
	return ctx.interactionAutocompleteRespond(choices)
}

func AutocompleteSignUpStartedQueueList(ctx AutocompleteContext, value string) error {
	ctx.log().Trace().Msg("Autocomplete sign-up started queue list")

	queues, err := core.SearchSignUpStartedQueues(useCaseContext(ctx.InteractionContext, core.SearchQueuesParams{
		ServerCommandParams: ctx.serverCommandParams(),
		Query:               value,
	}))
	if err != nil {
		return err
	}

	choices := lo.Map(queues, func(queue *ent.Queue, _ int) *discordgo.ApplicationCommandOptionChoice {
		return &discordgo.ApplicationCommandOptionChoice{
			Name:  queue.Name,
			Value: queue.ID,
		}
	})
	ctx.log().Trace().Msg("Choices formed")
	return ctx.interactionAutocompleteRespond(choices)
}

func AutocompleteNotEndedQueueList(ctx AutocompleteContext, value string) error {
	ctx.log().Trace().Msg("Autocomplete not ended queue list")

	queues, err := core.SearchNotEndedQueues(useCaseContext(ctx.InteractionContext, core.SearchQueuesParams{
		ServerCommandParams: ctx.serverCommandParams(),
		Query:               value,
	}))
	if err != nil {
		return err
	}

	choices := lo.Map(queues, func(queue *ent.Queue, _ int) *discordgo.ApplicationCommandOptionChoice {
		return &discordgo.ApplicationCommandOptionChoice{
			Name:  queue.Name,
			Value: queue.ID,
		}
	})
	ctx.log().Trace().Msg("Choices formed")
	return ctx.interactionAutocompleteRespond(choices)

}

func AutocompleteCurrentQueuePlaceCriteriaList(ctx AutocompleteContext, value string) error {
	ctx.log().Trace().Msg("Autocomplete current queue place criteria list")

	criteria, err := core.SearchCurrentQueuePlaceCriteria(useCaseContext(ctx.InteractionContext, core.SearchCurrentQueuePlaceCriteriaParams{
		ServerCommandParams: ctx.serverCommandParams(),
		Query:               value,
		Teacher:             getUser(ctx.I),
	}))
	if err != nil {
		return err
	}

	choices := lo.Map(criteria, func(criterion *ent.Criterion, _ int) *discordgo.ApplicationCommandOptionChoice {
		return &discordgo.ApplicationCommandOptionChoice{
			Name:  criterion.Name,
			Value: criterion.ID,
		}
	})
	ctx.log().Trace().Msg("Choices formed")
	return ctx.interactionAutocompleteRespond(choices)
}

func AutocompleteQueuePlacesForCurrentUser(ctx AutocompleteContext, value string) error {
	ctx.log().Trace().Msg("Autocomplete queue places for current user")

	queueId := int64(-1)
	rawQueueId := ctx.Options[0].Value
	switch rawQueueId.(type) {
	case float64:
		queueId = int64(rawQueueId.(float64))
	case int64:
		queueId = rawQueueId.(int64)
	default:
		return &core.HumanReadableError{
			Err:       core.ErrBadRequest,
			UserError: "❗ Сначала выберите очередь",
		}
	}

	ctx.Options[0].IntValue()

	places, err := core.SearchQueuePlacesForCurrentUser(useCaseContext(ctx.InteractionContext, core.SearchQueuePlacesForCurrentUserParams{
		ServerCommandParams: ctx.serverCommandParams(),
		User:                getUser(ctx.I),
		QueueId:             queueId,
		Query:               value,
	}))
	if err != nil {
		return err
	}

	choices := queuePlacesToChoices(places)
	ctx.log().Trace().Msg("Choices formed")
	return ctx.interactionAutocompleteRespond(choices)
}

func AutocompleteQueuePlacesForExaminer(ctx AutocompleteContext, value string) error {
	ctx.log().Trace().Msg("Autocomplete queue places for examiner")

	places, err := core.SearchQueuePlacesForExaminer(useCaseContext(ctx.InteractionContext, core.SearchQueuePlacesForExaminerParams{
		ServerCommandParams: ctx.serverCommandParams(),
		Teacher:             getUser(ctx.I),
		Query:               value,
	}))
	if err != nil {
		return err
	}

	choices := queuePlacesToChoices(places)
	ctx.log().Trace().Msg("Choices formed")
	return ctx.interactionAutocompleteRespond(choices)

}

func queuePlacesToChoices(places []*ent.QueuePlace) []*discordgo.ApplicationCommandOptionChoice {
	places = places[:min(len(places), 25)] // max 25 choices in Discord API
	choices := lo.Map(places, func(place *ent.QueuePlace, _ int) *discordgo.ApplicationCommandOptionChoice {
		name := utils.LimitString(
			strings.Join(lo.Map(place.Edges.Team, func(user *ent.User, _ int) string { return core.GetName(user) }), ", ")+
				" "+
				utils.JoinCriteria(lo.Map(place.Edges.QueuePlaceCriteria, func(c *ent.QueuePlaceCriterion, _ int) *ent.Criterion {
					return c.Edges.Criterion
				})),
			100,
		)
		return &discordgo.ApplicationCommandOptionChoice{
			Name:  name,
			Value: place.ID,
		}
	})
	return choices
}
