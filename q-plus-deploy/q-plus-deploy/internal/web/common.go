package web

import (
	"context"
	"errors"
	"fmt"
	errors2 "github.com/go-faster/errors"
	"github.com/go-faster/jx"
	"github.com/ogen-go/ogen/middleware"
	"github.com/ogen-go/ogen/ogenerrors"
	"github.com/rs/zerolog"
	"net/http"
	"q+/internal/core"
	"q+/internal/ent/rule"
	"q+/internal/generated/ent/privacy"
	"q+/internal/web/jwt"
	"strings"
)

// rawError renders err as json string.
func rawError(err error) jx.Raw {
	var e jx.Encoder
	e.Str(err.Error())
	return e.Bytes()
}

func useCaseContext[T any](ctx context.Context, h *MyOgentHandler, params T) core.UseCaseContext[T] {
	return core.UseCaseContext[T]{
		Ctx:    ctx,
		Core:   h.core,
		Params: params,
	}
}

// i hate this language
func useCaseContext_[T any](ctx context.Context, h *OauthHandler, params T) core.UseCaseContext[T] {
	return core.UseCaseContext[T]{
		Ctx:    ctx,
		Core:   h.core,
		Params: params,
	}
}

func EntPrivacyMiddleware(coder *jwt.Coder) func(r middleware.Request, next middleware.Next) (middleware.Response, error) {
	return func(r middleware.Request, next middleware.Next) (middleware.Response, error) {
		tokenString := r.Raw.Header.Get("Authorization")
		if tokenString == "" {
			return middleware.Response{}, &ogenerrors.SecurityError{
				OperationContext: ogenerrors.OperationContext{
					Name: r.OperationName,
					ID:   r.OperationID,
				},
				Security: "bearer",
				Err:      errors.New("no token"),
			}
		}
		tokenString = strings.TrimPrefix(tokenString, "Bearer ")
		courseId, err := coder.GetCourseId(tokenString)
		if err != nil {
			return middleware.Response{}, &ogenerrors.SecurityError{
				OperationContext: ogenerrors.OperationContext{
					Name: r.OperationName,
					ID:   r.OperationID,
				},
				Security: "bearer",
				Err:      err,
			}
		}
		ctx := context.WithValue(r.Context, rule.AllowedCourseIdCtxKey, courseId)
		(&r).SetContext(ctx)
		resp, err := next(r)
		if err != nil {
			logger := zerolog.Ctx(ctx)
			logger.Error().
				Str("event", "web_handler_error").
				Str("operation", r.OperationName).
				Str("id", r.OperationID).
				Err(err).
				Msg("error in web handler")
			if errors.Is(err, privacy.Deny) {
				return middleware.Response{}, &ForbiddenError{
					OperationContext: ogenerrors.OperationContext{
						Name: r.OperationName,
						ID:   r.OperationID,
					},
					Err: err,
				}
			}
		}
		return resp, err
	}
}

// ForbiddenError reports that error caused by security handler.
type ForbiddenError struct {
	ogenerrors.OperationContext
	Err error
}

func (e *ForbiddenError) OperationName() string {
	return e.Name
}

func (e *ForbiddenError) OperationID() string {
	return e.ID
}

func (e *ForbiddenError) FormatError(p errors2.Printer) (next error) {
	p.Printf("operation %s: forbidden", e.OperationName())
	return e.Err
}

func (e *ForbiddenError) Format(s fmt.State, verb rune) {
	errors2.FormatError(e, s, verb)
}

// Code returns http code to respond.
func (e *ForbiddenError) Code() int {
	return http.StatusForbidden
}

// Unwrap returns child error.
func (e *ForbiddenError) Unwrap() error {
	return e.Err
}

// Error implements error.
func (e *ForbiddenError) Error() string {
	return fmt.Sprintf("forbidden: %v", e.Err)
}
