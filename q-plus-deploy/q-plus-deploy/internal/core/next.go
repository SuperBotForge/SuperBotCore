package core

import (
	"context"
	"github.com/samber/lo"
	"q+/internal/core/discord"
	"q+/internal/generated/ent"
	"q+/internal/generated/ent/examiner"
	"q+/internal/generated/ent/queue"
	"q+/internal/generated/ent/queueplace"
	"q+/internal/generated/ent/queueplacecriterion"
	"q+/internal/generated/ent/user"
)

type NextParams struct {
	ServerCommandParams
	User *discord.User
}

type NextResponse struct {
	NextQueuePlace *ent.QueuePlace
	Criteria       []*ent.Criterion
	Examiner       *ent.Examiner
	BusyCount      int
	NotBusyCount   int
}

var Next = wrapTx(next)

func next(ctx UseCaseContext[NextParams]) (*NextResponse, error) {
	q, err := ctx.getActiveQueue(ctx.Params.DiscordChannelId)
	if err != nil {
		return nil, err
	}
	if q == nil {
		return nil, ErrActiveQueueNotFound
	}

	examUser, err := ctx.getUser(ctx.Params.User)
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
			q.WithTeam()
		}).
		Only(ctx.Ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, ErrExaminerNotFound
		}
		return nil, err
	}

	defer GoRedrawQueue(ctx, q.ID)

	placesForExaminer, err := getPlacesForExaminer(ctx.Ctx, q, exam.Edges.Criteria)
	if err != nil {
		return nil, err
	}
	notBusyPlaces := filterNotBusyPlaces(placesForExaminer)
	notBusyCount := len(notBusyPlaces)
	busyCount := len(placesForExaminer) - notBusyCount

	nextPlace := getNext(notBusyPlaces)

	if exam.Edges.CurrentQueuePlace != nil {
		err = unlockUsers(UseCaseContext[ServerCommandParams]{
			Ctx:    ctx.Ctx,
			Core:   ctx.Core,
			Params: ctx.Params.ServerCommandParams,
		}, q, exam)
		if err != nil {
			return nil, err
		}
		notBusyCount++
		busyCount--
	}

	if nextPlace == nil {
		return &NextResponse{
			NextQueuePlace: nil,
			Criteria:       nil,
			Examiner:       exam,
			BusyCount:      busyCount,
			NotBusyCount:   notBusyCount,
		}, nil
	}

	err = exam.Update().
		SetCurrentQueuePlace(nextPlace).
		Exec(ctx.Ctx)
	if err != nil {
		return nil, err
	}

	err = lockUsers(ctx, nextPlace)
	if err != nil {
		return nil, err
	}
	notBusyCount--
	busyCount++

	notPassedCriteria := lo.FilterMap(nextPlace.Edges.QueuePlaceCriteria, func(c *ent.QueuePlaceCriterion, _ int) (*ent.Criterion, bool) {
		return c.Edges.Criterion, !c.Passed
	})

	return &NextResponse{
		NextQueuePlace: nextPlace,
		Criteria:       intersect(exam.Edges.Criteria, notPassedCriteria),
		Examiner:       exam,
		BusyCount:      busyCount,
		NotBusyCount:   notBusyCount,
	}, nil
}

func getNext(notBusyPlaces []*ent.QueuePlace) *ent.QueuePlace {
	if len(notBusyPlaces) > 0 {
		return notBusyPlaces[0]
	}

	return nil
}

func getPlacesForExaminer(ctx context.Context, queue *ent.Queue, examCriteria []*ent.Criterion) ([]*ent.QueuePlace, error) {
	examCriteriaIds := lo.Map(examCriteria, func(c *ent.Criterion, _ int) int64 {
		return c.ID
	})
	return queue.
		QueryPlaces().
		Where(queueplace.HasQueuePlaceCriteriaWith(
			queueplacecriterion.Passed(false),
			queueplacecriterion.CriterionIDIn(examCriteriaIds...)),
		).
		Order(queueplace.ByPosition()).
		WithQueuePlaceCriteria(func(q *ent.QueuePlaceCriterionQuery) {
			q.WithCriterion()
		}).
		WithTeam().
		All(ctx)
}

func filterNotBusyPlaces(places []*ent.QueuePlace) []*ent.QueuePlace {
	return lo.Filter(places, func(place *ent.QueuePlace, _ int) bool {

		if !isAllStudentsAvailable(place.Edges.Team) {
			return false
		}

		return true
	})
}

func isAllStudentsAvailable(users []*ent.User) bool {
	for _, u := range users {
		if u.IsBusy {
			return false
		}
	}
	return true
}

func unlockUsers(ctx UseCaseContext[ServerCommandParams], q *ent.Queue, exam *ent.Examiner) error { // TODO check users to notify somehow another way, because now it don't handle case when student locks immediately
	queuePlace, err := exam.QueryCurrentQueuePlace().
		WithTeam().
		WithQueuePlaceCriteria().
		Only(ctx.Ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil
		}
		return err
	}

	affectedCriteria, err := queuePlace.
		QueryTeam().
		QueryQueuePlaces().
		Where(queueplace.HasQueueWith(queue.ID(q.ID))).
		QueryQueuePlaceCriteria().
		Where(queueplacecriterion.Passed(false)).
		All(ctx.Ctx)
	if err != nil {
		return err
	}

	notPassedCriteriaIds := lo.FilterMap(affectedCriteria, func(c *ent.QueuePlaceCriterion, _ int) (int64, bool) {
		return c.CriterionID, !c.Passed
	}) // TODO extract to utils
	idleExams, err := getIdleExams(ctx, q, notPassedCriteriaIds, []int64{exam.ID})
	if err != nil {
		return err
	}

	err = exam.Update().
		ClearCurrentQueuePlace().
		Exec(ctx.Ctx)
	if err != nil {
		return err
	}

	err = ctx.ent().User.
		Update().
		Where(user.HasQueuePlacesWith(queueplace.ID(queuePlace.ID))).
		SetIsBusy(false).
		Exec(ctx.Ctx)
	if err != nil {
		return err
	}

	if len(idleExams) > 0 {
		channels, err := ctx.getChannelsForCourse(ctx.Params.DiscordChannelId)
		if err != nil {
			return err
		}

		users := lo.Map(idleExams, func(e *ent.Examiner, _ int) *ent.User {
			return e.Edges.Teacher
		}) // TODO extract to utils
		err = ctx.Core.discordSender.SendNewStudentInQueueNotify(ctx.Ctx, users, queuePlace.Edges.Team, channels)
		if err != nil {
			return err
		}
	}
	return nil
}

func lockUsers[T any](ctx UseCaseContext[T], queuePlace *ent.QueuePlace) error {
	return ctx.ent().User.
		Update().
		Where(user.HasQueuePlacesWith(queueplace.ID(queuePlace.ID))).
		SetIsBusy(true).
		Exec(ctx.Ctx)
}

func intersect(a []*ent.Criterion, b []*ent.Criterion) []*ent.Criterion {
	set := make([]*ent.Criterion, 0)
	hash := make(map[int64]bool)

	for _, v := range a {
		hash[v.ID] = true
	}

	for _, v := range b {
		if hash[v.ID] {
			set = append(set, v)
		}
	}

	return set
}

func intersectIds(a []*ent.Criterion, b []int64) []*ent.Criterion {
	set := make([]*ent.Criterion, 0)
	hash := make(map[int64]bool)

	for _, v := range b {
		hash[v] = true
	}

	for _, v := range a {
		if hash[v.ID] {
			set = append(set, v)
		}
	}

	return set
}
