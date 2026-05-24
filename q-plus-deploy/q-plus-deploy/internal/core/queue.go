package core

import (
	"github.com/samber/lo"
	"q+/internal/core/discord"
	"q+/internal/generated/ent"
	"q+/internal/generated/ent/examiner"
	"q+/internal/generated/ent/queue"
	"q+/internal/generated/ent/queueplace"
	"q+/internal/generated/ent/queueplacecriterion"
	"q+/internal/generated/ent/queuetemplate"
	"strings"
	"time"
)

type ScheduleQueueItemParams struct {
	QueueTemplateId int64
	StartTime       time.Time
	EndTime         time.Time
}

type ScheduleQueuesParams struct {
	Queues []ScheduleQueueItemParams
}

var ScheduleQueues = wrapTx(scheduleQueues)

func scheduleQueues(ctx UseCaseContext[ScheduleQueuesParams]) ([]*ent.Queue, error) {
	var queues []*ent.Queue

	templateCache := make(map[int64]*ent.QueueTemplate)

	for _, item := range ctx.Params.Queues {
		var err error

		template, ok := templateCache[item.QueueTemplateId]
		if !ok {
			template, err = ctx.ent().QueueTemplate.
				Query().
				Where(queuetemplate.ID(item.QueueTemplateId)).
				WithMarkTableTab().
				WithCriteria().
				Only(ctx.Ctx)
			if err != nil {
				return nil, err
			}
			templateCache[item.QueueTemplateId] = template
		}

		q, err := ctx.ent().Queue. // TODO batch insert
						Create().
						SetQueueTemplateID(item.QueueTemplateId).
						SetName(template.Name + " " + item.StartTime.Format("02.01.2006")).
						SetSignUpLeadTime(template.SignUpLeadTime).
						SetMarkTableTab(template.Edges.MarkTableTab).
						AddCriteria(template.Edges.Criteria...).
						SetStartTime(item.StartTime).
						SetEndTime(item.EndTime).
						Save(ctx.Ctx)
		if err != nil {
			return nil, err
		}
		queues = append(queues, q)
	}
	return queues, nil
}

type SetTeacherParams struct {
	ServerCommandParams
	QueueId int64
	Teacher *discord.User
	Note    *string
}

type SetTeacherResponse struct {
	Examiner *ent.Examiner
	Criteria []*ent.Criterion
}

var SetTeacher = wrapTx(setTeacher) // TODO rename to Examiner?

func setTeacher(ctx UseCaseContext[SetTeacherParams]) (*SetTeacherResponse, error) {
	user, err := ctx.getUser(ctx.Params.Teacher)
	if err != nil {
		return nil, err
	}

	exam, err := ctx.ent().Examiner.
		Query().
		Where(
			examiner.TeacherID(user.ID),
			examiner.QueueID(ctx.Params.QueueId),
		).
		Only(ctx.Ctx)
	if err != nil {
		if !ent.IsNotFound(err) {
			return nil, err
		}

		examinerCreate := ctx.ent().Examiner.
			Create().
			SetTeacher(user).
			SetQueueID(ctx.Params.QueueId)

		if ctx.Params.Note != nil {
			note := strings.TrimSpace(*ctx.Params.Note)
			if note == "-" {
				note = ""
			}
			examinerCreate.SetNote(note)
		}

		exam, err = examinerCreate.Save(ctx.Ctx)
		if err != nil {
			return nil, err
		}
	} else {
		if ctx.Params.Note != nil {
			note := strings.TrimSpace(*ctx.Params.Note)
			if note == "-" {
				note = ""
			}
			exam, err = exam.Update().
				SetNote(note).
				Save(ctx.Ctx)
			if err != nil {
				return nil, err
			}
		}
	}
	exam.Edges.Teacher = user
	exam.Edges.Queue, err = exam.QueryQueue().Only(ctx.Ctx)
	if err != nil {
		return nil, err
	}

	criteria, err := ctx.ent().Queue.
		Query().
		Where(queue.ID(ctx.Params.QueueId)).
		QueryCriteria().
		All(ctx.Ctx)
	if err != nil {
		return nil, err
	}

	return &SetTeacherResponse{
		Examiner: exam,
		Criteria: criteria,
	}, nil
}

type DeleteTeacherParams struct {
	ServerCommandParams
	QueueId int64
	Teacher *discord.User
}

type DeleteTeacherResponse struct {
	Examiner *ent.User
	Queue    *ent.Queue
}

var DeleteTeacher = wrapTx(deleteTeacher)

func deleteTeacher(ctx UseCaseContext[DeleteTeacherParams]) (*DeleteTeacherResponse, error) {
	user, err := ctx.getUser(ctx.Params.Teacher)
	if err != nil {
		return nil, err
	}

	exam, err := ctx.ent().Examiner.
		Query().
		Where(
			examiner.TeacherID(user.ID),
			examiner.QueueID(ctx.Params.QueueId),
		).
		WithQueue().
		WithTeacher().
		WithCurrentQueuePlace().
		Only(ctx.Ctx)
	if ent.IsNotFound(err) {
		return nil, ErrExaminerNotFound
	}
	if err != nil {
		return nil, err
	}

	if exam.Edges.CurrentQueuePlace != nil {
		err = unlockUsers(UseCaseContext[ServerCommandParams]{
			Ctx:    ctx.Ctx,
			Core:   ctx.Core,
			Params: ctx.Params.ServerCommandParams,
		}, exam.Edges.Queue, exam)
		if err != nil {
			return nil, err
		}
	}

	err = ctx.ent().Examiner.
		DeleteOne(exam).
		Exec(ctx.Ctx)

	GoRedrawQueue(ctx, exam.Edges.Queue.ID)

	return &DeleteTeacherResponse{
		Examiner: exam.Edges.Teacher,
		Queue:    exam.Edges.Queue,
	}, nil
}

type StartSignUpParams struct {
	ServerCommandParams
	QueueId int64
}

var StartSignUp = wrapTx(startSignUp)

func startSignUp(ctx UseCaseContext[StartSignUpParams]) (*ent.Queue, error) {
	q, err := ctx.ent().Queue.
		Get(ctx.Ctx, ctx.Params.QueueId)
	if err != nil {
		return nil, err
	}
	err = _startSignUp(ctx.Ctx, ctx.Core, q)
	if err != nil {
		return nil, err
	}
	return q, nil
}

type StartQueueParams struct {
	ServerCommandParams
	QueueId int64
}

var StartQueue = wrapTx(startQueue)

func startQueue(ctx UseCaseContext[StartQueueParams]) (*ent.Queue, error) {
	q, err := endQueue(useCaseContext(ctx, EndQueueParams{
		ServerCommandParams: ctx.Params.ServerCommandParams,
	}))
	if err != nil {
		return nil, err
	}

	q, err = ctx.ent().Queue.
		Get(ctx.Ctx, ctx.Params.QueueId)
	if err != nil {
		return nil, err
	}
	err = _startQueue(ctx.Ctx, ctx.Core, q)
	if err != nil {
		return nil, err
	}
	return q, nil
}

type EndQueueParams struct {
	ServerCommandParams
}

var EndQueue = wrapTx(endQueue)

func endQueue(ctx UseCaseContext[EndQueueParams]) (*ent.Queue, error) {
	q, err := ctx.getActiveQueue(ctx.Params.DiscordChannelId)
	if err != nil {
		return nil, err
	}
	if q == nil {
		return nil, nil
	}

	return q.Update().
		SetQueueEnded(true).
		Save(ctx.Ctx)
}

type SignUpParams struct {
	ServerCommandParams
	QueueId int64
	Team    []*discord.User
	Note    string
}

type SignUpResponse struct {
	QueuePlace *ent.QueuePlace
	Criteria   []*ent.Criterion
}

var SignUp = wrapTx(signUp)

func signUp(ctx UseCaseContext[SignUpParams]) (*SignUpResponse, error) {
	users := make([]*ent.User, 0, len(ctx.Params.Team))
	for _, dUser := range ctx.Params.Team {
		user, err := ctx.getUser(dUser)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}

	// check sign up started

	var maxPosition int
	queuePlacesExist, err := ctx.ent().Queue.
		Query().
		Where(queue.ID(ctx.Params.QueueId)).
		QueryPlaces().
		Exist(ctx.Ctx)
	if queuePlacesExist {
		maxPosition, err = ctx.ent().
			QueuePlace. // TODO race condition?
			Query().
			Aggregate(
				ent.Max(queueplace.FieldPosition),
			).
			Int(ctx.Ctx)
		if err != nil {
			return nil, err
		}
	} else {
		maxPosition = 0
	}

	queuePlace, err := ctx.ent().QueuePlace.
		Create().
		SetQueueID(ctx.Params.QueueId).
		AddTeam(users...).
		SetPosition(maxPosition + 1).
		SetNote(ctx.Params.Note).
		Save(ctx.Ctx)
	if err != nil {
		return nil, err
	}

	queuePlace.Edges.Queue, err = queuePlace.QueryQueue().WithExaminers().Only(ctx.Ctx)
	if err != nil {
		return nil, err
	}
	queuePlace.Edges.Team, err = queuePlace.QueryTeam().All(ctx.Ctx)
	if err != nil {
		return nil, err
	}

	criteria, err := ctx.ent().Queue.
		Query().
		Where(queue.ID(ctx.Params.QueueId)).
		QueryCriteria().
		All(ctx.Ctx)
	if err != nil {
		return nil, err
	}

	return &SignUpResponse{
		QueuePlace: queuePlace,
		Criteria:   criteria,
	}, nil
}

type LeaveParams struct {
	ServerCommandParams
	User         *discord.User
	QueuePlaceId int64
}

var Leave = wrapTx(leave)

func leave(ctx UseCaseContext[LeaveParams]) (*ent.QueuePlace, error) {
	queuePlace, err := ctx.ent().QueuePlace.
		Query().
		Where(queueplace.ID(ctx.Params.QueuePlaceId)).
		WithTeam().
		WithQueuePlaceCriteria(func(q *ent.QueuePlaceCriterionQuery) {
			q.Where(queueplacecriterion.Passed(false))
			q.WithCriterion()
		}).
		WithQueue().
		Only(ctx.Ctx)
	if err != nil {
		return nil, err
	}
	if !lo.ContainsBy(queuePlace.Edges.Team, func(u *ent.User) bool {
		return u.DiscordID == ctx.Params.User.DiscordId
	}) {
		return nil, ErrUserNotInTeam
	}
	err = ctx.ent().QueuePlace.
		DeleteOne(queuePlace).
		Exec(ctx.Ctx)
	if err != nil {
		return nil, err
	}

	GoRedrawQueue(ctx, queuePlace.Edges.Queue.ID)

	return queuePlace, nil
}

type PauseParams struct {
	ServerCommandParams
	User *discord.User
}

var Pause = wrapTx(pause)

func pause(ctx UseCaseContext[PauseParams]) (*ent.Examiner, error) {
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
		WithCurrentQueuePlace().
		WithTeacher().
		Only(ctx.Ctx)
	if err != nil {
		return nil, err
	}

	if exam.Edges.CurrentQueuePlace != nil {
		err = unlockUsers(UseCaseContext[ServerCommandParams]{
			Ctx:    ctx.Ctx,
			Core:   ctx.Core,
			Params: ctx.Params.ServerCommandParams,
		}, q, exam)
		if err != nil {
			return nil, err
		}
	}

	GoRedrawQueue(ctx, q.ID)

	return exam, nil
}

type RepingParams struct {
	ServerCommandParams
	User *discord.User
}

type RepingResponse struct {
	NextQueuePlace *ent.QueuePlace
	Criteria       []*ent.Criterion
	Examiner       *ent.Examiner
}

var Reping = wrapTx(reping)

func reping(ctx UseCaseContext[RepingParams]) (*RepingResponse, error) {

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
			q.WithTeam().
				WithQueuePlaceCriteria(func(q *ent.QueuePlaceCriterionQuery) {
					q.WithCriterion()
				})
		}).
		Only(ctx.Ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, ErrExaminerNotFound
		}
		return nil, err
	}

	nextPlace := exam.Edges.CurrentQueuePlace

	if nextPlace == nil {
		return &RepingResponse{
			NextQueuePlace: nil,
			Criteria:       nil,
			Examiner:       exam,
		}, nil
	}

	notPassedCriteria := lo.FilterMap(nextPlace.Edges.QueuePlaceCriteria, func(c *ent.QueuePlaceCriterion, _ int) (*ent.Criterion, bool) {
		return c.Edges.Criterion, !c.Passed
	})

	return &RepingResponse{
		NextQueuePlace: nextPlace,
		Criteria:       intersect(exam.Edges.Criteria, notPassedCriteria),
		Examiner:       exam,
	}, nil
}
