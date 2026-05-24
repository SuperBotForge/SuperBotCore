package cron

import (
	"context"
	"fmt"
	"github.com/reugn/go-quartz/job"
	"github.com/rs/zerolog"
	"q+/internal/core"
	"q+/internal/ent/rule"
	"runtime/debug"
)

type Context struct {
	ctx  context.Context
	core *core.Core
}

type Function[R any] func(ctx Context) (R, error)

func useCaseContext[T any](ctx Context, params T) core.UseCaseContext[T] {
	return core.UseCaseContext[T]{
		Ctx:    ctx.ctx,
		Core:   ctx.core,
		Params: params,
	}
}

func newFunctionJob[R any](core *core.Core, function Function[R]) job.Function[R] {
	return func(ctx context.Context) (R, error) {
		defer func() {
			if r := recover(); r != nil {
				logger := zerolog.Ctx(ctx)
				logger.Error().
					Str("event", "panic_handling_cron_job").
					Str("error", fmt.Sprintf("%v", r)).
					Str("stacktrace", string(debug.Stack())).
					Msg("Panic handling cron job")
			}
		}()
		ctx = context.WithValue(ctx, rule.CronRuleCtxKey, true)
		return function(Context{
			ctx:  ctx,
			core: core,
		})
	}
}
