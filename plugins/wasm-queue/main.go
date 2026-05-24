package main

import (
	"embed"
	"errors"
	"fmt"
	"strings"

	wasmplugin "github.com/SuperBotForge/sdk/go-sdk"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

func main() {
	wasmplugin.Run(wasmplugin.Plugin{
		ID:      "queue",
		Name:    "Queue Manager",
		Version: "1.0.0",
		Requirements: []wasmplugin.Requirement{
			wasmplugin.Database("Store queues and members").Build(),
		},
		Migrations: wasmplugin.MigrationsFromFS(migrationsFS, "migrations"),
		Triggers: []wasmplugin.Trigger{
			{
				Name:        "queue_new",
				Type:        wasmplugin.TriggerMessenger,
				Description: "Create a new queue",
				Nodes: []wasmplugin.Node{
					wasmplugin.NewStep("name").
						LocalizedText(map[string]string{
							"en": "Enter queue name:",
							"ru": "Введите название очереди:",
						}, wasmplugin.StylePlain),
				},
				Handler: handleNew,
			},
			{
				Name:        "queue_join",
				Type:        wasmplugin.TriggerMessenger,
				Description: "Join a queue",
				Nodes: []wasmplugin.Node{
					wasmplugin.NewStep("name").
						LocalizedText(map[string]string{
							"en": "Enter queue name (or leave blank if only one queue is active):",
							"ru": "Введите название очереди (или оставьте пустым, если очередь одна):",
						}, wasmplugin.StylePlain),
				},
				Handler: handleJoin,
			},
			{
				Name:        "queue_leave",
				Type:        wasmplugin.TriggerMessenger,
				Description: "Leave the queue",
				Handler:     handleLeave,
			},
			{
				Name:        "queue_next",
				Type:        wasmplugin.TriggerMessenger,
				Description: "Call the next person (queue creator only)",
				Handler:     handleNext,
			},
			{
				Name:        "queue_status",
				Type:        wasmplugin.TriggerMessenger,
				Description: "Show all active queues",
				Handler:     handleStatus,
			},
			{
				Name:        "queue_pos",
				Type:        wasmplugin.TriggerMessenger,
				Description: "Show your position in the queue",
				Handler:     handlePos,
			},
			{
				Name:        "queue_close",
				Type:        wasmplugin.TriggerMessenger,
				Description: "Close your queue (creator only)",
				Handler:     handleClose,
			},
		},
	})
}

func handleNew(ctx *wasmplugin.EventContext) error {
	name := strings.TrimSpace(ctx.Param("name"))
	if name == "" {
		ctx.Reply(wasmplugin.NewMessage(tr(ctx, "name_empty")))
		return nil
	}

	db, err := openDB()
	if err != nil {
		ctx.LogError("queue_new: db: " + err.Error())
		ctx.Reply(wasmplugin.NewMessage(tr(ctx, "error")))
		return nil
	}
	defer db.Close()

	_, err = dbCreateQueue(db, ctx.Messenger.ChatID, name, ctx.Messenger.UserID)
	if err != nil {
		if errors.Is(err, errQueueNameTaken) {
			ctx.Reply(wasmplugin.NewMessage(fmt.Sprintf(tr(ctx, "name_taken"), name)))
			return nil
		}
		ctx.LogError("queue_new: create: " + err.Error())
		ctx.Reply(wasmplugin.NewMessage(tr(ctx, "error")))
		return nil
	}

	ctx.Reply(wasmplugin.NewMessage(fmt.Sprintf(tr(ctx, "created"), name)))
	return nil
}

func handleJoin(ctx *wasmplugin.EventContext) error {
	db, err := openDB()
	if err != nil {
		ctx.LogError("queue_join: db: " + err.Error())
		ctx.Reply(wasmplugin.NewMessage(tr(ctx, "error")))
		return nil
	}
	defer db.Close()

	name := strings.TrimSpace(ctx.Param("name"))

	var q interface{ GetID() int64; GetName() string }
	if name == "" {
		sq, err := dbSoleOpenQueue(db, ctx.Messenger.ChatID)
		if err != nil {
			ctx.Reply(wasmplugin.NewMessage(err.Error()))
			return nil
		}
		q = &queueRef{id: sq.ID, name: sq.Name}
	} else {
		sq, err := dbFindOpenQueue(db, ctx.Messenger.ChatID, name)
		if err != nil {
			ctx.Reply(wasmplugin.NewMessage(fmt.Sprintf(tr(ctx, "not_found"), name)))
			return nil
		}
		q = &queueRef{id: sq.ID, name: sq.Name}
	}

	pos, err := dbJoinQueue(db, q.GetID(), ctx.Messenger.UserID)
	if err != nil {
		if errors.Is(err, errAlreadyInQueue) {
			ctx.Reply(wasmplugin.NewMessage(tr(ctx, "already_in_queue")))
			return nil
		}
		ctx.LogError("queue_join: join: " + err.Error())
		ctx.Reply(wasmplugin.NewMessage(tr(ctx, "error")))
		return nil
	}

	ctx.Reply(wasmplugin.NewMessage(fmt.Sprintf(tr(ctx, "joined"), q.GetName(), pos)))
	return nil
}

func handleLeave(ctx *wasmplugin.EventContext) error {
	db, err := openDB()
	if err != nil {
		ctx.LogError("queue_leave: db: " + err.Error())
		ctx.Reply(wasmplugin.NewMessage(tr(ctx, "error")))
		return nil
	}
	defer db.Close()

	queueName, err := dbLeaveQueue(db, ctx.Messenger.ChatID, ctx.Messenger.UserID)
	if err != nil {
		if errors.Is(err, errNotInQueue) {
			ctx.Reply(wasmplugin.NewMessage(tr(ctx, "not_in_queue")))
			return nil
		}
		ctx.LogError("queue_leave: " + err.Error())
		ctx.Reply(wasmplugin.NewMessage(tr(ctx, "error")))
		return nil
	}

	ctx.Reply(wasmplugin.NewMessage(fmt.Sprintf(tr(ctx, "left"), queueName)))
	return nil
}

func handleNext(ctx *wasmplugin.EventContext) error {
	db, err := openDB()
	if err != nil {
		ctx.LogError("queue_next: db: " + err.Error())
		ctx.Reply(wasmplugin.NewMessage(tr(ctx, "error")))
		return nil
	}
	defer db.Close()

	q, err := dbOwnerQueue(db, ctx.Messenger.ChatID, ctx.Messenger.UserID)
	if err != nil {
		if errors.Is(err, errNotQueueOwner) {
			ctx.Reply(wasmplugin.NewMessage(tr(ctx, "not_owner")))
			return nil
		}
		ctx.LogError("queue_next: owner: " + err.Error())
		ctx.Reply(wasmplugin.NewMessage(tr(ctx, "error")))
		return nil
	}

	nextUserID, err := dbNextInQueue(db, q.ID)
	if err != nil {
		if errors.Is(err, errQueueEmpty) {
			ctx.Reply(wasmplugin.NewMessage(fmt.Sprintf(tr(ctx, "queue_empty"), q.Name)))
			return nil
		}
		ctx.LogError("queue_next: next: " + err.Error())
		ctx.Reply(wasmplugin.NewMessage(tr(ctx, "error")))
		return nil
	}

	ctx.Reply(wasmplugin.NewMessage(fmt.Sprintf(tr(ctx, "next_up"), q.Name, nextUserID)))
	return nil
}

func handleStatus(ctx *wasmplugin.EventContext) error {
	db, err := openDB()
	if err != nil {
		ctx.LogError("queue_status: db: " + err.Error())
		ctx.Reply(wasmplugin.NewMessage(tr(ctx, "error")))
		return nil
	}
	defer db.Close()

	queues, err := dbListOpenQueues(db, ctx.Messenger.ChatID)
	if err != nil {
		ctx.LogError("queue_status: list: " + err.Error())
		ctx.Reply(wasmplugin.NewMessage(tr(ctx, "error")))
		return nil
	}
	if len(queues) == 0 {
		ctx.Reply(wasmplugin.NewMessage(tr(ctx, "no_queues")))
		return nil
	}

	var sb strings.Builder
	sb.WriteString(tr(ctx, "status_header") + "\n\n")
	for _, item := range queues {
		sb.WriteString(fmt.Sprintf("• %s — %d\n", item.Queue.Name, item.Count))

		places, err := dbQueuePlaces(db, item.Queue.ID)
		if err == nil && len(places) > 0 {
			for _, p := range places {
				sb.WriteString(fmt.Sprintf("  %d. user %d\n", p.Position, p.UserID))
			}
		}
	}

	ctx.Reply(wasmplugin.NewMessage(sb.String()))
	return nil
}

func handlePos(ctx *wasmplugin.EventContext) error {
	db, err := openDB()
	if err != nil {
		ctx.LogError("queue_pos: db: " + err.Error())
		ctx.Reply(wasmplugin.NewMessage(tr(ctx, "error")))
		return nil
	}
	defer db.Close()

	queueName, pos, err := dbUserPosition(db, ctx.Messenger.ChatID, ctx.Messenger.UserID)
	if err != nil {
		ctx.LogError("queue_pos: " + err.Error())
		ctx.Reply(wasmplugin.NewMessage(tr(ctx, "error")))
		return nil
	}
	if pos == 0 {
		ctx.Reply(wasmplugin.NewMessage(tr(ctx, "not_in_queue")))
		return nil
	}

	ctx.Reply(wasmplugin.NewMessage(fmt.Sprintf(tr(ctx, "your_pos"), queueName, pos)))
	return nil
}

func handleClose(ctx *wasmplugin.EventContext) error {
	db, err := openDB()
	if err != nil {
		ctx.LogError("queue_close: db: " + err.Error())
		ctx.Reply(wasmplugin.NewMessage(tr(ctx, "error")))
		return nil
	}
	defer db.Close()

	q, err := dbOwnerQueue(db, ctx.Messenger.ChatID, ctx.Messenger.UserID)
	if err != nil {
		if errors.Is(err, errNotQueueOwner) {
			ctx.Reply(wasmplugin.NewMessage(tr(ctx, "not_owner")))
			return nil
		}
		ctx.LogError("queue_close: owner: " + err.Error())
		ctx.Reply(wasmplugin.NewMessage(tr(ctx, "error")))
		return nil
	}

	if err := dbCloseQueue(db, q.ID, ctx.Messenger.UserID); err != nil {
		ctx.LogError("queue_close: close: " + err.Error())
		ctx.Reply(wasmplugin.NewMessage(tr(ctx, "error")))
		return nil
	}

	ctx.Reply(wasmplugin.NewMessage(fmt.Sprintf(tr(ctx, "closed"), q.Name)))
	return nil
}

// queueRef is a tiny helper so we don't duplicate field access.
type queueRef struct{ id int64; name string }

func (r *queueRef) GetID() int64    { return r.id }
func (r *queueRef) GetName() string { return r.name }

// tr returns a localised string for the user's locale.
func tr(ctx *wasmplugin.EventContext, key string) string {
	locale := ctx.Locale()
	if m, ok := messages[key]; ok {
		if s, ok := m[locale]; ok {
			return s
		}
		if s, ok := m["en"]; ok {
			return s
		}
	}
	return key
}

var messages = map[string]map[string]string{
	"error":          {"en": "Something went wrong. Please try again.", "ru": "Что-то пошло не так. Попробуйте ещё раз."},
	"name_empty":     {"en": "Queue name cannot be empty.", "ru": "Название очереди не может быть пустым."},
	"name_taken":     {"en": "Queue \"%s\" already exists in this chat.", "ru": "Очередь \"%s\" уже существует в этом чате."},
	"created":        {"en": "Queue \"%s\" created! Users can join with /queue_join.", "ru": "Очередь \"%s\" создана! Участники могут встать в неё командой /queue_join."},
	"not_found":      {"en": "Queue \"%s\" not found.", "ru": "Очередь \"%s\" не найдена."},
	"already_in_queue": {"en": "You are already in the queue.", "ru": "Вы уже стоите в очереди."},
	"joined":         {"en": "You joined queue \"%s\". Your position: #%d.", "ru": "Вы встали в очередь \"%s\". Ваша позиция: №%d."},
	"not_in_queue":   {"en": "You are not in any queue.", "ru": "Вы не стоите ни в одной очереди."},
	"left":           {"en": "You left queue \"%s\".", "ru": "Вы вышли из очереди \"%s\"."},
	"not_owner":      {"en": "Only the queue creator can do this.", "ru": "Только создатель очереди может это сделать."},
	"queue_empty":    {"en": "Queue \"%s\" is empty.", "ru": "Очередь \"%s\" пуста."},
	"next_up":        {"en": "Queue \"%s\" — next up: user %d", "ru": "Очередь \"%s\" — следующий: пользователь %d"},
	"no_queues":      {"en": "No active queues in this chat.", "ru": "В этом чате нет активных очередей."},
	"status_header":  {"en": "Active queues:", "ru": "Активные очереди:"},
	"your_pos":       {"en": "Queue \"%s\" — your position: #%d.", "ru": "Очередь \"%s\" — ваша позиция: №%d."},
	"closed":         {"en": "Queue \"%s\" closed.", "ru": "Очередь \"%s\" закрыта."},
}
