package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"
)

type manualTeacherRequest struct {
	CreatePersonRequest
	TeacherPositionRequest
}

func validateManualTeacher(req *manualTeacherRequest) string {
	req.ExternalID = strings.TrimSpace(req.ExternalID)
	req.LastName = strings.TrimSpace(req.LastName)
	req.FirstName = strings.TrimSpace(req.FirstName)
	req.PositionTitle = strings.TrimSpace(req.PositionTitle)
	if req.ExternalID == "" || req.LastName == "" || req.FirstName == "" {
		return "external_id, last_name and first_name are required"
	}
	return validateTeacherPositionRequest(req.TeacherPositionRequest)
}

func validateTeacherPositionRequest(req TeacherPositionRequest) string {
	if req.DepartmentID == nil || *req.DepartmentID <= 0 {
		return "department_id is required"
	}
	if strings.TrimSpace(req.PositionTitle) == "" {
		return "position_title is required"
	}
	if req.Status != "active" && req.Status != "suspended" && req.Status != "ended" {
		return "invalid status"
	}
	if req.EmploymentType != "full_time" && req.EmploymentType != "part_time" && req.EmploymentType != "hourly" {
		return "invalid employment_type"
	}
	return ""
}

func (h *ImportHandler) handleCreateTeacherManual(w http.ResponseWriter, r *http.Request) {
	var req manualTeacherRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}
	if err := validateManualTeacher(&req); err != "" {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := h.createTeacher(r.Context(), req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"status": "ok"})
}

func (h *ImportHandler) createTeacher(ctx context.Context, req manualTeacherRequest) error {
	tx, err := h.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var departmentID int64
	if err := tx.QueryRow(ctx, `SELECT id FROM departments WHERE id=$1`, req.DepartmentID).Scan(&departmentID); err != nil {
		return fmt.Errorf("department not found")
	}
	// An existing person can also be a student. Keep their identity and contacts.
	_, err = tx.Exec(ctx, `INSERT INTO persons(external_id,last_name,first_name,middle_name,email,phone)
 VALUES($1,$2,$3,NULLIF($4,''),NULLIF($5,''),NULLIF($6,'')) ON CONFLICT(external_id) DO NOTHING`,
		req.ExternalID, req.LastName, req.FirstName, req.MiddleName, req.Email, req.Phone)
	if err != nil {
		return err
	}
	var personID int64
	var lastName, firstName string
	if err := tx.QueryRow(ctx, `SELECT id,last_name,first_name FROM persons WHERE external_id=$1 FOR UPDATE`, req.ExternalID).Scan(&personID, &lastName, &firstName); err != nil {
		return err
	}
	if !strings.EqualFold(lastName, req.LastName) || !strings.EqualFold(firstName, req.FirstName) {
		return fmt.Errorf("external_id already belongs to a person with a different name")
	}
	var exists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM teacher_positions WHERE person_id=$1 AND department_id=$2 AND position_title=$3)`, personID, departmentID, req.PositionTitle).Scan(&exists); err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("this teacher position already exists")
	}
	_, err = tx.Exec(ctx, `UPDATE persons p SET global_user_id=u.id, updated_at=now() FROM global_users u
 WHERE p.id=$1 AND p.global_user_id IS NULL AND u.tsu_accounts_id=p.external_id
 AND NOT EXISTS(SELECT 1 FROM persons other WHERE other.global_user_id=u.id AND other.id<>p.id)`, personID)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO teacher_positions(person_id,department_id,position_title,employment_type,status) VALUES($1,$2,$3,$4,$5)`,
		personID, departmentID, req.PositionTitle, req.EmploymentType, req.Status)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

type importedTeacherInfo struct {
	PersonInfo
	PositionID     int64  `json:"position_id"`
	GlobalUserID   *int64 `json:"global_user_id,omitempty"`
	DepartmentName string `json:"department_name"`
	PositionTitle  string `json:"position_title"`
	EmploymentType string `json:"employment_type"`
	Status         string `json:"status"`
}

func (h *ImportHandler) handleListTeachers(w http.ResponseWriter, r *http.Request) {
	rows, err := h.pool.Query(r.Context(), `SELECT p.id,COALESCE(p.external_id,''),p.last_name,p.first_name,
 COALESCE(p.middle_name,''),COALESCE(p.email,''),COALESCE(p.phone,''),tp.id,p.global_user_id,
 COALESCE(d.name,''),tp.position_title,tp.employment_type,tp.status
 FROM teacher_positions tp JOIN persons p ON p.id=tp.person_id LEFT JOIN departments d ON d.id=tp.department_id
 WHERE concat_ws(' ',p.last_name,p.first_name,p.middle_name,p.external_id,p.email,d.name) ILIKE $1
 ORDER BY p.last_name,p.first_name,tp.id`, "%"+r.URL.Query().Get("q")+"%")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list teachers")
		return
	}
	defer rows.Close()
	items, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (importedTeacherInfo, error) {
		var p importedTeacherInfo
		err := row.Scan(&p.ID, &p.ExternalID, &p.LastName, &p.FirstName, &p.MiddleName, &p.Email, &p.Phone, &p.PositionID, &p.GlobalUserID, &p.DepartmentName, &p.PositionTitle, &p.EmploymentType, &p.Status)
		return p, err
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to read teachers")
		return
	}
	if items == nil {
		items = []importedTeacherInfo{}
	}
	writeJSON(w, http.StatusOK, items)
}
