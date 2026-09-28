package outbox

import "SuperBotGo/internal/authz/tuples"

// Operation types stored in the outbox.
const (
	OpSyncDepartmentStaff = "SYNC_DEPARTMENT_STAFF"
	OpTouch               = "TOUCH"
	OpDelete              = "DELETE"
	OpDeleteByObject      = "DELETE_BY_OBJECT"
	OpDeleteBySubject     = "DELETE_BY_SUBJECT"
	OpReplace             = "REPLACE"
)

// TupleJSON is the JSON-serialisable form of tuples.Tuple.
type TupleJSON struct {
	ObjectType  string `json:"object_type"`
	ObjectID    string `json:"object_id"`
	Relation    string `json:"relation"`
	SubjectType string `json:"subject_type"`
	SubjectID   string `json:"subject_id"`
}

// Payload is the JSONB structure stored in authz_outbox.payload.
type Payload struct {
	ObjectType  string      `json:"object_type,omitempty"`
	ObjectID    string      `json:"object_id,omitempty"`
	Relation    string      `json:"relation,omitempty"`
	SubjectType string      `json:"subject_type,omitempty"`
	SubjectID   string      `json:"subject_id,omitempty"`
	Tuples      []TupleJSON `json:"tuples,omitempty"`
}

func FromTuples(tt []tuples.Tuple) []TupleJSON {
	out := make([]TupleJSON, len(tt))
	for i, t := range tt {
		out[i] = TupleJSON{
			ObjectType:  t.ObjectType,
			ObjectID:    t.ObjectID,
			Relation:    t.Relation,
			SubjectType: t.SubjectType,
			SubjectID:   t.SubjectID,
		}
	}
	return out
}

func ToTuples(jj []TupleJSON) []tuples.Tuple {
	out := make([]tuples.Tuple, len(jj))
	for i, j := range jj {
		out[i] = tuples.Tuple{
			ObjectType:  j.ObjectType,
			ObjectID:    j.ObjectID,
			Relation:    j.Relation,
			SubjectType: j.SubjectType,
			SubjectID:   j.SubjectID,
		}
	}
	return out
}
