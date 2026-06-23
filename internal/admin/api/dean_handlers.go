package api

import (
	"net/http"
	"strconv"
)

// DeanHandler serves the dean's office API under /api/dean/*.
// All endpoints require an active admin session AND a 'dean' appointment
// in administrative_appointments scoped to a faculty.
type DeanHandler struct {
	store *DeanStore
	auth  *AuthHandler
}

func NewDeanHandler(store *DeanStore, auth *AuthHandler) *DeanHandler {
	return &DeanHandler{store: store, auth: auth}
}

func (h *DeanHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/dean/me", h.handleMe)
	mux.HandleFunc("GET /api/dean/dashboard", h.handleDashboard)
	mux.HandleFunc("GET /api/dean/groups", h.handleListGroups)
	mux.HandleFunc("GET /api/dean/students", h.handleListStudents)
	mux.HandleFunc("GET /api/dean/students/{positionId}", h.handleGetStudent)
	mux.HandleFunc("PUT /api/dean/students/{positionId}", h.handleUpdateStudent)
}

// deanScope authenticates the request and returns the faculty_id for the dean.
func (h *DeanHandler) deanScope(w http.ResponseWriter, r *http.Request) (int64, bool) {
	userID, ok := h.auth.AuthenticateSession(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return 0, false
	}
	facultyID, err := h.store.GetDeanFacultyID(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to resolve dean scope")
		return 0, false
	}
	if facultyID == 0 {
		writeError(w, http.StatusForbidden, "no dean appointment found for this account")
		return 0, false
	}
	return facultyID, true
}

// handleMe returns the current user's faculty info.
func (h *DeanHandler) handleMe(w http.ResponseWriter, r *http.Request) {
	facultyID, ok := h.deanScope(w, r)
	if !ok {
		return
	}
	stats, err := h.store.GetFacultyStats(r.Context(), facultyID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load faculty info")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"faculty_id":   stats.FacultyID,
		"faculty_name": stats.FacultyName,
		"faculty_code": stats.FacultyCode,
	})
}

// handleDashboard returns stats + group breakdown for the dean's faculty.
func (h *DeanHandler) handleDashboard(w http.ResponseWriter, r *http.Request) {
	facultyID, ok := h.deanScope(w, r)
	if !ok {
		return
	}

	stats, err := h.store.GetFacultyStats(r.Context(), facultyID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load faculty stats")
		return
	}

	groups, err := h.store.ListGroupStats(r.Context(), facultyID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load group stats")
		return
	}
	if groups == nil {
		groups = []GroupStats{}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"faculty": stats,
		"groups":  groups,
	})
}

// handleListGroups returns a flat list of groups for the dean's faculty (for filter dropdowns).
func (h *DeanHandler) handleListGroups(w http.ResponseWriter, r *http.Request) {
	facultyID, ok := h.deanScope(w, r)
	if !ok {
		return
	}

	groups, err := h.store.ListFacultyGroups(r.Context(), facultyID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load groups")
		return
	}
	if groups == nil {
		groups = []GroupBrief{}
	}
	writeJSON(w, http.StatusOK, groups)
}

// handleListStudents returns students of the faculty with optional filters.
func (h *DeanHandler) handleListStudents(w http.ResponseWriter, r *http.Request) {
	facultyID, ok := h.deanScope(w, r)
	if !ok {
		return
	}

	q := r.URL.Query()
	filter := ListStudentsFilter{
		Status:          q.Get("status"),
		NationalityType: q.Get("nationality_type"),
		FundingType:     q.Get("funding_type"),
		Search:          q.Get("search"),
	}
	if gid := q.Get("group_id"); gid != "" {
		if v, err := strconv.ParseInt(gid, 10, 64); err == nil {
			filter.GroupID = v
		}
	}

	students, err := h.store.ListStudents(r.Context(), facultyID, filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load students")
		return
	}
	if students == nil {
		students = []StudentRow{}
	}
	writeJSON(w, http.StatusOK, students)
}

// handleGetStudent returns a single student by position ID, scoped to dean's faculty.
func (h *DeanHandler) handleGetStudent(w http.ResponseWriter, r *http.Request) {
	facultyID, ok := h.deanScope(w, r)
	if !ok {
		return
	}

	positionID, err := strconv.ParseInt(r.PathValue("positionId"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid position id")
		return
	}

	student, found, err := h.store.GetStudentByPosition(r.Context(), positionID, facultyID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load student")
		return
	}
	if !found {
		writeError(w, http.StatusNotFound, "student not found")
		return
	}
	writeJSON(w, http.StatusOK, student)
}

// handleUpdateStudent updates position and contact info for a student.
func (h *DeanHandler) handleUpdateStudent(w http.ResponseWriter, r *http.Request) {
	facultyID, ok := h.deanScope(w, r)
	if !ok {
		return
	}

	positionID, err := strconv.ParseInt(r.PathValue("positionId"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid position id")
		return
	}

	// Verify the student belongs to this dean's faculty.
	student, found, err := h.store.GetStudentByPosition(r.Context(), positionID, facultyID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to verify student scope")
		return
	}
	if !found {
		writeError(w, http.StatusNotFound, "student not found")
		return
	}

	var body struct {
		StudyGroupID    *int64 `json:"study_group_id"`
		Status          string `json:"status"`
		NationalityType string `json:"nationality_type"`
		FundingType     string `json:"funding_type"`
		EducationForm   string `json:"education_form"`
		Email           string `json:"email"`
		Phone           string `json:"phone"`
	}
	if !decodeJSONBody(w, r, &body) {
		return
	}

	if body.Status == "" || body.NationalityType == "" || body.FundingType == "" || body.EducationForm == "" {
		writeError(w, http.StatusBadRequest, "status, nationality_type, funding_type and education_form are required")
		return
	}

	if err := h.store.UpdateStudentPosition(r.Context(), positionID, UpdateStudentPositionRequest{
		StudyGroupID:    body.StudyGroupID,
		Status:          body.Status,
		NationalityType: body.NationalityType,
		FundingType:     body.FundingType,
		EducationForm:   body.EducationForm,
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update student position")
		return
	}

	if body.Email != "" || body.Phone != "" {
		if err := h.store.UpdatePersonContacts(r.Context(), student.PersonID, UpdatePersonContactsRequest{
			Email: body.Email,
			Phone: body.Phone,
		}); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to update contacts")
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "saved"})
}
