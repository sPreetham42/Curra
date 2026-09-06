package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/sPreetham42/timetable-platform/application/internal/curra"
	"github.com/sPreetham42/timetable-platform/application/internal/domain"
	"github.com/sPreetham42/timetable-platform/application/internal/services"
)

type Handlers struct {
	timetables   *services.TimetableService
	snapshots    *services.SnapshotService
	runs         *services.RunService
	versions     *services.VersionService
	publishing   *services.PublishingService
	moveSwap     *services.MoveSwapService
	verification *services.VerificationService
	catalog      *services.CatalogService
	slice        *services.SliceService
}

func New(
	timetables *services.TimetableService,
	snapshots *services.SnapshotService,
	runs *services.RunService,
	versions *services.VersionService,
	publishing *services.PublishingService,
	moveSwap *services.MoveSwapService,
	verification *services.VerificationService,
	catalog *services.CatalogService,
	slice *services.SliceService,
) *Handlers {
	return &Handlers{
		timetables:   timetables,
		snapshots:    snapshots,
		runs:         runs,
		versions:     versions,
		publishing:   publishing,
		moveSwap:     moveSwap,
		verification: verification,
		catalog:      catalog,
		slice:        slice,
	}
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"data": data})
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]any{"code": code, "message": message},
	})
}

func handleError(w http.ResponseWriter, err error) {
	if err == nil {
		return
	}
	if errors.Is(err, services.ErrNotFound) {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "resource not found")
		return
	}
	if errors.Is(err, services.ErrUnauthorized) {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "not authenticated")
		return
	}
	if errors.Is(err, services.ErrForbidden) {
		writeError(w, http.StatusForbidden, "FORBIDDEN", "access denied")
		return
	}
	if errors.Is(err, services.ErrPreconditionRequired) {
		writeError(w, http.StatusPreconditionRequired, "PRECONDITION_REQUIRED", "If-Match header is required for mutating operations")
		return
	}
	if errors.Is(err, services.ErrConflict) {
		writeError(w, http.StatusConflict, "CONFLICT", "version mismatch: resource was modified by another transaction")
		return
	}
	if errors.Is(err, services.ErrInvalidState) {
		writeError(w, http.StatusUnprocessableEntity, "INVALID_STATE", err.Error())
		return
	}
	if errors.Is(err, services.ErrValidation) {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_FAILED", err.Error())
		return
	}
	writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
}

func parseIfMatch(r *http.Request) int {
	header := r.Header.Get("If-Match")
	if header == "" {
		return 0
	}
	val, err := strconv.Atoi(header)
	if err != nil {
		return 0
	}
	return val
}

// GET /health
func (h *Handlers) Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

// GET /api/v1/auth/me
func (h *Handlers) GetMe(w http.ResponseWriter, r *http.Request) {
	user, ok := services.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "not authenticated")
		return
	}
	role, _ := services.RoleFromContext(r.Context())
	inst, _ := services.InstitutionFromContext(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{
		"user":        user,
		"institution": inst,
		"role":        role.Role,
	})
}

// POST /api/v1/timetables
func (h *Handlers) CreateTimetable(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "name is required")
		return
	}

	tt, err := h.timetables.Create(r.Context(), req.Name)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, tt)
}

// GET /api/v1/timetables
func (h *Handlers) ListTimetables(w http.ResponseWriter, r *http.Request) {
	tts, err := h.timetables.ListByInstitution(r.Context())
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, tts)
}

// GET /api/v1/timetables/{id}
func (h *Handlers) GetTimetable(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid id")
		return
	}

	tt, err := h.timetables.GetByID(r.Context(), id)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, tt)
}

// PATCH /api/v1/timetables/{id}
func (h *Handlers) UpdateTimetable(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid id")
		return
	}

	ifMatch := parseIfMatch(r)
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "name is required")
		return
	}

	tt, err := h.timetables.Update(r.Context(), id, req.Name, ifMatch)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, tt)
}

// POST /api/v1/timetables/{id}/snapshots
func (h *Handlers) CreateSnapshot(w http.ResponseWriter, r *http.Request) {
	timetableID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid timetable id")
		return
	}

	user, _ := services.UserFromContext(r.Context())
	snap, err := h.snapshots.CreateSnapshot(r.Context(), timetableID, user.ID)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, snap)
}

// GET /api/v1/timetables/{id}/snapshots
func (h *Handlers) ListSnapshots(w http.ResponseWriter, r *http.Request) {
	timetableID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid timetable id")
		return
	}

	snaps, err := h.snapshots.ListSnapshots(r.Context(), timetableID)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, snaps)
}

// GET /api/v1/snapshots/{id}
func (h *Handlers) GetSnapshot(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid snapshot id")
		return
	}

	snap, err := h.snapshots.GetSnapshot(r.Context(), id)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, snap)
}

// GET /api/v1/snapshots/{id}/problem
func (h *Handlers) GetProblemJSON(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid snapshot id")
		return
	}

	snap, err := h.snapshots.GetSnapshot(r.Context(), id)
	if err != nil {
		handleError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(snap.ProblemJSON)
}

// POST /api/v1/timetables/{id}/runs
func (h *Handlers) CreateRun(w http.ResponseWriter, r *http.Request) {
	timetableID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid timetable id")
		return
	}

	var req struct {
		SnapshotID   uuid.UUID       `json:"snapshotId"`
		SolverConfig json.RawMessage `json:"solverConfig,omitempty"`
		Seed         *int64          `json:"seed,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.SnapshotID == uuid.Nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "snapshotId is required")
		return
	}

	idempotencyKey := r.Header.Get("Idempotency-Key")
	user, _ := services.UserFromContext(r.Context())

	run, err := h.runs.CreateRun(r.Context(), timetableID, req.SnapshotID, req.SolverConfig, req.Seed, user.ID, idempotencyKey)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, run)
}

// GET /api/v1/timetables/{id}/runs
func (h *Handlers) ListRuns(w http.ResponseWriter, r *http.Request) {
	timetableID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid timetable id")
		return
	}

	runs, err := h.runs.ListRuns(r.Context(), timetableID)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, runs)
}

// GET /api/v1/runs/{id}
func (h *Handlers) GetRun(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid run id")
		return
	}

	run, err := h.runs.GetRun(r.Context(), id)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, run)
}

// POST /api/v1/runs/{id}/cancel
func (h *Handlers) CancelRun(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid run id")
		return
	}

	run, err := h.runs.CancelRun(r.Context(), id)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, run)
}

// POST /api/v1/timetables/{id}/versions
func (h *Handlers) CreateVersion(w http.ResponseWriter, r *http.Request) {
	timetableID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid timetable id")
		return
	}

	var req struct {
		SnapshotID  uuid.UUID  `json:"snapshotId"`
		SourceRunID *uuid.UUID `json:"sourceRunId,omitempty"`
		Name        string     `json:"name,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.SnapshotID == uuid.Nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "snapshotId is required")
		return
	}

	idempotencyKey := r.Header.Get("Idempotency-Key")
	user, _ := services.UserFromContext(r.Context())

	ver, err := h.versions.CreateVersion(r.Context(), timetableID, req.SourceRunID, req.SnapshotID, req.Name, user.ID, idempotencyKey)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, ver)
}

// GET /api/v1/timetables/{id}/versions
func (h *Handlers) ListVersions(w http.ResponseWriter, r *http.Request) {
	timetableID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid timetable id")
		return
	}

	versions, err := h.versions.ListVersions(r.Context(), timetableID)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, versions)
}

// GET /api/v1/versions/{id}
func (h *Handlers) GetVersion(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid version id")
		return
	}

	ver, assignments, err := h.versions.GetVersion(r.Context(), id)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"version":     ver,
		"assignments": assignments,
	})
}

// GET /api/v1/versions/{id}/assignments
func (h *Handlers) ListAssignments(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid version id")
		return
	}

	_, assignments, err := h.versions.GetVersion(r.Context(), id)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, assignments)
}

// PATCH /api/v1/versions/{id}
func (h *Handlers) UpdateVersionName(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid version id")
		return
	}

	ifMatch := parseIfMatch(r)
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "name is required")
		return
	}

	ver, err := h.versions.UpdateVersionName(r.Context(), id, req.Name, ifMatch)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, ver)
}

// POST /api/v1/versions/{id}/review
func (h *Handlers) SubmitReview(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid version id")
		return
	}

	ifMatch := parseIfMatch(r)
	ver, err := h.versions.SubmitReview(r.Context(), id, ifMatch)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, ver)
}

// POST /api/v1/versions/{id}/send-back
func (h *Handlers) SendBack(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid version id")
		return
	}

	ifMatch := parseIfMatch(r)
	ver, err := h.versions.SendBack(r.Context(), id, ifMatch)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, ver)
}

// POST /api/v1/versions/{id}/publish
func (h *Handlers) PublishVersion(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid version id")
		return
	}

	ifMatch := parseIfMatch(r)
	user, _ := services.UserFromContext(r.Context())
	ver, err := h.publishing.Publish(r.Context(), id, ifMatch, user.ID)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, ver)
}

// POST /api/v1/versions/{id}/archive
func (h *Handlers) ArchiveVersion(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid version id")
		return
	}

	ifMatch := parseIfMatch(r)
	ver, err := h.versions.Archive(r.Context(), id, ifMatch)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, ver)
}

// POST /api/v1/versions/{id}/assignments/move
func (h *Handlers) MoveAssignment(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid version id")
		return
	}

	ifMatch := parseIfMatch(r)
	dryRun := r.URL.Query().Get("dryRun") == "true"

	var req domain.MoveDTO
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.AssignmentID == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "assignmentId and valid placement required")
		return
	}

	resp, updatedVer, err := h.moveSwap.Move(r.Context(), id, req, ifMatch, dryRun)
	if err != nil {
		if errors.Is(err, services.ErrValidation) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnprocessableEntity)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"error":      map[string]any{"code": "VALIDATION_FAILED", "message": err.Error()},
				"validation": resp,
			})
			return
		}
		handleError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"validation": resp,
		"version":    updatedVer,
	})
}

// POST /api/v1/versions/{id}/assignments/swap
func (h *Handlers) SwapAssignments(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid version id")
		return
	}

	ifMatch := parseIfMatch(r)
	dryRun := r.URL.Query().Get("dryRun") == "true"

	var req domain.SwapDTO
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Assignment1ID == "" || req.Assignment2ID == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "assignment1Id and assignment2Id required")
		return
	}

	resp, updatedVer, err := h.moveSwap.Swap(r.Context(), id, req, ifMatch, dryRun)
	if err != nil {
		if errors.Is(err, services.ErrValidation) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnprocessableEntity)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"error":      map[string]any{"code": "VALIDATION_FAILED", "message": err.Error()},
				"validation": resp,
			})
			return
		}
		handleError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"validation": resp,
		"version":    updatedVer,
	})
}

// POST /api/v1/verify
func (h *Handlers) Verify(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SnapshotID uuid.UUID `json:"snapshotId"`
		VersionID  uuid.UUID `json:"versionId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.SnapshotID == uuid.Nil || req.VersionID == uuid.Nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "snapshotId and versionId are required")
		return
	}

	resp, err := h.verification.VerifyVersion(r.Context(), req.SnapshotID, req.VersionID)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// Academic Catalogs
func (h *Handlers) ListDepartments(w http.ResponseWriter, r *http.Request) {
	instID, ok := services.InstitutionIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "not authenticated")
		return
	}
	items, err := h.catalog.ListDepartments(r.Context(), instID)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (h *Handlers) CreateDepartment(w http.ResponseWriter, r *http.Request) {
	instID, ok := services.InstitutionIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "not authenticated")
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "name is required")
		return
	}
	item, err := h.catalog.CreateDepartment(r.Context(), instID, req.Name)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (h *Handlers) ListPrograms(w http.ResponseWriter, r *http.Request) {
	instID, ok := services.InstitutionIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "not authenticated")
		return
	}
	items, err := h.catalog.ListPrograms(r.Context(), instID)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (h *Handlers) ListClasses(w http.ResponseWriter, r *http.Request) {
	instID, ok := services.InstitutionIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "not authenticated")
		return
	}
	items, err := h.catalog.ListClasses(r.Context(), instID)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (h *Handlers) ListStudentGroups(w http.ResponseWriter, r *http.Request) {
	instID, ok := services.InstitutionIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "not authenticated")
		return
	}
	items, err := h.catalog.ListStudentGroups(r.Context(), instID)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (h *Handlers) ListSubjects(w http.ResponseWriter, r *http.Request) {
	instID, ok := services.InstitutionIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "not authenticated")
		return
	}
	items, err := h.catalog.ListSubjects(r.Context(), instID)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (h *Handlers) ListFaculty(w http.ResponseWriter, r *http.Request) {
	instID, ok := services.InstitutionIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "not authenticated")
		return
	}
	items, err := h.catalog.ListFaculty(r.Context(), instID)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (h *Handlers) ListRooms(w http.ResponseWriter, r *http.Request) {
	instID, ok := services.InstitutionIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "not authenticated")
		return
	}
	items, err := h.catalog.ListRooms(r.Context(), instID)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (h *Handlers) ListTimeSlots(w http.ResponseWriter, r *http.Request) {
	instID, ok := services.InstitutionIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "not authenticated")
		return
	}
	items, err := h.catalog.ListTimeSlots(r.Context(), instID)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

// Academic Years
func (h *Handlers) ListAcademicYears(w http.ResponseWriter, r *http.Request) {
	instID, ok := services.InstitutionIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "not authenticated")
		return
	}
	items, err := h.catalog.ListAcademicYears(r.Context(), instID)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (h *Handlers) CreateAcademicYear(w http.ResponseWriter, r *http.Request) {
	instID, ok := services.InstitutionIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "not authenticated")
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "name is required")
		return
	}
	item, err := h.catalog.CreateAcademicYear(r.Context(), instID, req.Name)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

// Terms
func (h *Handlers) ListTerms(w http.ResponseWriter, r *http.Request) {
	instID, ok := services.InstitutionIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "not authenticated")
		return
	}
	items, err := h.catalog.ListTerms(r.Context(), instID)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (h *Handlers) CreateTerm(w http.ResponseWriter, r *http.Request) {
	instID, ok := services.InstitutionIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "not authenticated")
		return
	}
	var req struct {
		AcademicYearID uuid.UUID `json:"academicYearId"`
		Name           string    `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" || req.AcademicYearID == uuid.Nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "academicYearId and name are required")
		return
	}
	item, err := h.catalog.CreateTerm(r.Context(), instID, req.AcademicYearID, req.Name)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

// Time Slots
func (h *Handlers) CreateTimeSlot(w http.ResponseWriter, r *http.Request) {
	instID, ok := services.InstitutionIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "not authenticated")
		return
	}
	var req struct {
		Day    string `json:"day"`
		Period int    `json:"period"`
		Label  string `json:"label"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Day == "" || req.Period <= 0 || req.Label == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "day, period, and label are required")
		return
	}
	item, err := h.catalog.CreateTimeSlot(r.Context(), instID, req.Day, req.Period, req.Label)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

// Programs
func (h *Handlers) CreateProgram(w http.ResponseWriter, r *http.Request) {
	instID, ok := services.InstitutionIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "not authenticated")
		return
	}
	var req struct {
		DepartmentID uuid.UUID `json:"departmentId"`
		Name         string    `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" || req.DepartmentID == uuid.Nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "departmentId and name are required")
		return
	}
	item, err := h.catalog.CreateProgram(r.Context(), instID, req.DepartmentID, req.Name)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

// Classes
func (h *Handlers) CreateClass(w http.ResponseWriter, r *http.Request) {
	instID, ok := services.InstitutionIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "not authenticated")
		return
	}
	var req struct {
		ProgramID uuid.UUID `json:"programId"`
		Name      string    `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" || req.ProgramID == uuid.Nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "programId and name are required")
		return
	}
	item, err := h.catalog.CreateClass(r.Context(), instID, req.ProgramID, req.Name)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

// Student Groups
func (h *Handlers) CreateStudentGroup(w http.ResponseWriter, r *http.Request) {
	instID, ok := services.InstitutionIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "not authenticated")
		return
	}
	var req struct {
		ClassID     uuid.UUID `json:"classId"`
		Name        string    `json:"name"`
		Size        int       `json:"size"`
		IsWhole     bool      `json:"isWhole"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" || req.ClassID == uuid.Nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "classId, name and size are required")
		return
	}
	item, err := h.catalog.CreateStudentGroup(r.Context(), instID, req.ClassID, req.Name, req.Size, req.IsWhole)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

// Subjects
func (h *Handlers) CreateSubject(w http.ResponseWriter, r *http.Request) {
	instID, ok := services.InstitutionIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "not authenticated")
		return
	}
	var req struct {
		Code string `json:"code"`
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Code == "" || req.Name == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "code and name are required")
		return
	}
	item, err := h.catalog.CreateSubject(r.Context(), instID, req.Code, req.Name)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

// Faculty
func (h *Handlers) CreateFaculty(w http.ResponseWriter, r *http.Request) {
	instID, ok := services.InstitutionIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "not authenticated")
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "name is required")
		return
	}
	item, err := h.catalog.CreateFaculty(r.Context(), instID, req.Name)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

// Rooms
func (h *Handlers) CreateRoom(w http.ResponseWriter, r *http.Request) {
	instID, ok := services.InstitutionIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "not authenticated")
		return
	}
	var req struct {
		Name     string `json:"name"`
		Capacity int    `json:"capacity"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "name is required")
		return
	}
	item, err := h.catalog.CreateRoom(r.Context(), instID, req.Name, req.Capacity)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

// Room Features
func (h *Handlers) ListRoomFeatures(w http.ResponseWriter, r *http.Request) {
	instID, ok := services.InstitutionIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "not authenticated")
		return
	}
	items, err := h.catalog.ListRoomFeatures(r.Context(), instID)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (h *Handlers) CreateRoomFeature(w http.ResponseWriter, r *http.Request) {
	instID, ok := services.InstitutionIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "not authenticated")
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "name is required")
		return
	}
	item, err := h.catalog.CreateRoomFeature(r.Context(), instID, req.Name)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

// Room Feature Assignments
func (h *Handlers) GetRoomFeatures(w http.ResponseWriter, r *http.Request) {
	instID, ok := services.InstitutionIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "not authenticated")
		return
	}
	roomID, err := uuid.Parse(r.PathValue("roomId"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid roomId")
		return
	}
	items, err := h.catalog.GetRoomFeatures(r.Context(), instID, roomID)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (h *Handlers) AssignRoomFeature(w http.ResponseWriter, r *http.Request) {
	instID, ok := services.InstitutionIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "not authenticated")
		return
	}
	roomID, err := uuid.Parse(r.PathValue("roomId"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid roomId")
		return
	}
	var req struct {
		FeatureID uuid.UUID `json:"featureId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.FeatureID == uuid.Nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "featureId is required")
		return
	}
	if err := h.catalog.AssignRoomFeature(r.Context(), instID, roomID, req.FeatureID); err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusNoContent, nil)
}

func (h *Handlers) UnassignRoomFeature(w http.ResponseWriter, r *http.Request) {
	instID, ok := services.InstitutionIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "not authenticated")
		return
	}
	roomID, err := uuid.Parse(r.PathValue("roomId"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid roomId")
		return
	}
	featureID, err := uuid.Parse(r.PathValue("featureId"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid featureId")
		return
	}
	if err := h.catalog.UnassignRoomFeature(r.Context(), instID, roomID, featureID); err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusNoContent, nil)
}

// Course Offerings
func (h *Handlers) ListCourseOfferings(w http.ResponseWriter, r *http.Request) {
	instID, ok := services.InstitutionIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "not authenticated")
		return
	}
	items, err := h.catalog.ListCourseOfferings(r.Context(), instID)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (h *Handlers) CreateCourseOffering(w http.ResponseWriter, r *http.Request) {
	instID, ok := services.InstitutionIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "not authenticated")
		return
	}
	var req struct {
		TermID         uuid.UUID `json:"termId"`
		ClassID        uuid.UUID `json:"classId"`
		SubjectID      uuid.UUID `json:"subjectId"`
		StudentGroupID uuid.UUID `json:"studentGroupId"`
		FacultyID      uuid.UUID `json:"facultyId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.TermID == uuid.Nil || req.ClassID == uuid.Nil || req.SubjectID == uuid.Nil || req.StudentGroupID == uuid.Nil || req.FacultyID == uuid.Nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "termId, classId, subjectId, studentGroupId, and facultyId are required")
		return
	}
	item, err := h.catalog.CreateCourseOffering(r.Context(), instID, req.TermID, req.ClassID, req.SubjectID, req.StudentGroupID, req.FacultyID)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

// Session Requirements
func (h *Handlers) CreateSessionRequirement(w http.ResponseWriter, r *http.Request) {
	instID, ok := services.InstitutionIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "not authenticated")
		return
	}
	var req struct {
		CourseOfferingID uuid.UUID `json:"courseOfferingId"`
		Type             string    `json:"type"` // THEORY or LAB
		SessionsPerWeek  int       `json:"sessionsPerWeek"`
		Duration         int       `json:"duration"`
		Consecutive      bool      `json:"consecutive"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.CourseOfferingID == uuid.Nil || req.Type == "" || req.SessionsPerWeek <= 0 || req.Duration <= 0 {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "courseOfferingId, type (THEORY|LAB), sessionsPerWeek, and duration are required")
		return
	}
	item, err := h.catalog.CreateSessionRequirement(r.Context(), instID, req.CourseOfferingID, req.Type, req.SessionsPerWeek, req.Duration, req.Consecutive)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

// Faculty Availability
func (h *Handlers) ListFacultyAvailability(w http.ResponseWriter, r *http.Request) {
	instID, ok := services.InstitutionIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "not authenticated")
		return
	}
	items, err := h.catalog.ListFacultyAvailability(r.Context(), instID)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (h *Handlers) CreateFacultyAvailability(w http.ResponseWriter, r *http.Request) {
	instID, ok := services.InstitutionIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "not authenticated")
		return
	}
	var req struct {
		FacultyID  uuid.UUID `json:"facultyId"`
		TimeSlotID uuid.UUID `json:"timeSlotId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.FacultyID == uuid.Nil || req.TimeSlotID == uuid.Nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "facultyId and timeSlotId are required")
		return
	}
	item, err := h.catalog.CreateFacultyAvailability(r.Context(), instID, req.FacultyID, req.TimeSlotID)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (h *Handlers) DeleteFacultyAvailability(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid id")
		return
	}
	if err := h.catalog.DeleteFacultyAvailability(r.Context(), id); err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// Room Availability
func (h *Handlers) ListRoomAvailability(w http.ResponseWriter, r *http.Request) {
	instID, ok := services.InstitutionIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "not authenticated")
		return
	}
	items, err := h.catalog.ListRoomAvailability(r.Context(), instID)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (h *Handlers) CreateRoomAvailability(w http.ResponseWriter, r *http.Request) {
	instID, ok := services.InstitutionIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "not authenticated")
		return
	}
	var req struct {
		RoomID     uuid.UUID `json:"roomId"`
		TimeSlotID uuid.UUID `json:"timeSlotId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.RoomID == uuid.Nil || req.TimeSlotID == uuid.Nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "roomId and timeSlotId are required")
		return
	}
	item, err := h.catalog.CreateRoomAvailability(r.Context(), instID, req.RoomID, req.TimeSlotID)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (h *Handlers) DeleteRoomAvailability(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid id")
		return
	}
	if err := h.catalog.DeleteRoomAvailability(r.Context(), id); err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// Faculty Preferences
func (h *Handlers) ListFacultyPreferences(w http.ResponseWriter, r *http.Request) {
	instID, ok := services.InstitutionIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "not authenticated")
		return
	}
	items, err := h.catalog.ListFacultyPreferences(r.Context(), instID)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (h *Handlers) CreateFacultyPreference(w http.ResponseWriter, r *http.Request) {
	instID, ok := services.InstitutionIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "not authenticated")
		return
	}
	var req struct {
		FacultyID  uuid.UUID `json:"facultyId"`
		TimeSlotID uuid.UUID `json:"timeSlotId"`
		Weight     int       `json:"weight"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.FacultyID == uuid.Nil || req.TimeSlotID == uuid.Nil || req.Weight <= 0 {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "facultyId, timeSlotId, and weight (positive integer) are required")
		return
	}
	item, err := h.catalog.CreateFacultyPreference(r.Context(), instID, req.FacultyID, req.TimeSlotID, req.Weight)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (h *Handlers) DeleteFacultyPreference(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid id")
		return
	}
	if err := h.catalog.DeleteFacultyPreference(r.Context(), id); err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// CompileConstraints endpoint
func (h *Handlers) CompileConstraints(w http.ResponseWriter, r *http.Request) {
	var req curra.CompileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid request body")
		return
	}
	// compile via adapter (can be added if exposed via verification service or direct adapter)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// POST /api/v1/solve-jobs
//
// Submits an asynchronous solve job. Returns immediately with 202 Accepted
// and the run ID. The job is processed by the background worker.
// Poll GET /api/v1/solve-jobs/{id} to observe state, or read the full
// result via GET /api/v1/solve-jobs/{id}/result.
// Supports Idempotency-Key header for duplicate request deduplication.
func (h *Handlers) CreateSolveJob(w http.ResponseWriter, r *http.Request) {
	var body struct {
		TimetableID     uuid.UUID  `json:"timetableId"`
		SnapshotID      *uuid.UUID `json:"snapshotId,omitempty"`
		Seed            int64      `json:"seed,omitempty"`
		UseSeed         bool       `json:"useSeed,omitempty"`
		DeadlineSeconds int        `json:"deadlineSeconds,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid request body")
		return
	}
	if body.TimetableID == uuid.Nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "timetableId is required")
		return
	}
	user, _ := services.UserFromContext(r.Context())
	if user.ID == uuid.Nil {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "not authenticated")
		return
	}

	idempotencyKey := r.Header.Get("Idempotency-Key")

	runID, err := h.slice.CreateSolveJob(r.Context(), services.SolveRequest{
		TimetableID:      body.TimetableID,
		SnapshotID:       body.SnapshotID,
		Seed:             body.Seed,
		UseSeed:          body.UseSeed,
		DeadlineSeconds:  body.DeadlineSeconds,
	}, user.ID, idempotencyKey)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{
		"runId": runID,
	})
}

// GET /api/v1/solve-jobs/{id}
func (h *Handlers) GetSolveJob(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid job id")
		return
	}
	job, err := h.slice.GetJob(r.Context(), id)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, job)
}

// GET /api/v1/solve-jobs/{id}/result
func (h *Handlers) GetSolveJobResult(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid job id")
		return
	}
	result, err := h.slice.GetResult(r.Context(), id)
	if err != nil {
		handleError(w, err)
		return
	}
	if !result.Verified {
		writeError(w, http.StatusUnprocessableEntity, "VERIFICATION_FAILED",
			"engine result failed independent verification")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// POST /api/v1/solve-jobs/{id}/cancel
func (h *Handlers) CancelSolveJob(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid job id")
		return
	}
	err = h.slice.CancelSolveJob(r.Context(), id)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "cancelled"})
}

// GET /api/v1/engine-snapshots/{id}
func (h *Handlers) GetEngineSnapshot(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid snapshot id")
		return
	}

	snap, err := h.slice.GetEngineSnapshot(r.Context(), id)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, snap)
}
