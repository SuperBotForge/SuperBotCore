package api

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DeanStore provides read/write access scoped to a single faculty.
type DeanStore struct {
	pool *pgxpool.Pool
}

func NewDeanStore(pool *pgxpool.Pool) *DeanStore {
	return &DeanStore{pool: pool}
}

// GetDeanFacultyID returns the faculty_id for the active dean appointment of
// the given global_user_id, or 0 if none exists.
func (s *DeanStore) GetDeanFacultyID(ctx context.Context, globalUserID int64) (int64, error) {
	var facultyID int64
	err := s.pool.QueryRow(ctx, `
		SELECT aa.scope_id
		FROM administrative_appointments aa
		JOIN persons p ON p.id = aa.person_id
		WHERE p.global_user_id = $1
		  AND aa.appointment_type = 'dean'
		  AND aa.scope_type = 'faculty'
		  AND aa.status = 'active'
		LIMIT 1
	`, globalUserID).Scan(&facultyID)
	if err == pgx.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("get dean faculty: %w", err)
	}
	return facultyID, nil
}

// FacultyStats is returned by GetFacultyStats.
type FacultyStats struct {
	FacultyID       int64  `json:"faculty_id"`
	FacultyName     string `json:"faculty_name"`
	FacultyCode     string `json:"faculty_code"`
	GroupCount      int    `json:"group_count"`
	ActiveStudents  int    `json:"active_students"`
	BudgetStudents  int    `json:"budget_students"`
	ContractStudents int   `json:"contract_students"`
	ForeignStudents int    `json:"foreign_students"`
}

func (s *DeanStore) GetFacultyStats(ctx context.Context, facultyID int64) (FacultyStats, error) {
	var st FacultyStats
	err := s.pool.QueryRow(ctx, `
		SELECT faculty_id, faculty_name, faculty_code,
		       group_count, active_students, budget_students,
		       contract_students, foreign_students
		FROM v_faculty_stats
		WHERE faculty_id = $1
	`, facultyID).Scan(
		&st.FacultyID, &st.FacultyName, &st.FacultyCode,
		&st.GroupCount, &st.ActiveStudents, &st.BudgetStudents,
		&st.ContractStudents, &st.ForeignStudents,
	)
	if err == pgx.ErrNoRows {
		return FacultyStats{FacultyID: facultyID}, nil
	}
	if err != nil {
		return FacultyStats{}, fmt.Errorf("get faculty stats: %w", err)
	}
	return st, nil
}

// GroupStats is one row from v_group_stats.
type GroupStats struct {
	GroupID          int64  `json:"group_id"`
	GroupCode        string `json:"group_code"`
	GroupName        string `json:"group_name"`
	ProgramName      string `json:"program_name"`
	ActiveStudents   int    `json:"active_students"`
	BudgetStudents   int    `json:"budget_students"`
	ContractStudents int    `json:"contract_students"`
	ForeignStudents  int    `json:"foreign_students"`
}

func (s *DeanStore) ListGroupStats(ctx context.Context, facultyID int64) ([]GroupStats, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT group_id, group_code, COALESCE(group_name,''), program_name,
		       active_students, budget_students, contract_students, foreign_students
		FROM v_group_stats
		WHERE faculty_id = $1
		ORDER BY group_code
	`, facultyID)
	if err != nil {
		return nil, fmt.Errorf("list group stats: %w", err)
	}
	defer rows.Close()

	var result []GroupStats
	for rows.Next() {
		var g GroupStats
		if err := rows.Scan(
			&g.GroupID, &g.GroupCode, &g.GroupName, &g.ProgramName,
			&g.ActiveStudents, &g.BudgetStudents, &g.ContractStudents, &g.ForeignStudents,
		); err != nil {
			return nil, fmt.Errorf("scan group stats: %w", err)
		}
		result = append(result, g)
	}
	return result, rows.Err()
}

// StudentRow is one row from v_students.
type StudentRow struct {
	PersonID        int64   `json:"person_id"`
	ExternalID      string  `json:"external_id,omitempty"`
	LastName        string  `json:"last_name"`
	FirstName       string  `json:"first_name"`
	MiddleName      string  `json:"middle_name,omitempty"`
	Email           string  `json:"email,omitempty"`
	Phone           string  `json:"phone,omitempty"`
	BotUserID       *int64  `json:"bot_user_id,omitempty"`
	PositionID      int64   `json:"position_id"`
	Status          string  `json:"status"`
	NationalityType string  `json:"nationality_type"`
	FundingType     string  `json:"funding_type"`
	EducationForm   string  `json:"education_form"`
	GroupID         *int64  `json:"group_id,omitempty"`
	GroupCode       string  `json:"group_code,omitempty"`
	ProgramName     string  `json:"program_name,omitempty"`
}

type ListStudentsFilter struct {
	GroupID         int64
	Status          string
	NationalityType string
	FundingType     string
	Search          string
}

func (s *DeanStore) ListStudents(ctx context.Context, facultyID int64, f ListStudentsFilter) ([]StudentRow, error) {
	query := `
		SELECT person_id, COALESCE(external_id,''), last_name, first_name,
		       COALESCE(middle_name,''), COALESCE(email,''), COALESCE(phone,''),
		       bot_user_id, position_id, status, nationality_type, funding_type, education_form,
		       group_id, COALESCE(group_code,''), COALESCE(program_name,'')
		FROM v_students
		WHERE faculty_id = $1`
	args := []any{facultyID}
	n := 2

	if f.GroupID > 0 {
		query += fmt.Sprintf(" AND group_id = $%d", n)
		args = append(args, f.GroupID)
		n++
	}
	if f.Status != "" {
		query += fmt.Sprintf(" AND status = $%d", n)
		args = append(args, f.Status)
		n++
	}
	if f.NationalityType != "" {
		query += fmt.Sprintf(" AND nationality_type = $%d", n)
		args = append(args, f.NationalityType)
		n++
	}
	if f.FundingType != "" {
		query += fmt.Sprintf(" AND funding_type = $%d", n)
		args = append(args, f.FundingType)
		n++
	}
	if f.Search != "" {
		query += fmt.Sprintf(` AND (last_name ILIKE $%d OR first_name ILIKE $%d OR email ILIKE $%d OR external_id ILIKE $%d)`, n, n, n, n)
		args = append(args, "%"+f.Search+"%")
		n++
	}
	_ = n

	query += " ORDER BY last_name, first_name"

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list students: %w", err)
	}
	defer rows.Close()

	var result []StudentRow
	for rows.Next() {
		var r StudentRow
		if err := rows.Scan(
			&r.PersonID, &r.ExternalID, &r.LastName, &r.FirstName,
			&r.MiddleName, &r.Email, &r.Phone,
			&r.BotUserID, &r.PositionID, &r.Status, &r.NationalityType, &r.FundingType, &r.EducationForm,
			&r.GroupID, &r.GroupCode, &r.ProgramName,
		); err != nil {
			return nil, fmt.Errorf("scan student row: %w", err)
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

// UpdateStudentPosition updates mutable fields of a student position.
type UpdateStudentPositionRequest struct {
	StudyGroupID    *int64  `json:"study_group_id"`
	Status          string  `json:"status"`
	NationalityType string  `json:"nationality_type"`
	FundingType     string  `json:"funding_type"`
	EducationForm   string  `json:"education_form"`
}

func (s *DeanStore) UpdateStudentPosition(ctx context.Context, positionID int64, req UpdateStudentPositionRequest) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE student_positions SET
			study_group_id   = COALESCE($2, study_group_id),
			status           = $3,
			nationality_type = $4,
			funding_type     = $5,
			education_form   = $6,
			updated_at       = now()
		WHERE id = $1
	`, positionID, req.StudyGroupID, req.Status, req.NationalityType, req.FundingType, req.EducationForm)
	if err != nil {
		return fmt.Errorf("update student position %d: %w", positionID, err)
	}
	return nil
}

// UpdatePersonContacts updates contact info for a person.
type UpdatePersonContactsRequest struct {
	Email string `json:"email"`
	Phone string `json:"phone"`
}

func (s *DeanStore) UpdatePersonContacts(ctx context.Context, personID int64, req UpdatePersonContactsRequest) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE persons SET email = $2, phone = $3, updated_at = now() WHERE id = $1
	`, personID, req.Email, req.Phone)
	if err != nil {
		return fmt.Errorf("update person contacts %d: %w", personID, err)
	}
	return nil
}

// GetStudentByPosition returns a single student row by position_id, checking faculty scope.
func (s *DeanStore) GetStudentByPosition(ctx context.Context, positionID, facultyID int64) (StudentRow, bool, error) {
	var r StudentRow
	err := s.pool.QueryRow(ctx, `
		SELECT person_id, COALESCE(external_id,''), last_name, first_name,
		       COALESCE(middle_name,''), COALESCE(email,''), COALESCE(phone,''),
		       bot_user_id, position_id, status, nationality_type, funding_type, education_form,
		       group_id, COALESCE(group_code,''), COALESCE(program_name,'')
		FROM v_students
		WHERE position_id = $1 AND faculty_id = $2
	`, positionID, facultyID).Scan(
		&r.PersonID, &r.ExternalID, &r.LastName, &r.FirstName,
		&r.MiddleName, &r.Email, &r.Phone,
		&r.BotUserID, &r.PositionID, &r.Status, &r.NationalityType, &r.FundingType, &r.EducationForm,
		&r.GroupID, &r.GroupCode, &r.ProgramName,
	)
	if err == pgx.ErrNoRows {
		return StudentRow{}, false, nil
	}
	if err != nil {
		return StudentRow{}, false, fmt.Errorf("get student by position: %w", err)
	}
	return r, true, nil
}

// ListFacultyGroups returns all groups belonging to a faculty (for filter dropdowns).
type GroupBrief struct {
	ID   int64  `json:"id"`
	Code string `json:"code"`
	Name string `json:"name"`
}

func (s *DeanStore) ListFacultyGroups(ctx context.Context, facultyID int64) ([]GroupBrief, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT sg.id, sg.code, COALESCE(sg.name,'')
		FROM study_groups sg
		JOIN streams     st ON st.id = sg.stream_id
		JOIN programs    pr ON pr.id = st.program_id
		JOIN departments d  ON d.id  = pr.department_id
		WHERE d.faculty_id = $1
		ORDER BY sg.code
	`, facultyID)
	if err != nil {
		return nil, fmt.Errorf("list faculty groups: %w", err)
	}
	defer rows.Close()

	var result []GroupBrief
	for rows.Next() {
		var g GroupBrief
		if err := rows.Scan(&g.ID, &g.Code, &g.Name); err != nil {
			return nil, fmt.Errorf("scan group brief: %w", err)
		}
		result = append(result, g)
	}
	return result, rows.Err()
}
