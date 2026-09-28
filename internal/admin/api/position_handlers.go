package api

import (
	"net/http"
	"strconv"
)

func validateStudentPositionRequest(req StudentPositionRequest) string {
	if req.StudyGroupID == nil || *req.StudyGroupID == 0 {
		return "study_group_id is required"
	}
	if req.Status == "" {
		return "status is required"
	}
	if req.NationalityType == "" {
		return "nationality_type is required"
	}
	if req.FundingType == "" {
		return "funding_type is required"
	}
	if req.EducationForm == "" {
		return "education_form is required"
	}
	return ""
}

type PositionHandler struct {
	store PositionStore
}

func NewPositionHandler(store PositionStore) *PositionHandler {
	return &PositionHandler{store: store}
}

func (h *PositionHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/admin/persons/{personId}/positions", h.handlePersonPositions)
	mux.HandleFunc("PUT /api/admin/persons/{personId}/positions/{kind}/{posId}", h.handlePersonPositionMutation)
	mux.HandleFunc("DELETE /api/admin/persons/{personId}/positions/{kind}/{posId}", h.handlePersonPositionMutation)
	mux.HandleFunc("GET /api/admin/users/{id}/person", h.handleGetPerson)
	mux.HandleFunc("POST /api/admin/users/{id}/person", h.handleCreatePerson)
	mux.HandleFunc("POST /api/admin/users/{id}/person/link", h.handleLinkPerson)
	mux.HandleFunc("GET /api/admin/persons/search", h.handleSearchPersons)
	mux.HandleFunc("GET /api/admin/persons/imported", h.handleListImportedStudents)
	mux.HandleFunc("GET /api/admin/users/{id}/positions", h.handleGetPositions)

	mux.HandleFunc("POST /api/admin/users/{id}/positions/student", h.handleCreateStudentPosition)
	mux.HandleFunc("PUT /api/admin/users/{id}/positions/student/{posId}", h.handleUpdateStudentPosition)
	mux.HandleFunc("DELETE /api/admin/users/{id}/positions/student/{posId}", h.handleDeleteStudentPosition)

	mux.HandleFunc("POST /api/admin/users/{id}/positions/teacher", h.handleCreateTeacherPosition)
	mux.HandleFunc("PUT /api/admin/users/{id}/positions/teacher/{posId}", h.handleUpdateTeacherPosition)
	mux.HandleFunc("DELETE /api/admin/users/{id}/positions/teacher/{posId}", h.handleDeleteTeacherPosition)

	mux.HandleFunc("POST /api/admin/users/{id}/positions/admin-appointment", h.handleCreateAdminAppointment)
	mux.HandleFunc("PUT /api/admin/users/{id}/positions/admin-appointment/{posId}", h.handleUpdateAdminAppointment)
	mux.HandleFunc("DELETE /api/admin/users/{id}/positions/admin-appointment/{posId}", h.handleDeleteAdminAppointment)
}

func (h *PositionHandler) requirePerson(w http.ResponseWriter, r *http.Request) (int64, bool) {
	userID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user ID")
		return 0, false
	}
	person, err := h.store.GetPersonByUserID(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get person")
		return 0, false
	}
	if person == nil {
		writeError(w, http.StatusNotFound, "no person linked to this user")
		return 0, false
	}
	return person.ID, true
}

func (h *PositionHandler) handleGetPerson(w http.ResponseWriter, r *http.Request) {
	userID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user ID")
		return
	}
	person, err := h.store.GetPersonByUserID(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get person")
		return
	}
	if person == nil {
		writeJSON(w, http.StatusOK, nil)
		return
	}
	writeJSON(w, http.StatusOK, person)
}

func (h *PositionHandler) handleCreatePerson(w http.ResponseWriter, r *http.Request) {
	userID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user ID")
		return
	}
	var req CreatePersonRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}
	if req.LastName == "" || req.FirstName == "" {
		writeError(w, http.StatusBadRequest, "last_name and first_name are required")
		return
	}
	person, err := h.store.CreatePersonForUser(r.Context(), userID, req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create person")
		return
	}
	writeJSON(w, http.StatusCreated, person)
}

func (h *PositionHandler) handleSearchPersons(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" {
		writeJSON(w, http.StatusOK, []PersonInfo{})
		return
	}
	persons, err := h.store.SearchUnlinkedPersons(r.Context(), q)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to search persons")
		return
	}
	writeJSON(w, http.StatusOK, persons)
}

func (h *PositionHandler) handleListImportedStudents(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	items, err := h.store.ListImportedStudents(r.Context(), q)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list imported students")
		return
	}
	if items == nil {
		items = []ImportedStudentInfo{}
	}
	writeJSON(w, http.StatusOK, items)
}

func (h *PositionHandler) handleLinkPerson(w http.ResponseWriter, r *http.Request) {
	userID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user ID")
		return
	}
	var req struct {
		PersonID int64 `json:"person_id"`
	}
	if !decodeJSONBody(w, r, &req) {
		return
	}
	if req.PersonID == 0 {
		writeError(w, http.StatusBadRequest, "person_id is required")
		return
	}
	if err := h.store.LinkPersonToUser(r.Context(), req.PersonID, userID); err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "linked"})
}

func (h *PositionHandler) handleGetPositions(w http.ResponseWriter, r *http.Request) {
	personID, ok := h.requirePerson(w, r)
	if !ok {
		return
	}
	positions, err := h.store.GetAllPositions(r.Context(), personID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get positions")
		return
	}
	writeJSON(w, http.StatusOK, positions)
}

func (h *PositionHandler) handleCreateStudentPosition(w http.ResponseWriter, r *http.Request) {
	personID, ok := h.requirePerson(w, r)
	if !ok {
		return
	}
	var req StudentPositionRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}
	if msg := validateStudentPositionRequest(req); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	pos, err := h.store.CreateStudentPosition(r.Context(), personID, req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, pos)
}

func (h *PositionHandler) handleUpdateStudentPosition(w http.ResponseWriter, r *http.Request) {
	posID, err := strconv.ParseInt(r.PathValue("posId"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid position ID")
		return
	}
	var req StudentPositionRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}
	if msg := validateStudentPositionRequest(req); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	if err := h.store.UpdateStudentPosition(r.Context(), posID, req); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (h *PositionHandler) handleDeleteStudentPosition(w http.ResponseWriter, r *http.Request) {
	posID, err := strconv.ParseInt(r.PathValue("posId"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid position ID")
		return
	}
	if err := h.store.DeleteStudentPosition(r.Context(), posID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete student position")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *PositionHandler) handleCreateTeacherPosition(w http.ResponseWriter, r *http.Request) {
	personID, ok := h.requirePerson(w, r)
	if !ok {
		return
	}
	var req TeacherPositionRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}
	if err := validateTeacherPositionRequest(req); err != "" {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	pos, err := h.store.CreateTeacherPosition(r.Context(), personID, req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create teacher position")
		return
	}
	writeJSON(w, http.StatusCreated, pos)
}

func (h *PositionHandler) handleUpdateTeacherPosition(w http.ResponseWriter, r *http.Request) {
	posID, err := strconv.ParseInt(r.PathValue("posId"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid position ID")
		return
	}
	var req TeacherPositionRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}
	if err := validateTeacherPositionRequest(req); err != "" {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := h.store.UpdateTeacherPosition(r.Context(), posID, req); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update teacher position")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (h *PositionHandler) handleDeleteTeacherPosition(w http.ResponseWriter, r *http.Request) {
	posID, err := strconv.ParseInt(r.PathValue("posId"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid position ID")
		return
	}
	if err := h.store.DeleteTeacherPosition(r.Context(), posID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete teacher position")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *PositionHandler) handleCreateAdminAppointment(w http.ResponseWriter, r *http.Request) {
	personID, ok := h.requirePerson(w, r)
	if !ok {
		return
	}
	var req AdminAppointmentRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}
	pos, err := h.store.CreateAdminAppointment(r.Context(), personID, req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create admin appointment")
		return
	}
	writeJSON(w, http.StatusCreated, pos)
}

func (h *PositionHandler) handleUpdateAdminAppointment(w http.ResponseWriter, r *http.Request) {
	posID, err := strconv.ParseInt(r.PathValue("posId"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid position ID")
		return
	}
	var req AdminAppointmentRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}
	if err := h.store.UpdateAdminAppointment(r.Context(), posID, req); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update admin appointment")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (h *PositionHandler) handleDeleteAdminAppointment(w http.ResponseWriter, r *http.Request) {
	posID, err := strconv.ParseInt(r.PathValue("posId"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid position ID")
		return
	}
	if err := h.store.DeleteAdminAppointment(r.Context(), posID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete admin appointment")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
