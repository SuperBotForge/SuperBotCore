package main

import (
	"database/sql"
	"errors"
	"fmt"
)

var (
	errQueueNotFound    = errors.New("queue not found")
	errAlreadyInQueue   = errors.New("user already in queue")
	errNotInQueue       = errors.New("user is not in queue")
	errNotQueueOwner    = errors.New("only the queue creator can do this")
	errQueueNameTaken   = errors.New("a queue with this name already exists")
	errNoOpenQueue      = errors.New("no open queue in this chat")
	errQueueEmpty       = errors.New("queue is empty")
)

func openDB() (*sql.DB, error) {
	return sql.Open("superbot", "")
}

type queue struct {
	ID        int64
	ChatID    string
	Name      string
	Status    string
	CreatedBy int64
}

type queuePlace struct {
	ID       int64
	QueueID  int64
	UserID   int64
	Position int
	Status   string
}

// dbCreateQueue creates a new open queue. Returns errQueueNameTaken if the name is already in use.
func dbCreateQueue(db *sql.DB, chatID, name string, createdBy int64) (int64, error) {
	var id int64
	err := db.QueryRow(
		`INSERT INTO queues (chat_id, name, created_by) VALUES ($1, $2, $3) RETURNING id`,
		chatID, name, createdBy,
	).Scan(&id)
	if err != nil {
		// unique index violation on (chat_id, name) where status='open'
		return 0, fmt.Errorf("%w: %s", errQueueNameTaken, err.Error())
	}
	return id, nil
}

// dbFindOpenQueue returns the open queue with the given name in the chat.
func dbFindOpenQueue(db *sql.DB, chatID, name string) (*queue, error) {
	q := &queue{}
	err := db.QueryRow(
		`SELECT id, chat_id, name, status, created_by FROM queues
		 WHERE chat_id = $1 AND name = $2 AND status = 'open'`,
		chatID, name,
	).Scan(&q.ID, &q.ChatID, &q.Name, &q.Status, &q.CreatedBy)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errQueueNotFound
	}
	return q, err
}

// dbListOpenQueues returns all open queues in the chat with their member counts.
func dbListOpenQueues(db *sql.DB, chatID string) ([]struct {
	Queue queue
	Count int
}, error) {
	rows, err := db.Query(
		`SELECT q.id, q.chat_id, q.name, q.status, q.created_by,
		        COUNT(p.id) FILTER (WHERE p.status = 'waiting') AS cnt
		 FROM queues q
		 LEFT JOIN queue_places p ON p.queue_id = q.id
		 WHERE q.chat_id = $1 AND q.status = 'open'
		 GROUP BY q.id
		 ORDER BY q.created_at`,
		chatID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []struct {
		Queue queue
		Count int
	}
	for rows.Next() {
		var item struct {
			Queue queue
			Count int
		}
		if err := rows.Scan(
			&item.Queue.ID, &item.Queue.ChatID, &item.Queue.Name,
			&item.Queue.Status, &item.Queue.CreatedBy, &item.Count,
		); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

// dbSoleOpenQueue returns the single open queue in the chat, or errNoOpenQueue
// if there are none, or an error message if there are multiple.
func dbSoleOpenQueue(db *sql.DB, chatID string) (*queue, error) {
	rows, err := dbListOpenQueues(db, chatID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, errNoOpenQueue
	}
	if len(rows) > 1 {
		return nil, fmt.Errorf("multiple queues are open — specify the queue name")
	}
	return &rows[0].Queue, nil
}

// dbOwnerQueue returns the open queue that the given user created.
func dbOwnerQueue(db *sql.DB, chatID string, userID int64) (*queue, error) {
	q := &queue{}
	err := db.QueryRow(
		`SELECT id, chat_id, name, status, created_by FROM queues
		 WHERE chat_id = $1 AND created_by = $2 AND status = 'open'
		 ORDER BY created_at DESC LIMIT 1`,
		chatID, userID,
	).Scan(&q.ID, &q.ChatID, &q.Name, &q.Status, &q.CreatedBy)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errNotQueueOwner
	}
	return q, err
}

// dbJoinQueue adds the user to the queue. Returns errAlreadyInQueue if already present.
func dbJoinQueue(db *sql.DB, queueID, userID int64) (int, error) {
	// check duplicate
	var existing int64
	err := db.QueryRow(
		`SELECT id FROM queue_places WHERE queue_id = $1 AND user_id = $2 AND status = 'waiting'`,
		queueID, userID,
	).Scan(&existing)
	if err == nil {
		return 0, errAlreadyInQueue
	}

	var pos int
	err = db.QueryRow(
		`INSERT INTO queue_places (queue_id, user_id, position)
		 SELECT $1, $2, COALESCE(MAX(position), 0) + 1
		 FROM queue_places
		 WHERE queue_id = $1
		 RETURNING position`,
		queueID, userID,
	).Scan(&pos)
	return pos, err
}

// dbLeaveQueue removes the user from any waiting queue in the chat.
// Returns the queue name they left, or errNotInQueue.
func dbLeaveQueue(db *sql.DB, chatID string, userID int64) (string, error) {
	var queueName string
	err := db.QueryRow(
		`DELETE FROM queue_places p
		 USING queues q
		 WHERE p.queue_id = q.id
		   AND q.chat_id = $1
		   AND p.user_id = $2
		   AND p.status = 'waiting'
		   AND q.status = 'open'
		 RETURNING q.name`,
		chatID, userID,
	).Scan(&queueName)
	if errors.Is(err, sql.ErrNoRows) {
		return "", errNotInQueue
	}
	return queueName, err
}

// dbNextInQueue pops the first waiting place from the queue.
// Returns the userID of the next person, or errQueueEmpty.
func dbNextInQueue(db *sql.DB, queueID int64) (int64, error) {
	var nextUserID int64
	err := db.QueryRow(
		`UPDATE queue_places
		 SET status = 'done'
		 WHERE id = (
		     SELECT id FROM queue_places
		     WHERE queue_id = $1 AND status = 'waiting'
		     ORDER BY position ASC
		     LIMIT 1
		 )
		 RETURNING user_id`,
		queueID,
	).Scan(&nextUserID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, errQueueEmpty
	}
	return nextUserID, err
}

// dbQueuePlaces returns the waiting list for a queue.
func dbQueuePlaces(db *sql.DB, queueID int64) ([]queuePlace, error) {
	rows, err := db.Query(
		`SELECT id, queue_id, user_id, position, status
		 FROM queue_places
		 WHERE queue_id = $1 AND status = 'waiting'
		 ORDER BY position`,
		queueID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var places []queuePlace
	for rows.Next() {
		var p queuePlace
		if err := rows.Scan(&p.ID, &p.QueueID, &p.UserID, &p.Position, &p.Status); err != nil {
			return nil, err
		}
		places = append(places, p)
	}
	return places, rows.Err()
}

// dbCloseQueue marks the queue as closed. Only the creator can do this.
func dbCloseQueue(db *sql.DB, queueID, createdBy int64) error {
	res, err := db.Exec(
		`UPDATE queues SET status = 'closed' WHERE id = $1 AND created_by = $2 AND status = 'open'`,
		queueID, createdBy,
	)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return errNotQueueOwner
	}
	return nil
}

// dbUserPosition returns the user's position in a queue (1-based), or 0 if not found.
func dbUserPosition(db *sql.DB, chatID string, userID int64) (queueName string, pos int, err error) {
	err = db.QueryRow(
		`SELECT q.name, p.position
		 FROM queue_places p
		 JOIN queues q ON q.id = p.queue_id
		 WHERE q.chat_id = $1 AND p.user_id = $2
		   AND p.status = 'waiting' AND q.status = 'open'
		 ORDER BY p.joined_at
		 LIMIT 1`,
		chatID, userID,
	).Scan(&queueName, &pos)
	if errors.Is(err, sql.ErrNoRows) {
		return "", 0, nil
	}
	return queueName, pos, err
}
