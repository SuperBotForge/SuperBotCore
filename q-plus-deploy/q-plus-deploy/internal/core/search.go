package core

import (
	"entgo.io/ent/dialect/sql"
	"github.com/samber/lo"
	"q+/internal/core/discord"
	"q+/internal/generated/ent"
	"q+/internal/generated/ent/courseinstance"
	"q+/internal/generated/ent/criterion"
	"q+/internal/generated/ent/discordserver"
	"q+/internal/generated/ent/examiner"
	"q+/internal/generated/ent/queue"
	"q+/internal/generated/ent/queueplace"
	"q+/internal/generated/ent/queueplacecriterion"
	"q+/internal/generated/ent/queuetemplate"
	"q+/internal/generated/ent/user"
	"q+/internal/generated/ent/useraccount"
	"strings"
)

type SearchCourseInstancesParams struct {
	ServerCommandParams
	Query string
}

func SearchCourseInstances(ctx UseCaseContext[SearchCourseInstancesParams]) ([]*ent.CourseInstance, error) {
	return ctx.ent().DiscordServer.
		Query().
		Where(discordserver.ID(ctx.Params.DiscordServerID)).
		QueryCourseInstances().
		Where(courseinstance.NameContainsFold(ctx.Params.Query)).
		Order(courseinstance.ByID()).
		All(ctx.Ctx)
}

type SearchQueueTemplatesParams struct {
	ServerCommandParams
	Query string
}

func SearchQueueTemplates(ctx UseCaseContext[SearchQueueTemplatesParams]) ([]*ent.QueueTemplate, error) {
	course, err := ctx.getCourseAt(ctx.Params.DiscordChannelId)
	if err != nil {
		return nil, err
	}

	return course.
		QueryQueueTemplates().
		Where(queuetemplate.NameContains(ctx.Params.Query)).
		Order(queuetemplate.ByID()).
		All(ctx.Ctx)
}

type SearchQueuesParams struct {
	ServerCommandParams
	Query string
}

func SearchQueues(ctx UseCaseContext[SearchQueuesParams]) ([]*ent.Queue, error) {
	course, err := ctx.getCourseAt(ctx.Params.DiscordChannelId)
	if err != nil {
		return nil, err
	}

	return course.
		QueryQueueTemplates().
		QueryQueues().
		Where(queue.NameContainsFold(ctx.Params.Query)).
		Order(queue.ByID()).
		All(ctx.Ctx)
}

func SearchSignUpStartedQueues(ctx UseCaseContext[SearchQueuesParams]) ([]*ent.Queue, error) {
	course, err := ctx.getCourseAt(ctx.Params.DiscordChannelId)
	if err != nil {
		return nil, err
	}

	return course.
		QueryQueueTemplates().
		QueryQueues().
		Where(
			queue.NameContainsFold(ctx.Params.Query),
			queue.SignUpStarted(true),
			queue.QueueEnded(false),
		).
		Order(queue.ByID()).
		All(ctx.Ctx)
}

func SearchNotEndedQueues(ctx UseCaseContext[SearchQueuesParams]) ([]*ent.Queue, error) {
	course, err := ctx.getCourseAt(ctx.Params.DiscordChannelId)
	if err != nil {
		return nil, err
	}

	return course.
		QueryQueueTemplates().
		QueryQueues().
		Where(
			queue.NameContainsFold(ctx.Params.Query),
			queue.QueueEnded(false),
		).
		Order(queue.ByStartTime(sql.OrderNullsFirst())).
		All(ctx.Ctx)
}

type SearchCurrentQueuePlaceCriteriaParams struct {
	ServerCommandParams
	Query   string
	Teacher *discord.User
}

func SearchCurrentQueuePlaceCriteria(ctx UseCaseContext[SearchCurrentQueuePlaceCriteriaParams]) ([]*ent.Criterion, error) {
	q, err := ctx.getActiveQueue(ctx.Params.DiscordChannelId)
	if err != nil {
		return nil, err
	}
	if q == nil {
		return nil, ErrActiveQueueNotFound
	}

	examUser, err := ctx.getUser(ctx.Params.Teacher)
	if err != nil {
		return nil, err
	}

	exam, err := ctx.ent().Examiner.
		Query().
		Where(
			examiner.TeacherID(examUser.ID),
			examiner.QueueID(q.ID),
		).
		WithCriteria().
		WithCurrentQueuePlace(func(q *ent.QueuePlaceQuery) {
			q.WithQueuePlaceCriteria()
		}).
		Only(ctx.Ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, ErrExaminerNotFound
		}
		return nil, err
	}

	if exam.Edges.CurrentQueuePlace == nil {
		return nil, ErrCurrentQueuePlaceNotFound
	}

	notPassedCriteriaIds := lo.FilterMap(exam.Edges.CurrentQueuePlace.Edges.QueuePlaceCriteria, func(c *ent.QueuePlaceCriterion, _ int) (int64, bool) {
		return c.CriterionID, !c.Passed
	})

	return intersectIds(exam.Edges.Criteria, notPassedCriteriaIds), nil
}

type SearchQueuePlacesForCurrentUserParams struct {
	ServerCommandParams
	User    *discord.User
	QueueId int64
	Query   string
}

func SearchQueuePlacesForCurrentUser(ctx UseCaseContext[SearchQueuePlacesForCurrentUserParams]) ([]*ent.QueuePlace, error) {
	q, err := ctx.ent().Queue.Get(ctx.Ctx, ctx.Params.QueueId)
	if ent.IsNotFound(err) {
		return nil, ErrQueueNotFound
	}
	if err != nil {
		return nil, err
	}

	return q.
		QueryPlaces().
		Where(
			queueplace.HasTeamWith(
				user.Or(
					user.DiscordID(ctx.Params.User.DiscordId),
					user.HasUserAccountsWith(
						useraccount.AccountIdentifier(ctx.Params.User.DiscordId),
						useraccount.TypeEQ(useraccount.TypeDiscord),
					),
				),
			),
			queueplace.Or(
				queueplace.HasTeamWith(user.NameContainsFold(ctx.Params.Query)),
				queueplace.HasCriteriaWith(criterion.NameContainsFold(ctx.Params.Query)),
			),
		).
		Order(queueplace.ByPosition()).
		WithTeam().
		WithQueuePlaceCriteria(func(q *ent.QueuePlaceCriterionQuery) {
			q.Where(queueplacecriterion.Passed(false))
			q.WithCriterion()
		}).
		All(ctx.Ctx)
}

type SearchQueuePlacesForExaminerParams struct {
	ServerCommandParams
	Teacher *discord.User
	Query   string
}

func SearchQueuePlacesForExaminer(ctx UseCaseContext[SearchQueuePlacesForExaminerParams]) ([]*ent.QueuePlace, error) {
	q, err := ctx.getActiveQueue(ctx.Params.DiscordChannelId)
	if err != nil {
		return nil, err
	}
	if q == nil {
		return nil, ErrActiveQueueNotFound
	}

	examUser, err := ctx.getUser(ctx.Params.Teacher)
	if err != nil {
		return nil, err
	}

	exam, err := ctx.ent().Examiner. // TODO seams to may be extracted to ctx.getExaminerForActiveQueue
						Query().
						Where(
			examiner.TeacherID(examUser.ID),
			examiner.QueueID(q.ID),
		).
		WithCriteria().
		Only(ctx.Ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, ErrExaminerNotFound
		}
		return nil, err
	}

	places, err := getPlacesForExaminer(ctx.Ctx, q, exam.Edges.Criteria)
	if err != nil {
		return nil, err
	}
	places = lo.Filter(places, func(p *ent.QueuePlace, _ int) bool {
		team := strings.Join(lo.Map(p.Edges.Team, func(u *ent.User, _ int) string {
			return GetName(u)
		}), ", ")
		criteria := strings.Join(lo.Map(p.Edges.QueuePlaceCriteria, func(c *ent.QueuePlaceCriterion, _ int) string {
			return c.Edges.Criterion.Name
		}), ", ")
		text := team + " " + criteria
		return strings.Contains(strings.ToLower(text), strings.ToLower(ctx.Params.Query))
	})
	return places, nil
}
