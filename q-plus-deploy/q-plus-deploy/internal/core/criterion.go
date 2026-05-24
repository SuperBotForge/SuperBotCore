package core

import (
	"github.com/samber/lo"
	"q+/internal/generated/ent"
	"q+/internal/generated/ent/courseinstance"
	"q+/internal/generated/ent/criterion"
	"strconv"
	"strings"
)

type GetCourseAtChannelParams struct {
	ServerCommandParams
}

var GetCourseWithCriteriaAtChannel = wrapTx(getCourseWithCriteriaAtChannel)

func getCourseWithCriteriaAtChannel(ctx UseCaseContext[GetCourseAtChannelParams]) (*ent.CourseInstance, error) {
	course, err := ctx.getCourseAt(ctx.Params.DiscordChannelId)
	if err != nil {
		return nil, err
	}
	criteria, err := course.QueryCriteria().Order(criterion.ByID()).All(ctx.Ctx)
	if err != nil {
		return nil, err
	}
	course.Edges.Criteria = criteria
	return course, nil
}

type GetCourseParams struct {
	CourseId int64
}

var GetCourse = wrapTx(getCourse)

func getCourse(ctx UseCaseContext[GetCourseParams]) (*ent.CourseInstance, error) {
	return ctx.ent().CourseInstance.
		Query().
		Where(courseinstance.ID(ctx.Params.CourseId)).
		WithCriteria(func(q *ent.CriterionQuery) {
			q.Order(criterion.ByID())
		}).
		Only(ctx.Ctx)
}

type EditCriteriaParams struct {
	CourseId     int64
	CriteriaList string
}

type EditCriteriaResponse struct {
	Course  *ent.CourseInstance
	Created []string
	Updated []lo.Tuple2[string, string]
	Deleted []string
}

var EditCriteriaFromTextList = wrapTx(editCriteriaFromTextList)

func editCriteriaFromTextList(ctx UseCaseContext[EditCriteriaParams]) (*EditCriteriaResponse, error) {
	course, err := ctx.ent().CourseInstance.
		Query().
		Where(courseinstance.ID(ctx.Params.CourseId)).
		WithCriteria(func(q *ent.CriterionQuery) {
			q.Order(criterion.ByID())
		}).
		Only(ctx.Ctx)
	if err != nil {
		return nil, err
	}

	type shortCriterion struct {
		id     int64
		number int
		name   string
	}

	oldCriteria := lo.Map(course.Edges.Criteria, func(criterion *ent.Criterion, i int) *shortCriterion {
		return &shortCriterion{id: criterion.ID, number: i + 1, name: criterion.Name}
	})

	criteriaList := strings.Split(strings.TrimSpace(ctx.Params.CriteriaList), "\n")

	newCriteria := lo.FilterMap(criteriaList, func(s string, i int) (*shortCriterion, bool) {
		s = strings.TrimSpace(s)
		before, after, found := strings.Cut(s, ".")
		before = strings.TrimSpace(before)
		after = strings.TrimSpace(after)
		if !found && len(before) == 0 {
			return nil, false
		} else if !found { // no dot in row
			return &shortCriterion{number: 0, name: before}, true
		} else if len(after) == 0 {
			return nil, false
		}
		criterionNumber, err := strconv.Atoi(before)
		if err != nil {
			return &shortCriterion{number: 0, name: s}, true
		}
		return &shortCriterion{number: criterionNumber, name: after}, true
	})

	// find diff between old and new criteria, form create, update and delete lists
	createList := lo.FilterMap(newCriteria, func(newCriterion *shortCriterion, i int) (*shortCriterion, bool) {
		for _, oldCriterion := range oldCriteria {
			if oldCriterion.number == newCriterion.number {
				return nil, false
			}
		}
		return newCriterion, true
	})
	updateList := lo.FilterMap(newCriteria, func(newCriterion *shortCriterion, i int) (lo.Tuple2[*shortCriterion, *shortCriterion], bool) {
		for _, oldCriterion := range oldCriteria {
			if oldCriterion.number == newCriterion.number && oldCriterion.name != newCriterion.name {
				return lo.T2(oldCriterion, newCriterion), true
			}
		}
		return lo.T2[*shortCriterion, *shortCriterion](nil, nil), false
	})
	deleteList := lo.FilterMap(oldCriteria, func(oldCriterion *shortCriterion, i int) (*shortCriterion, bool) {
		for _, newCriterion := range newCriteria {
			if oldCriterion.number == newCriterion.number {
				return nil, false
			}
		}
		return oldCriterion, true
	})

	// delete old criteria
	deleteCount, err := ctx.ent().Criterion.
		Delete().
		Where(criterion.IDIn(lo.Map(deleteList, func(criterion *shortCriterion, i int) int64 {
			return criterion.id
		})...)).
		Exec(ctx.Ctx)
	if err != nil {
		return nil, err
	}
	if deleteCount != len(deleteList) {
		//return nil, ErrNotAllCriteriaDeleted
	}

	// update existing criteria
	err = ctx.ent().Criterion.MapCreateBulk(updateList, func(c *ent.CriterionCreate, i int) {
		c.SetID(updateList[i].A.id).SetName(updateList[i].B.name).SetCourseInstanceID(course.ID)
	}).
		OnConflictColumns(criterion.FieldID).
		UpdateNewValues().
		Exec(ctx.Ctx)
	if err != nil {
		return nil, err
	}
	// TODO swap criteria (detect swaps and warn user?)

	// create new criteria
	err = ctx.ent().Criterion.MapCreateBulk(createList, func(c *ent.CriterionCreate, i int) {
		c.SetName(createList[i].name).SetCourseInstanceID(course.ID)
	}).
		Exec(ctx.Ctx)
	if err != nil {
		return nil, err
	}

	course, err = ctx.ent().CourseInstance.
		Query().
		Where(courseinstance.ID(ctx.Params.CourseId)).
		WithCriteria(func(q *ent.CriterionQuery) {
			q.Order(criterion.ByID())
		}).
		Only(ctx.Ctx)
	if err != nil {
		return nil, err
	}

	return &EditCriteriaResponse{
		Course: course,
		Created: lo.Map(createList, func(criterion *shortCriterion, i int) string {
			return criterion.name
		}),
		Updated: lo.Map(updateList, func(criterion lo.Tuple2[*shortCriterion, *shortCriterion], i int) lo.Tuple2[string, string] {
			return lo.T2(criterion.A.name, criterion.B.name)
		}),
		Deleted: lo.Map(deleteList, func(criterion *shortCriterion, i int) string {
			return criterion.name
		}),
	}, nil
}
