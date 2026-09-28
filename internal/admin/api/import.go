// internal/admin/api/import.go
package api

import (
	"SuperBotGo/internal/university"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/xuri/excelize/v2"
)

type ImportHandler struct {
	syncSvc *university.SyncService
	pool    *pgxpool.Pool
}

func NewImportHandler(syncSvc *university.SyncService, pool *pgxpool.Pool) *ImportHandler {
	return &ImportHandler{syncSvc: syncSvc, pool: pool}
}

func (h *ImportHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/admin/import/teachers/manual", h.handleCreateTeacherManual)
	mux.HandleFunc("GET /api/admin/persons/teachers", h.handleListTeachers)
	mux.HandleFunc("POST /api/admin/import/students", h.handleImportStudents)
	mux.HandleFunc("POST /api/admin/import/students/manual", h.handleCreateStudentManual)
	mux.HandleFunc("GET /api/admin/import/template", h.handleDownloadTemplate)
}

type ImportResult struct {
	Total   int           `json:"total"`
	Created int           `json:"created"`
	Updated int           `json:"updated"`
	Skipped int           `json:"skipped"`
	Errors  []ImportError `json:"errors,omitempty"`
}

type ImportError struct {
	Row     int    `json:"row"`
	Field   string `json:"field,omitempty"`
	Message string `json:"message"`
}

type createStudentManualRequest struct {
	ExternalID      string   `json:"external_id"`
	LastName        string   `json:"last_name"`
	FirstName       string   `json:"first_name"`
	MiddleName      string   `json:"middle_name"`
	Email           string   `json:"email"`
	Phone           string   `json:"phone"`
	ProgramCode     string   `json:"program_code"`
	StreamCode      string   `json:"stream_code"`
	GroupCode       string   `json:"group_code"`
	SubgroupCodes   []string `json:"subgroup_codes"`
	Status          string   `json:"status"`
	NationalityType string   `json:"nationality_type"`
	FundingType     string   `json:"funding_type"`
	EducationForm   string   `json:"education_form"`
}

// handleImportStudents обрабатывает POST с .xlsx файлом
func (h *ImportHandler) handleImportStudents(w http.ResponseWriter, r *http.Request) {
	maxSize := int64(10 << 20) // 10MB
	if err := r.ParseMultipartForm(maxSize); err != nil {
		writeError(w, http.StatusBadRequest, "failed to parse form: "+err.Error())
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "file is required")
		return
	}
	defer file.Close()

	// Parse Excel
	rows, err := parseStudentExcel(file)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	result := ImportResult{Total: len(rows)}

	for i, row := range rows {
		// Validate row
		if verr := validateStudentRow(row); verr != nil {
			result.Errors = append(result.Errors, ImportError{
				Row:     i + 2,
				Field:   verr.Field,
				Message: verr.Message,
			})
			result.Skipped++
			continue
		}

		// Import student
		created, err := h.importStudent(r.Context(), row)
		if err != nil {
			result.Errors = append(result.Errors, ImportError{
				Row:     i + 2,
				Message: err.Error(),
			})
			result.Skipped++
			continue
		}

		if created {
			result.Created++
		} else {
			result.Updated++
		}
	}

	writeJSON(w, http.StatusOK, result)
}

func (h *ImportHandler) handleCreateStudentManual(w http.ResponseWriter, r *http.Request) {
	var req createStudentManualRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}

	row := studentRow{
		ExternalID:      strings.TrimSpace(req.ExternalID),
		LastName:        strings.TrimSpace(req.LastName),
		FirstName:       strings.TrimSpace(req.FirstName),
		MiddleName:      strings.TrimSpace(req.MiddleName),
		Email:           strings.TrimSpace(req.Email),
		Phone:           strings.TrimSpace(req.Phone),
		ProgramCode:     strings.TrimSpace(req.ProgramCode),
		StreamCode:      strings.TrimSpace(req.StreamCode),
		GroupCode:       strings.TrimSpace(req.GroupCode),
		Status:          strings.TrimSpace(req.Status),
		NationalityType: strings.TrimSpace(req.NationalityType),
		FundingType:     strings.TrimSpace(req.FundingType),
		EducationForm:   strings.TrimSpace(req.EducationForm),
	}
	for _, code := range req.SubgroupCodes {
		if trimmed := strings.TrimSpace(code); trimmed != "" {
			row.SubgroupCodes = append(row.SubgroupCodes, trimmed)
		}
	}

	if verr := validateStudentRow(row); verr != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("%s: %s", verr.Field, verr.Message))
		return
	}

	created, err := h.importStudent(r.Context(), row)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"created": created,
	})
}

// handleDownloadTemplate отдаёт шаблон Excel
func (h *ImportHandler) handleDownloadTemplate(w http.ResponseWriter, r *http.Request) {
	f := excelize.NewFile()
	defer f.Close()

	dataSheet := "Students"
	f.SetSheetName("Sheet1", dataSheet)

	// Header row - только существующие поля
	headers := []string{
		"external_id", "last_name", "first_name", "middle_name",
		"email", "phone",
		"program_code", "stream_code", "group_code", "subgroup_codes",
		"status", "nationality_type", "funding_type", "education_form",
	}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(dataSheet, cell, h)
	}

	refs, err := h.loadTemplateReferences(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load template references: "+err.Error())
		return
	}

	// Example row based on actual reference data from DB.
	example := buildTemplateExample(refs)
	for i, v := range example {
		cell, _ := excelize.CoordinatesToCellName(i+1, 2)
		f.SetCellValue(dataSheet, cell, v)
	}

	// Style header
	style, _ := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})
	for i := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellStyle(dataSheet, cell, cell, style)
	}

	f.SetColWidth(dataSheet, "A", "N", 18)

	h.fillInstructionsSheet(f)
	h.fillReferenceSheet(f, "Programs", []string{"code", "name"}, refs.Programs)
	h.fillReferenceSheet(f, "Streams", []string{"code", "name"}, refs.Streams)
	h.fillReferenceSheet(f, "Groups", []string{"code", "name"}, refs.Groups)
	h.fillReferenceSheet(f, "Subgroups", []string{"code", "name"}, refs.Subgroups)
	if sheetIndex, err := f.GetSheetIndex(dataSheet); err == nil {
		f.SetActiveSheet(sheetIndex)
	}

	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", "attachment; filename=students_template.xlsx")
	f.Write(w)
}

type templateRefRow struct {
	Code string
	Name string
}

type templateReferences struct {
	Programs  []templateRefRow
	Streams   []templateRefRow
	Groups    []templateRefRow
	Subgroups []templateRefRow
}

func (h *ImportHandler) loadTemplateReferences(ctx context.Context) (templateReferences, error) {
	var refs templateReferences
	var err error

	if refs.Programs, err = h.queryTemplateRows(ctx, `SELECT code, name FROM programs ORDER BY code LIMIT 200`); err != nil {
		return templateReferences{}, err
	}
	if refs.Streams, err = h.queryTemplateRows(ctx, `SELECT code, name FROM streams ORDER BY code LIMIT 200`); err != nil {
		return templateReferences{}, err
	}
	if refs.Groups, err = h.queryTemplateRows(ctx, `SELECT code, name FROM study_groups ORDER BY code LIMIT 200`); err != nil {
		return templateReferences{}, err
	}
	if refs.Subgroups, err = h.queryTemplateRows(ctx, `SELECT code, name FROM subgroups ORDER BY code LIMIT 200`); err != nil {
		return templateReferences{}, err
	}

	return refs, nil
}

func (h *ImportHandler) queryTemplateRows(ctx context.Context, sql string) ([]templateRefRow, error) {
	rows, err := h.pool.Query(ctx, sql)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]templateRefRow, 0)
	for rows.Next() {
		var row templateRefRow
		if err := rows.Scan(&row.Code, &row.Name); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

func buildTemplateExample(refs templateReferences) []string {
	programCode := "program_code_from_Programs_sheet"
	streamCode := "stream_code_from_Streams_sheet"
	groupCode := "group_code_from_Groups_sheet"
	subgroupCodes := ""

	if len(refs.Programs) > 0 {
		programCode = refs.Programs[0].Code
	}
	if len(refs.Streams) > 0 {
		streamCode = refs.Streams[0].Code
	}
	if len(refs.Groups) > 0 {
		groupCode = refs.Groups[0].Code
	}
	if len(refs.Subgroups) > 0 {
		subgroupCodes = refs.Subgroups[0].Code
	}

	return []string{
		"STU002", "Петров", "Пётр", "Сергеевич",
		"petrov@university.ru", "+79005554433",
		programCode, streamCode, groupCode, subgroupCodes,
		"active", "domestic", "budget", "full_time",
	}
}

func (h *ImportHandler) fillInstructionsSheet(f *excelize.File) {
	const sheet = "Instructions"
	f.NewSheet(sheet)

	rows := [][]string{
		{"Как пользоваться шаблоном"},
		{"1. Заполняйте данные на листе Students."},
		{"2. Значения program_code, stream_code, group_code и subgroup_codes берите из справочных листов."},
		{"3. subgroup_codes можно оставить пустым или перечислить несколько кодов через запятую без пробелов."},
		{"4. Допустимые значения status: active."},
		{"5. Допустимые значения nationality_type: domestic, foreign."},
		{"6. Допустимые значения funding_type: budget, contract."},
		{"7. Допустимые значения education_form: full_time, part_time, remote."},
		{"8. Не меняйте названия колонок в первой строке листа Students."},
	}

	for rowIndex, row := range rows {
		for colIndex, value := range row {
			cell, _ := excelize.CoordinatesToCellName(colIndex+1, rowIndex+1)
			f.SetCellValue(sheet, cell, value)
		}
	}

	style, _ := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})
	f.SetCellStyle(sheet, "A1", "A1", style)
	f.SetColWidth(sheet, "A", "A", 120)
}

func (h *ImportHandler) fillReferenceSheet(f *excelize.File, sheet string, headers []string, rows []templateRefRow) {
	f.NewSheet(sheet)

	for i, header := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheet, cell, header)
	}

	style, _ := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})
	for i := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellStyle(sheet, cell, cell, style)
	}

	for rowIndex, row := range rows {
		f.SetCellValue(sheet, fmt.Sprintf("A%d", rowIndex+2), row.Code)
		f.SetCellValue(sheet, fmt.Sprintf("B%d", rowIndex+2), row.Name)
	}

	f.SetColWidth(sheet, "A", "A", 24)
	f.SetColWidth(sheet, "B", "B", 60)
}

// studentRow представляет одну строку из Excel
type studentRow struct {
	// Person fields
	ExternalID string
	LastName   string
	FirstName  string
	MiddleName string
	Email      string
	Phone      string

	// StudentPosition fields
	ProgramCode   string
	StreamCode    string
	GroupCode     string
	SubgroupCodes []string

	// Enums
	Status          string
	NationalityType string
	FundingType     string
	EducationForm   string
}

// validationError описывает ошибку валидации строки
type validationError struct {
	Field   string
	Message string
}

// parseStudentExcel читает .xlsx и возвращает slice строк
func parseStudentExcel(r io.Reader) ([]studentRow, error) {
	f, err := excelize.OpenReader(r)
	if err != nil {
		return nil, fmt.Errorf("failed to open excel: %w", err)
	}
	defer f.Close()

	sheetName := "Students"
	if _, err := f.GetSheetIndex(sheetName); err != nil {
		sheetName = "Sheet1"
	}

	rows, err := f.GetRows(sheetName)
	if err != nil {
		return nil, fmt.Errorf("failed to get rows: %w", err)
	}

	if len(rows) < 2 {
		return nil, fmt.Errorf("file must have header and at least one data row")
	}

	// Parse header to get column indices
	header := rows[0]
	colMap := make(map[string]int)
	for i, h := range header {
		colMap[strings.TrimSpace(strings.ToLower(h))] = i
	}

	var result []studentRow
	for i, row := range rows[1:] {
		if len(row) == 0 {
			continue
		}

		getCol := func(name string) string {
			if idx, ok := colMap[name]; ok && idx < len(row) {
				return strings.TrimSpace(row[idx])
			}
			return ""
		}

		sr := studentRow{
			// Person
			ExternalID: getCol("external_id"),
			LastName:   getCol("last_name"),
			FirstName:  getCol("first_name"),
			MiddleName: getCol("middle_name"),
			Email:      getCol("email"),
			Phone:      getCol("phone"),
			// StudentPosition
			ProgramCode:     getCol("program_code"),
			StreamCode:      getCol("stream_code"),
			GroupCode:       getCol("group_code"),
			Status:          getCol("status"),
			NationalityType: getCol("nationality_type"),
			FundingType:     getCol("funding_type"),
			EducationForm:   getCol("education_form"),
		}

		// Parse subgroup codes (comma-separated)
		sgStr := getCol("subgroup_codes")
		if sgStr != "" {
			parts := strings.Split(sgStr, ",")
			for _, p := range parts {
				if s := strings.TrimSpace(p); s != "" {
					sr.SubgroupCodes = append(sr.SubgroupCodes, s)
				}
			}
		}

		result = append(result, sr)
		slog.Debug("parsed row", "row", i+2, "external_id", sr.ExternalID)
	}

	return result, nil
}

// validateStudentRow проверяет корректность данных строки
func validateStudentRow(row studentRow) *validationError {
	if row.ExternalID == "" {
		return &validationError{Field: "external_id", Message: "required"}
	}
	if row.LastName == "" {
		return &validationError{Field: "last_name", Message: "required"}
	}
	if row.FirstName == "" {
		return &validationError{Field: "first_name", Message: "required"}
	}
	if row.GroupCode == "" {
		return &validationError{Field: "group_code", Message: "required"}
	}

	// Validate enums
	validStatus := map[string]bool{"active": true, "suspended": true, "ended": true}
	if row.Status != "" && !validStatus[row.Status] {
		return &validationError{Field: "status", Message: "must be: active, suspended, ended"}
	}

	validNat := map[string]bool{"domestic": true, "foreign": true}
	if row.NationalityType != "" && !validNat[row.NationalityType] {
		return &validationError{Field: "nationality_type", Message: "must be: domestic, foreign"}
	}

	validFund := map[string]bool{"budget": true, "contract": true}
	if row.FundingType != "" && !validFund[row.FundingType] {
		return &validationError{Field: "funding_type", Message: "must be: budget, contract"}
	}

	validEdu := map[string]bool{"full_time": true, "part_time": true, "remote": true}
	if row.EducationForm != "" && !validEdu[row.EducationForm] {
		return &validationError{Field: "education_form", Message: "must be: full_time, part_time, remote"}
	}

	return nil
}

// importStudent импортирует одного студента, возвращает (created, error)
func (h *ImportHandler) importStudent(ctx context.Context, row studentRow) (bool, error) {
	result, err := h.syncSvc.ImportStudent(ctx, university.StudentImportInput{
		Person: university.PersonInput{
			ExternalID: row.ExternalID,
			LastName:   row.LastName,
			FirstName:  row.FirstName,
			MiddleName: row.MiddleName,
			Email:      row.Email,
			Phone:      row.Phone,
		},
		Position: university.StudentPositionInput{
			PersonExternalID: row.ExternalID,
			ProgramCode:      row.ProgramCode,
			StreamCode:       row.StreamCode,
			GroupCode:        row.GroupCode,
			Status:           row.Status,
			NationalityType:  row.NationalityType,
			FundingType:      row.FundingType,
			EducationForm:    row.EducationForm,
		},
		SubgroupCodes: row.SubgroupCodes,
	})
	if err != nil {
		return false, err
	}

	for _, warning := range result.Warnings {
		slog.Warn("failed to sync subgroup",
			"external_id", row.ExternalID,
			"error", warning)
	}

	return result.Created, nil
}
