package api

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"SuperBotGo/internal/authz/outbox"
	"SuperBotGo/internal/authz/tuples"
)

type PersonInfo struct {
	ID         int64  `json:"id"`
	ExternalID string `json:"external_id,omitempty"`
	LastName   string `json:"last_name"`
	FirstName  string `json:"first_name"`
	MiddleName string `json:"middle_name,omitempty"`
	Email      string `json:"email,omitempty"`
	Phone      string `json:"phone,omitempty"`
}

type CreatePersonRequest struct {
	ExternalID string `json:"external_id"`
	LastName   string `json:"last_name"`
	FirstName  string `json:"first_name"`
	MiddleName string `json:"middle_name"`
	Email      string `json:"email"`
	Phone      string `json:"phone"`
}

type ImportedStudentInfo struct {
	PositionID       int64  `json:"position_id"`
	PersonID         int64  `json:"person_id"`
	GlobalUserID     *int64 `json:"global_user_id,omitempty"`
	ExternalID       string `json:"external_id,omitempty"`
	LastName         string `json:"last_name"`
	FirstName        string `json:"first_name"`
	MiddleName       string `json:"middle_name,omitempty"`
	Email            string `json:"email,omitempty"`
	Phone            string `json:"phone,omitempty"`
	ProgramName      string `json:"program_name,omitempty"`
	StreamName       string `json:"stream_name,omitempty"`
	StudyGroupName   string `json:"study_group_name,omitempty"`
	Status           string `json:"status"`
	ImportedViaExcel bool   `json:"imported_via_excel"`
}

type SubgroupBrief struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type StudentPositionInfo struct {
	ID              int64           `json:"id"`
	ProgramID       *int64          `json:"program_id,omitempty"`
	ProgramName     string          `json:"program_name,omitempty"`
	StreamID        *int64          `json:"stream_id,omitempty"`
	StreamName      string          `json:"stream_name,omitempty"`
	StudyGroupID    *int64          `json:"study_group_id,omitempty"`
	StudyGroupName  string          `json:"study_group_name,omitempty"`
	Status          string          `json:"status"`
	NationalityType string          `json:"nationality_type"`
	FundingType     string          `json:"funding_type"`
	EducationForm   string          `json:"education_form"`
	Subgroups       []SubgroupBrief `json:"subgroups"`
	// Auxiliary for cascading select init
	DepartmentID *int64 `json:"department_id,omitempty"`
	FacultyID    *int64 `json:"faculty_id,omitempty"`
}

type StudentPositionRequest struct {
	ProgramID       *int64  `json:"program_id"`
	StreamID        *int64  `json:"stream_id"`
	StudyGroupID    *int64  `json:"study_group_id"`
	Status          string  `json:"status"`
	NationalityType string  `json:"nationality_type"`
	FundingType     string  `json:"funding_type"`
	EducationForm   string  `json:"education_form"`
	SubgroupIDs     []int64 `json:"subgroup_ids"`
}

type TeacherPositionInfo struct {
	ID             int64  `json:"id"`
	DepartmentID   *int64 `json:"department_id,omitempty"`
	DepartmentName string `json:"department_name,omitempty"`
	PositionTitle  string `json:"position_title"`
	EmploymentType string `json:"employment_type"`
	Status         string `json:"status"`
	FacultyID      *int64 `json:"faculty_id,omitempty"`
}

type TeacherPositionRequest struct {
	DepartmentID   *int64 `json:"department_id"`
	PositionTitle  string `json:"position_title"`
	EmploymentType string `json:"employment_type"`
	Status         string `json:"status"`
}

type AdminAppointmentInfo struct {
	ID              int64  `json:"id"`
	AppointmentType string `json:"appointment_type"`
	ScopeType       string `json:"scope_type"`
	ScopeID         *int64 `json:"scope_id,omitempty"`
	ScopeName       string `json:"scope_name,omitempty"`
	Status          string `json:"status"`
}

type AdminAppointmentRequest struct {
	AppointmentType string `json:"appointment_type"`
	ScopeType       string `json:"scope_type"`
	ScopeID         *int64 `json:"scope_id"`
	Status          string `json:"status"`
}

type AllPositions struct {
	Student []StudentPositionInfo  `json:"student"`
	Teacher []TeacherPositionInfo  `json:"teacher"`
	Admin   []AdminAppointmentInfo `json:"admin"`
}

type PositionStore interface {
	GetPersonByUserID(ctx context.Context, globalUserID int64) (*PersonInfo, error)
	SearchUnlinkedPersons(ctx context.Context, query string) ([]PersonInfo, error)
	ListImportedStudents(ctx context.Context, query string) ([]ImportedStudentInfo, error)
	LinkPersonToUser(ctx context.Context, personID, globalUserID int64) error
	CreatePersonForUser(ctx context.Context, globalUserID int64, req CreatePersonRequest) (*PersonInfo, error)
	GetAllPositions(ctx context.Context, personID int64) (*AllPositions, error)

	CreateStudentPosition(ctx context.Context, personID int64, req StudentPositionRequest) (*StudentPositionInfo, error)
	UpdateStudentPosition(ctx context.Context, posID int64, req StudentPositionRequest) error
	DeleteStudentPosition(ctx context.Context, posID int64) error

	CreateTeacherPosition(ctx context.Context, personID int64, req TeacherPositionRequest) (*TeacherPositionInfo, error)
	UpdateTeacherPosition(ctx context.Context, posID int64, req TeacherPositionRequest) error
	DeleteTeacherPosition(ctx context.Context, posID int64) error

	CreateAdminAppointment(ctx context.Context, personID int64, req AdminAppointmentRequest) (*AdminAppointmentInfo, error)
	UpdateAdminAppointment(ctx context.Context, posID int64, req AdminAppointmentRequest) error
	DeleteAdminAppointment(ctx context.Context, posID int64) error
}

type PgPositionStore struct {
	pool *pgxpool.Pool
}

func NewPgPositionStore(pool *pgxpool.Pool) *PgPositionStore {
	return &PgPositionStore{pool: pool}
}

var _ PositionStore = (*PgPositionStore)(nil)

func (s *PgPositionStore) GetPersonByUserID(ctx context.Context, globalUserID int64) (*PersonInfo, error) {
	var p PersonInfo
	err := s.pool.QueryRow(ctx,
		`SELECT id, COALESCE(external_id,''), last_name, first_name, COALESCE(middle_name,''), COALESCE(email,''), COALESCE(phone,'')
		 FROM persons WHERE global_user_id = $1`, globalUserID,
	).Scan(&p.ID, &p.ExternalID, &p.LastName, &p.FirstName, &p.MiddleName, &p.Email, &p.Phone)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get person: %w", err)
	}
	return &p, nil
}

func (s *PgPositionStore) SearchUnlinkedPersons(ctx context.Context, query string) ([]PersonInfo, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, COALESCE(external_id,''), last_name, first_name, COALESCE(middle_name,''), COALESCE(email,''), COALESCE(phone,'')
		 FROM persons
		 WHERE global_user_id IS NULL
		   AND (last_name ILIKE $1 OR first_name ILIKE $1 OR external_id ILIKE $1 OR email ILIKE $1)
		 ORDER BY last_name, first_name
		 LIMIT 20`, "%"+query+"%")
	if err != nil {
		return nil, fmt.Errorf("search persons: %w", err)
	}
	defer rows.Close()

	result := make([]PersonInfo, 0)
	for rows.Next() {
		var p PersonInfo
		if err := rows.Scan(&p.ID, &p.ExternalID, &p.LastName, &p.FirstName, &p.MiddleName, &p.Email, &p.Phone); err != nil {
			return nil, fmt.Errorf("scan person: %w", err)
		}
		result = append(result, p)
	}
	return result, nil
}

func (s *PgPositionStore) ListImportedStudents(ctx context.Context, query string) ([]ImportedStudentInfo, error) {
	searchTerm := "%"
	if query != "" {
		searchTerm = "%" + query + "%"
	}

	rows, err := s.pool.Query(ctx, `
		SELECT
			p.id,
			p.global_user_id,
			COALESCE(p.external_id, ''),
			p.last_name,
			p.first_name,
			COALESCE(p.middle_name, ''),
			COALESCE(p.email, ''),
			COALESCE(p.phone, ''),
			COALESCE(pr.name, ''),
			COALESCE(st.name, st.code, ''),
			COALESCE(gr.name, gr.code, ''),
			sp.status, sp.id
		FROM persons p
		JOIN student_positions sp ON sp.person_id = p.id
		LEFT JOIN programs pr ON pr.id = sp.program_id
		LEFT JOIN streams st ON st.id = sp.stream_id
		LEFT JOIN study_groups gr ON gr.id = sp.study_group_id
		WHERE
			p.last_name ILIKE $1 OR
			p.first_name ILIKE $1 OR
			COALESCE(p.middle_name, '') ILIKE $1 OR
			COALESCE(p.external_id, '') ILIKE $1 OR
			COALESCE(p.email, '') ILIKE $1
		ORDER BY p.last_name, p.first_name, p.id, sp.id
		LIMIT 200
	`, searchTerm)
	if err != nil {
		return nil, fmt.Errorf("list imported students: %w", err)
	}
	defer rows.Close()

	result := make([]ImportedStudentInfo, 0)
	for rows.Next() {
		var item ImportedStudentInfo
		if err := rows.Scan(
			&item.PersonID,
			&item.GlobalUserID,
			&item.ExternalID,
			&item.LastName,
			&item.FirstName,
			&item.MiddleName,
			&item.Email,
			&item.Phone,
			&item.ProgramName,
			&item.StreamName,
			&item.StudyGroupName,
			&item.Status, &item.PositionID,
		); err != nil {
			return nil, fmt.Errorf("scan imported student: %w", err)
		}
		item.ImportedViaExcel = true
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *PgPositionStore) LinkPersonToUser(ctx context.Context, personID, globalUserID int64) error {
	result, err := s.pool.Exec(ctx,
		`UPDATE persons SET global_user_id = $2, updated_at = now() WHERE id = $1 AND global_user_id IS NULL`,
		personID, globalUserID)
	if err != nil {
		return fmt.Errorf("link person: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("person not found or already linked")
	}
	return nil
}

func (s *PgPositionStore) CreatePersonForUser(ctx context.Context, globalUserID int64, req CreatePersonRequest) (*PersonInfo, error) {
	var p PersonInfo
	err := s.pool.QueryRow(ctx,
		`INSERT INTO persons (external_id, last_name, first_name, middle_name, email, phone, global_user_id)
		 VALUES (NULLIF($1,''), $2, $3, NULLIF($4,''), NULLIF($5,''), NULLIF($6,''), $7)
		 RETURNING id, COALESCE(external_id,''), last_name, first_name, COALESCE(middle_name,''), COALESCE(email,''), COALESCE(phone,'')`,
		req.ExternalID, req.LastName, req.FirstName, req.MiddleName, req.Email, req.Phone, globalUserID,
	).Scan(&p.ID, &p.ExternalID, &p.LastName, &p.FirstName, &p.MiddleName, &p.Email, &p.Phone)
	if err != nil {
		return nil, fmt.Errorf("create person: %w", err)
	}
	return &p, nil
}

func (s *PgPositionStore) GetAllPositions(ctx context.Context, personID int64) (*AllPositions, error) {
	result := &AllPositions{
		Student: make([]StudentPositionInfo, 0),
		Teacher: make([]TeacherPositionInfo, 0),
		Admin:   make([]AdminAppointmentInfo, 0),
	}

	studentPositions, err := s.loadStudentPositions(ctx, personID)
	if err != nil {
		return nil, err
	}
	result.Student = studentPositions

	if err := s.loadStudentSubgroups(ctx, result.Student); err != nil {
		return nil, err
	}

	teacherPositions, err := s.loadTeacherPositions(ctx, personID)
	if err != nil {
		return nil, err
	}
	result.Teacher = teacherPositions

	adminAppointments, err := s.loadAdminAppointments(ctx, personID)
	if err != nil {
		return nil, err
	}
	result.Admin = adminAppointments

	return result, nil
}

func (s *PgPositionStore) loadStudentPositions(ctx context.Context, personID int64) ([]StudentPositionInfo, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT sp.id, sp.program_id, COALESCE(p.name,''), sp.stream_id, COALESCE(st.name, st.code, ''),
		       sp.study_group_id, COALESCE(sg.name, sg.code, ''), sp.status, sp.nationality_type, sp.funding_type, sp.education_form,
		       p.department_id, d.faculty_id
		FROM student_positions sp
		LEFT JOIN programs p ON sp.program_id = p.id
		LEFT JOIN departments d ON p.department_id = d.id
		LEFT JOIN streams st ON sp.stream_id = st.id
		LEFT JOIN study_groups sg ON sp.study_group_id = sg.id
		WHERE sp.person_id = $1
		ORDER BY sp.id`, personID)
	if err != nil {
		return nil, fmt.Errorf("query student positions: %w", err)
	}
	defer rows.Close()

	positions := make([]StudentPositionInfo, 0)
	for rows.Next() {
		var sp StudentPositionInfo
		if err := rows.Scan(&sp.ID, &sp.ProgramID, &sp.ProgramName, &sp.StreamID, &sp.StreamName,
			&sp.StudyGroupID, &sp.StudyGroupName, &sp.Status, &sp.NationalityType, &sp.FundingType, &sp.EducationForm,
			&sp.DepartmentID, &sp.FacultyID); err != nil {
			return nil, fmt.Errorf("scan student position: %w", err)
		}
		sp.Subgroups = make([]SubgroupBrief, 0)
		positions = append(positions, sp)
	}
	return positions, nil
}

func (s *PgPositionStore) loadStudentSubgroups(ctx context.Context, positions []StudentPositionInfo) error {
	if len(positions) == 0 {
		return nil
	}

	posIDs := make([]int64, len(positions))
	posIdx := make(map[int64]int, len(positions))
	for i, sp := range positions {
		posIDs[i] = sp.ID
		posIdx[sp.ID] = i
	}

	rows, err := s.pool.Query(ctx, `
		SELECT ss.student_position_id, sub.id, COALESCE(sub.name, sub.code)
		FROM student_subgroups ss
		JOIN subgroups sub ON sub.id = ss.subgroup_id
		WHERE ss.student_position_id = ANY($1)
		ORDER BY sub.id`, posIDs)
	if err != nil {
		return fmt.Errorf("query student subgroups: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var posID, subID int64
		var subName string
		if err := rows.Scan(&posID, &subID, &subName); err != nil {
			return fmt.Errorf("scan student subgroup: %w", err)
		}
		if idx, ok := posIdx[posID]; ok {
			positions[idx].Subgroups = append(positions[idx].Subgroups, SubgroupBrief{ID: subID, Name: subName})
		}
	}
	return nil
}

func (s *PgPositionStore) loadTeacherPositions(ctx context.Context, personID int64) ([]TeacherPositionInfo, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT tp.id, tp.department_id, COALESCE(d.name,''), tp.position_title, tp.employment_type, tp.status, d.faculty_id
		FROM teacher_positions tp
		LEFT JOIN departments d ON tp.department_id = d.id
		WHERE tp.person_id = $1
		ORDER BY tp.id`, personID)
	if err != nil {
		return nil, fmt.Errorf("query teacher positions: %w", err)
	}
	defer rows.Close()

	positions := make([]TeacherPositionInfo, 0)
	for rows.Next() {
		var tp TeacherPositionInfo
		if err := rows.Scan(&tp.ID, &tp.DepartmentID, &tp.DepartmentName, &tp.PositionTitle, &tp.EmploymentType, &tp.Status, &tp.FacultyID); err != nil {
			return nil, fmt.Errorf("scan teacher position: %w", err)
		}
		positions = append(positions, tp)
	}
	return positions, nil
}

func (s *PgPositionStore) loadAdminAppointments(ctx context.Context, personID int64) ([]AdminAppointmentInfo, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT aa.id, aa.appointment_type, aa.scope_type, aa.scope_id, aa.status,
		       COALESCE(f.name, d.name, p.name, st.name, sg.name, '') as scope_name
		FROM administrative_appointments aa
		LEFT JOIN faculties f ON aa.scope_type = 'faculty' AND aa.scope_id = f.id
		LEFT JOIN departments d ON aa.scope_type = 'department' AND aa.scope_id = d.id
		LEFT JOIN programs p ON aa.scope_type = 'program' AND aa.scope_id = p.id
		LEFT JOIN streams st ON aa.scope_type = 'stream' AND aa.scope_id = st.id
		LEFT JOIN study_groups sg ON aa.scope_type = 'group' AND aa.scope_id = sg.id
		WHERE aa.person_id = $1
		ORDER BY aa.id`, personID)
	if err != nil {
		return nil, fmt.Errorf("query admin appointments: %w", err)
	}
	defer rows.Close()

	appointments := make([]AdminAppointmentInfo, 0)
	for rows.Next() {
		var aa AdminAppointmentInfo
		if err := rows.Scan(&aa.ID, &aa.AppointmentType, &aa.ScopeType, &aa.ScopeID, &aa.Status, &aa.ScopeName); err != nil {
			return nil, fmt.Errorf("scan admin appointment: %w", err)
		}
		appointments = append(appointments, aa)
	}
	return appointments, nil
}

func (s *PgPositionStore) syncStudentMemberTuple(ctx context.Context, tx pgx.Tx, personID int64) error {
	var externalID *string
	err := tx.QueryRow(ctx, `SELECT external_id FROM persons WHERE id = $1`, personID).Scan(&externalID)
	if err != nil {
		return err
	}
	if externalID == nil || *externalID == "" {
		return nil
	}

	if err := outbox.EnqueueDeleteBySubject(ctx, tx, "user", *externalID, "member"); err != nil {
		return err
	}

	rows, err := tx.Query(ctx,
		`SELECT DISTINCT 'study_group', sg.code FROM student_positions sp
		 JOIN study_groups sg ON sg.id = sp.study_group_id
		 WHERE sp.person_id = $1 AND sp.status = 'active' AND sp.study_group_id IS NOT NULL
         UNION
         SELECT DISTINCT 'subgroup', sg.code FROM student_positions sp
         JOIN student_subgroups ss ON ss.student_position_id=sp.id
         JOIN subgroups sg ON sg.id=ss.subgroup_id
         WHERE sp.person_id=$1 AND sp.status='active'`, personID)
	if err != nil {
		return err
	}
	defer rows.Close()

	var tt []tuples.Tuple
	for rows.Next() {
		var objectType, code string
		if err := rows.Scan(&objectType, &code); err != nil {
			return err
		}
		tt = append(tt, tuples.Tuple{ObjectType: objectType, ObjectID: code, Relation: "member", SubjectType: "user", SubjectID: *externalID})
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	if len(tt) > 0 {
		return outbox.EnqueueTouch(ctx, tx, tt)
	}
	return nil
}

func (s *PgPositionStore) saveSubgroups(ctx context.Context, tx pgx.Tx, positionID int64, subgroupIDs []int64) error {
	_, err := tx.Exec(ctx, `DELETE FROM student_subgroups WHERE student_position_id = $1`, positionID)
	if err != nil {
		return fmt.Errorf("clear student subgroups: %w", err)
	}
	for _, sgID := range subgroupIDs {
		_, err := tx.Exec(ctx,
			`INSERT INTO student_subgroups (student_position_id, subgroup_id) VALUES ($1, $2)
			 ON CONFLICT (student_position_id, subgroup_id) DO NOTHING`,
			positionID, sgID)
		if err != nil {
			return fmt.Errorf("insert student subgroup: %w", err)
		}
	}
	return nil
}

func (s *PgPositionStore) withStudentPositionTx(ctx context.Context, fn func(tx pgx.Tx) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func studentPositionInfoFromRequest(id int64, req StudentPositionRequest) *StudentPositionInfo {
	return &StudentPositionInfo{
		ID:              id,
		ProgramID:       req.ProgramID,
		StreamID:        req.StreamID,
		StudyGroupID:    req.StudyGroupID,
		Status:          req.Status,
		NationalityType: req.NationalityType,
		FundingType:     req.FundingType,
		EducationForm:   req.EducationForm,
	}
}

func (s *PgPositionStore) studentPositionPersonID(ctx context.Context, tx pgx.Tx, posID int64) (int64, error) {
	var personID int64
	if err := tx.QueryRow(ctx, `SELECT person_id FROM student_positions WHERE id = $1`, posID).Scan(&personID); err != nil {
		return 0, fmt.Errorf("find position: %w", err)
	}
	return personID, nil
}

func (s *PgPositionStore) CreateStudentPosition(ctx context.Context, personID int64, req StudentPositionRequest) (*StudentPositionInfo, error) {
	var id int64
	if err := s.withStudentPositionTx(ctx, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx,
			`INSERT INTO student_positions (person_id, program_id, stream_id, study_group_id, status, nationality_type, funding_type, education_form)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id`,
			personID, req.ProgramID, req.StreamID, req.StudyGroupID, req.Status, req.NationalityType, req.FundingType, req.EducationForm,
		).Scan(&id); err != nil {
			return fmt.Errorf("create student position: %w", err)
		}
		if err := s.saveSubgroups(ctx, tx, id, req.SubgroupIDs); err != nil {
			return err
		}
		if err := s.syncStudentMemberTuple(ctx, tx, personID); err != nil {
			return fmt.Errorf("sync spicedb: %w", err)
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return studentPositionInfoFromRequest(id, req), nil
}

func (s *PgPositionStore) UpdateStudentPosition(ctx context.Context, posID int64, req StudentPositionRequest) error {
	return s.withStudentPositionTx(ctx, func(tx pgx.Tx) error {
		personID, err := s.studentPositionPersonID(ctx, tx, posID)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx,
			`UPDATE student_positions SET program_id=$2, stream_id=$3, study_group_id=$4, status=$5,
			 nationality_type=$6, funding_type=$7, education_form=$8, updated_at=now() WHERE id=$1`,
			posID, req.ProgramID, req.StreamID, req.StudyGroupID, req.Status, req.NationalityType, req.FundingType, req.EducationForm); err != nil {
			return fmt.Errorf("update student position: %w", err)
		}
		if err := s.saveSubgroups(ctx, tx, posID, req.SubgroupIDs); err != nil {
			return err
		}
		if err := s.syncStudentMemberTuple(ctx, tx, personID); err != nil {
			return fmt.Errorf("sync spicedb: %w", err)
		}
		return nil
	})
}

func (s *PgPositionStore) DeleteStudentPosition(ctx context.Context, posID int64) error {
	return s.withStudentPositionTx(ctx, func(tx pgx.Tx) error {
		personID, err := s.studentPositionPersonID(ctx, tx, posID)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM student_positions WHERE id = $1`, posID); err != nil {
			return err
		}
		if err := s.syncStudentMemberTuple(ctx, tx, personID); err != nil {
			return fmt.Errorf("sync spicedb: %w", err)
		}
		return nil
	})
}

func (s *PgPositionStore) CreateTeacherPosition(ctx context.Context, personID int64, req TeacherPositionRequest) (*TeacherPositionInfo, error) {
	var id int64
	err := s.pool.QueryRow(ctx,
		`INSERT INTO teacher_positions (person_id, department_id, position_title, employment_type, status)
		 VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		personID, req.DepartmentID, req.PositionTitle, req.EmploymentType, req.Status,
	).Scan(&id)
	if err != nil {
		return nil, fmt.Errorf("create teacher position: %w", err)
	}
	return &TeacherPositionInfo{ID: id, DepartmentID: req.DepartmentID, PositionTitle: req.PositionTitle,
		EmploymentType: req.EmploymentType, Status: req.Status}, nil
}

func (s *PgPositionStore) UpdateTeacherPosition(ctx context.Context, posID int64, req TeacherPositionRequest) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE teacher_positions SET department_id=$2, position_title=$3, employment_type=$4, status=$5, updated_at=now() WHERE id=$1`,
		posID, req.DepartmentID, req.PositionTitle, req.EmploymentType, req.Status)
	if err != nil {
		return fmt.Errorf("update teacher position: %w", err)
	}
	return nil
}

func (s *PgPositionStore) DeleteTeacherPosition(ctx context.Context, posID int64) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM teacher_positions WHERE id = $1`, posID)
	return err
}

func (s *PgPositionStore) CreateAdminAppointment(ctx context.Context, personID int64, req AdminAppointmentRequest) (*AdminAppointmentInfo, error) {
	var id int64
	err := s.pool.QueryRow(ctx,
		`INSERT INTO administrative_appointments (person_id, appointment_type, scope_type, scope_id, status)
		 VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		personID, req.AppointmentType, req.ScopeType, req.ScopeID, req.Status,
	).Scan(&id)
	if err != nil {
		return nil, fmt.Errorf("create admin appointment: %w", err)
	}
	return &AdminAppointmentInfo{ID: id, AppointmentType: req.AppointmentType, ScopeType: req.ScopeType,
		ScopeID: req.ScopeID, Status: req.Status}, nil
}

func (s *PgPositionStore) UpdateAdminAppointment(ctx context.Context, posID int64, req AdminAppointmentRequest) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE administrative_appointments SET appointment_type=$2, scope_type=$3, scope_id=$4, status=$5, updated_at=now() WHERE id=$1`,
		posID, req.AppointmentType, req.ScopeType, req.ScopeID, req.Status)
	if err != nil {
		return fmt.Errorf("update admin appointment: %w", err)
	}
	return nil
}

func (s *PgPositionStore) DeleteAdminAppointment(ctx context.Context, posID int64) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM administrative_appointments WHERE id = $1`, posID)
	return err
}
