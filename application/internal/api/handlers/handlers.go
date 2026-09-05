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
func (h *Handlers) CreateSolveJob(w http.ResponseWriter, r *http.Request) {
	var body struct {
		TimetableID     uuid.UUID  `json:"timetableId"`
		SnapshotID      *uuid.UUID `json:"snapshotId,omitempty"`
		Seed           int64      `json:"seed,omitempty"`
		UseSeed        bool       `json:"useSeed,omitempty"`
		DeadlineSeconds int       `json:"deadlineSeconds,omitempty"`
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

	runID, err := h.slice.CreateSolveJob(r.Context(), services.SolveRequest{
		TimetableID:      body.TimetableID,
		SnapshotID:       body.SnapshotID,
		Seed:             body.Seed,
		UseSeed:          body.UseSeed,
		DeadlineSeconds:  body.DeadlineSeconds,
	}, user.ID)
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
