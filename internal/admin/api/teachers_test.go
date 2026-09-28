package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestManualTeacherRejectsInvalidInput(t *testing.T) {
	for _, body := range []string{
		`{}`, `{ "external_id":" ","last_name":"Name","first_name":"First" }`,
		`{"external_id":"id","last_name":"Name","first_name":"First","department_id":0,"position_title":"Lecturer","status":"active","employment_type":"full_time"}`,
		`{"external_id":"id","last_name":"Name","first_name":"First","department_id":1,"position_title":" ","status":"active","employment_type":"full_time"}`,
		`{"external_id":"id","last_name":"Name","first_name":"First","department_id":1,"position_title":"Lecturer","status":"bad","employment_type":"full_time"}`,
		`{"external_id":"id","last_name":"Name","first_name":"First","department_id":1,"position_title":"Lecturer","status":"active","employment_type":"bad"}`,
	} {
		h := NewImportHandler(nil, nil)
		rec := httptest.NewRecorder()
		h.handleCreateTeacherManual(rec, httptest.NewRequest(http.MethodPost, "/api/admin/import/teachers/manual", strings.NewReader(body)))
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("body=%s status=%d", body, rec.Code)
		}
	}
}
