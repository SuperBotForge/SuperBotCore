package api

import (
	"net/http"
	"strconv"
)

func (h *PositionHandler) handlePersonPositions(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("personId"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, 400, "invalid person ID")
		return
	}
	positions, err := h.store.GetAllPositions(r.Context(), id)
	if err != nil {
		writeError(w, 500, "failed to load positions")
		return
	}
	writeJSON(w, 200, positions)
}

// These routes use person IDs, so an account in a messenger is not required.
func (h *PositionHandler) handlePersonPositionMutation(w http.ResponseWriter, r *http.Request) {
	personID, err := strconv.ParseInt(r.PathValue("personId"), 10, 64)
	if err != nil || personID <= 0 {
		writeError(w, 400, "invalid person ID")
		return
	}
	posID, err := strconv.ParseInt(r.PathValue("posId"), 10, 64)
	if err != nil || posID <= 0 {
		writeError(w, 400, "invalid position ID")
		return
	}
	kind := r.PathValue("kind")
	if kind != "student" && kind != "teacher" {
		writeError(w, 400, "invalid position type")
		return
	}
	positions, err := h.store.GetAllPositions(r.Context(), personID)
	if err != nil {
		writeError(w, 500, "failed to load positions")
		return
	}
	found := false
	if positions != nil {
		if kind == "student" {
			for _, p := range positions.Student {
				if p.ID == posID {
					found = true
				}
			}
		}
		if kind == "teacher" {
			for _, p := range positions.Teacher {
				if p.ID == posID {
					found = true
				}
			}
		}
	}
	if !found {
		writeError(w, 404, "position does not belong to this person")
		return
	}
	if r.Method == http.MethodDelete {
		if kind == "student" {
			err = h.store.DeleteStudentPosition(r.Context(), posID)
		} else {
			err = h.store.DeleteTeacherPosition(r.Context(), posID)
		}
	} else if kind == "student" {
		var req StudentPositionRequest
		if !decodeJSONBody(w, r, &req) {
			return
		}
		if message := validateStudentPositionRequest(req); message != "" {
			writeError(w, 400, message)
			return
		}
		err = h.store.UpdateStudentPosition(r.Context(), posID, req)
	} else {
		var req TeacherPositionRequest
		if !decodeJSONBody(w, r, &req) {
			return
		}
		if message := validateTeacherPositionRequest(req); message != "" {
			writeError(w, 400, message)
			return
		}
		err = h.store.UpdateTeacherPosition(r.Context(), posID, req)
	}
	if err != nil {
		writeError(w, 500, "failed to change position")
		return
	}
	writeJSON(w, 200, map[string]string{"status": "ok"})
}
