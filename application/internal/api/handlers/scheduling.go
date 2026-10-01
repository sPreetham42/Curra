package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/sPreetham42/timetable-platform/application/internal/domain"
)

func (h *Handlers) ListSchedulingConfigs(w http.ResponseWriter, r *http.Request) {
	instID, ok := services.InstitutionIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "not authenticated")
		return
	}
	items, err := h.scheduling.ListSchedulingConfigs(r.Context(), instID)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (h *Handlers) CreateSchedulingConfig(w http.ResponseWriter, r *http.Request) {
	instID, ok := services.InstitutionIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "not authenticated")
		return
	}
	var req struct {
		Name   string          `json:"name"`
		Config json.RawMessage `json:"config"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "name is required")
		return
	}
	item, err := h.scheduling.CreateSchedulingConfig(r.Context(), instID, req.Name, req.Config)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (h *Handlers) GetSchedulingConfig(w http.ResponseWriter, r *http.Request) {
	instID, ok := services.InstitutionIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "not authenticated")
		return
	}
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid id")
		return
	}
	item, err := h.scheduling.GetSchedulingConfig(r.Context(), instID, id)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h *Handlers) UpdateSchedulingConfig(w http.ResponseWriter, r *http.Request) {
	instID, ok := services.InstitutionIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "not authenticated")
		return
	}
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid id")
		return
	}
	ifMatch := parseIfMatch(r)
	var req struct {
		Name   string          `json:"name"`
		Config json.RawMessage `json:"config"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "name is required")
		return
	}
	item, err := h.scheduling.UpdateSchedulingConfig(r.Context(), instID, id, req.Name, req.Config, ifMatch)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h *Handlers) DeleteSchedulingConfig(w http.ResponseWriter, r *http.Request) {
	instID, ok := services.InstitutionIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "not authenticated")
		return
	}
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid id")
		return
	}
	ifMatch := parseIfMatch(r)
	if err := h.scheduling.DeleteSchedulingConfig(r.Context(), instID, id, ifMatch); err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
