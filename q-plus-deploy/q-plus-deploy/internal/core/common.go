package core

import (
	"context"
	"fmt"
	"github.com/rs/zerolog"
	"github.com/samber/do/v2"
	"q+/internal/core/discord"
	"q+/internal/core/presenter"
	"q+/internal/generated/ent"
	"q+/internal/generated/ent/channelsforcourse"
	"q+/internal/generated/ent/courseinstance"
	"q+/internal/generated/ent/queue"
	"q+/internal/generated/ent/user"
	"q+/internal/generated/ent/useraccount"
)

type Core struct {
	client                 *ent.Client
	originalClient         *ent.Client
	sheetsService          presenter.SheetsService
	discordSender          discord.Sender
	discordMentionRenderer discord.MentionRenderer
}

func NewCore(i do.Injector) (*Core, error) {
	client := do.MustInvoke[*ent.Client](i)
	sheetsService := do.MustInvokeAs[presenter.SheetsService](i)
	discordSender := do.MustInvokeAs[discord.Sender](i)
	discordMentionRenderer := do.MustInvokeAs[discord.MentionRenderer](i)

	return &Core{
		client:                 client,
		originalClient:         client,
		sheetsService:          sheetsService,
		discordSender:          discordSender,
		discordMentionRenderer: discordMentionRenderer,
	}, nil
}

func (c *Core) withClient(client *ent.Client) *Core {
	newCore := *c
	newCore.client = client
	return &newCore
}

type ServerCommandParams struct {
	DiscordServerID  string
	DiscordChannelId string
}

type UseCaseContext[T any] struct {
	Ctx    context.Context
	Core   *Core
	Params T
}

func (ctx UseCaseContext[T]) log() *zerolog.Logger {
	return zerolog.Ctx(ctx.Ctx)
}

func (ctx UseCaseContext[T]) ent() *ent.Client {
	return ctx.Core.client
}

func withTx(ctx context.Context, client *ent.Client, fn func(client *ent.Client) error) error {
	tx, err := client.Tx(ctx)
	if err != nil {
		return err
	}
	defer func() {
		if v := recover(); v != nil {
			_ = tx.Rollback()
			panic(v)
		}
	}()
	if err = fn(tx.Client()); err != nil {
		if rerr := tx.Rollback(); rerr != nil {
			err = fmt.Errorf("%w: rolling back transaction: %v", err, rerr)
		}
		return err
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}
	return nil
}

func wrapTx[TInput any, TOutput any](fn func(ctx UseCaseContext[TInput]) (TOutput, error)) func(ctx UseCaseContext[TInput]) (TOutput, error) {
	return func(ctx UseCaseContext[TInput]) (TOutput, error) {
		var output TOutput
		err := withTx(ctx.Ctx, ctx.Core.client, func(txClient *ent.Client) error {
			txCtx := ctx
			txCtx.Core = txCtx.Core.withClient(txClient)
			var err error
			output, err = fn(txCtx)
			return err
		})
		return output, err
	}
}

func wrapTx0[TInput any](fn func(ctx UseCaseContext[TInput]) error) func(ctx UseCaseContext[TInput]) error {
	return func(ctx UseCaseContext[TInput]) error {
		_, err := wrapTx(func(ctx UseCaseContext[TInput]) (any, error) {
			err := fn(ctx)
			return nil, err
		})(ctx)
		return err
	}
}

func useCaseContext[T any, T2 any](ctx UseCaseContext[T], params T2) UseCaseContext[T2] {
	return UseCaseContext[T2]{
		Ctx:    ctx.Ctx,
		Core:   ctx.Core,
		Params: params,
	}
}

func (ctx UseCaseContext[T]) createDiscordServer(discordServerId string) error {
	return ctx.ent().DiscordServer.
		Create().
		SetID(discordServerId).
		OnConflictColumns(courseinstance.FieldID).
		Ignore().
		Exec(ctx.Ctx)
}

func (ctx UseCaseContext[T]) getUser(u *discord.User) (*ent.User, error) {
	needRegistration := false

	if needRegistration {
		userId, err := ctx.ent().User.
			Query().
			Where(user.HasUserAccountsWith(
				useraccount.TypeEQ(useraccount.TypeDiscord),
				useraccount.AccountIdentifier(u.DiscordId),
			)).
			Only(ctx.Ctx)
		if ent.IsNotFound(err) {
			return nil, ErrUserNotRegistered(ctx.Core.discordMentionRenderer)
		}
		if err != nil {
			return nil, err
		}
		return userId, nil
	} else {
		userId, err := ctx.ent().User.
			Create().
			SetDiscordID(u.DiscordId).
			SetName(u.Username).
			SetSurname("").
			SetPatronymic("").
			SetGroup("").
			OnConflictColumns(user.FieldDiscordID).
			UpdateName().
			ID(ctx.Ctx)
		if err != nil {
			return nil, err
		}
		return ctx.ent().User.Get(ctx.Ctx, userId)
	}
}

func (ctx UseCaseContext[T]) getUserById(discordId string) (*ent.User, error) {
	userId, err := ctx.ent().User.
		Query().
		Where(user.HasUserAccountsWith(
			useraccount.TypeEQ(useraccount.TypeDiscord),
			useraccount.AccountIdentifier(discordId),
		)).
		Only(ctx.Ctx)
	if ent.IsNotFound(err) {
		return nil, ErrUserNotRegistered(ctx.Core.discordMentionRenderer)
	}
	if err != nil {
		return nil, err
	}
	return userId, nil
}

func (ctx UseCaseContext[T]) getChannelsForCourse(channelId string) (*ent.ChannelsForCourse, error) {
	channelsForCourse, err := ctx.ent().ChannelsForCourse.
		Query().
		Where(channelsforcourse.Or(
			channelsforcourse.QueueChannelID(channelId),
			channelsforcourse.StudentChannelID(channelId),
			channelsforcourse.TeacherChannelID(channelId),
		)).
		First(ctx.Ctx)
	if ent.IsNotFound(err) {
		return nil, ErrNotInCourseChannels
	}
	if err != nil {
		return nil, err
	}
	return channelsForCourse, nil
}

func (ctx UseCaseContext[T]) getCourseAt(channelId string) (*ent.CourseInstance, error) {
	channelsForCourse, err := ctx.getChannelsForCourse(channelId)
	if err != nil {
		return nil, err
	}

	course, err := channelsForCourse.QueryCourseInstance().
		First(ctx.Ctx)
	if ent.IsNotFound(err) {
		return nil, ErrCourseInstanceNotFound(ctx.Core.discordMentionRenderer)
	}
	if err != nil {
		return nil, err
	}
	return course, nil
}

func (ctx UseCaseContext[T]) getActiveQueue(channelId string) (*ent.Queue, error) {
	course, err := ctx.getCourseAt(channelId)
	if err != nil {
		return nil, err
	}
	q, err := course.
		QueryQueueTemplates().
		QueryQueues().
		Where(
			queue.QueueStarted(true),
			queue.QueueEnded(false),
		).
		First(ctx.Ctx)
	if err != nil {
		if !ent.IsNotFound(err) {
			return nil, err
		}
		return nil, nil
	}
	return q, nil
}
