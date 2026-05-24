package core

import (
	"errors"
	"fmt"
	"q+/internal/core/discord"
	"q+/internal/generated/ent"
)

type HumanReadableError struct {
	Err       error
	UserError string
}

func (e *HumanReadableError) Error() string {
	return fmt.Sprintf("error: %v", e.Err)
}

var (
	ErrBase       = errors.New("base error")
	ErrBadRequest = fmt.Errorf("bad request: %w", ErrBase)
	ErrNotFound   = fmt.Errorf("not found: %w", ErrBase)
	ErrForbidden  = fmt.Errorf("forbidden: %w", ErrBase)
)

var (
	ErrUserNotRegistered = func(renderer discord.MentionRenderer) error {
		return &HumanReadableError{
			Err: fmt.Errorf("user not registered: %w", ErrBadRequest),
			UserError: "❗ Для использования бота необходимо зарегистрироваться c помощью команды " +
				renderer.ClickableSlashCommand("register"),
		}
	}

	ErrNameAlreadyExistsOnServer = fmt.Errorf("name already exists on server: %w", ErrBadRequest)
	ErrNotInCourseChannels       = &HumanReadableError{
		UserError: "❗ Команда выполнена не в каналах бота",
		Err:       fmt.Errorf("command executed not in course channels: %w", ErrBadRequest),
	}
	ErrChannelAlreadyExistsOnServer = func(cause *ent.ConstraintError) error {
		return fmt.Errorf("channel already exists on server: %w: %w", cause, ErrBadRequest)
	}
	ErrCourseInstanceNotFound = func(renderer discord.MentionRenderer) error {
		return &HumanReadableError{
			UserError: fmt.Sprintf("❗ Предмет не найден (создать можно командой %s)", renderer.ClickableSlashCommand("course create")),
			Err:       fmt.Errorf("course instance not found: %w", ErrNotFound),
		}
	}
)

var (
	ErrExaminerNotFound = &HumanReadableError{
		UserError: "❗ Принимающий не найден, возможно вы не записаны в очередь как принимающий",
		Err:       fmt.Errorf("examiner not found: %w", ErrNotFound),
	}
	ErrActiveQueueNotFound = &HumanReadableError{
		UserError: "❗ Нет активной очереди, начните очередь командой /start-queue",
		Err:       fmt.Errorf("active queue not found: %w", ErrBadRequest),
	}
	ErrCurrentQueuePlaceNotFound = &HumanReadableError{
		UserError: "❗ У вас нет текущей принимаемой команды",
		Err:       fmt.Errorf("current queue place not found: %w", ErrNotFound),
	}
	ErrQueuePlaceNotFound = &HumanReadableError{
		UserError: "❗ Запись в очередь не найдена",
		Err:       fmt.Errorf("queue place not found: %w", ErrNotFound),
	}
	ErrQueueTemplateNotFound = &HumanReadableError{
		UserError: "❗ Шаблон очереди не найден",
		Err:       fmt.Errorf("queue template not found: %w", ErrNotFound),
	}
	ErrQueueNotFound = &HumanReadableError{
		UserError: "❗ Очередь не найдена",
		Err:       fmt.Errorf("queue not found: %w", ErrNotFound),
	}
)

var (
	ErrUserNotInTeam = &HumanReadableError{
		UserError: "❗ Вы не состоите в этой команде",
		Err:       fmt.Errorf("user not in team: %w", ErrForbidden),
	}
)
