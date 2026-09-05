package api

import (
	"net/http"

	"github.com/sPreetham42/timetable-platform/application/internal/api/handlers"
	"github.com/sPreetham42/timetable-platform/application/internal/api/middleware"
)

// NewRouter constructs the complete HTTP handler mux with routing and authentication.
func NewRouter(h *handlers.Handlers, auth *middleware.AuthMiddleware) http.Handler {
	mux := http.NewServeMux()

	// Public routes
	mux.HandleFunc("GET /health", h.Health)

	// Protected routes (sub-mux wrapped with AuthMiddleware)
	protectedMux := http.NewServeMux()

	// Auth
	protectedMux.HandleFunc("GET /api/v1/auth/me", h.GetMe)

	// Timetables
	protectedMux.HandleFunc("POST /api/v1/timetables", h.CreateTimetable)
	protectedMux.HandleFunc("GET /api/v1/timetables", h.ListTimetables)
	protectedMux.HandleFunc("GET /api/v1/timetables/{id}", h.GetTimetable)
	protectedMux.HandleFunc("PATCH /api/v1/timetables/{id}", h.UpdateTimetable)

	// Snapshots
	protectedMux.HandleFunc("POST /api/v1/timetables/{id}/snapshots", h.CreateSnapshot)
	protectedMux.HandleFunc("GET /api/v1/timetables/{id}/snapshots", h.ListSnapshots)
	protectedMux.HandleFunc("GET /api/v1/snapshots/{id}", h.GetSnapshot)
	protectedMux.HandleFunc("GET /api/v1/snapshots/{id}/problem", h.GetProblemJSON)

	// Runs
	protectedMux.HandleFunc("POST /api/v1/timetables/{id}/runs", h.CreateRun)
	protectedMux.HandleFunc("GET /api/v1/timetables/{id}/runs", h.ListRuns)
	protectedMux.HandleFunc("GET /api/v1/runs/{id}", h.GetRun)
	protectedMux.HandleFunc("POST /api/v1/runs/{id}/cancel", h.CancelRun)

	// Versions
	protectedMux.HandleFunc("POST /api/v1/timetables/{id}/versions", h.CreateVersion)
	protectedMux.HandleFunc("GET /api/v1/timetables/{id}/versions", h.ListVersions)
	protectedMux.HandleFunc("GET /api/v1/versions/{id}", h.GetVersion)
	protectedMux.HandleFunc("GET /api/v1/versions/{id}/assignments", h.ListAssignments)
	protectedMux.HandleFunc("PATCH /api/v1/versions/{id}", h.UpdateVersionName)
	protectedMux.HandleFunc("POST /api/v1/versions/{id}/submit-review", h.SubmitReview)
	protectedMux.HandleFunc("POST /api/v1/versions/{id}/review", h.SubmitReview) // Alias for contract compatibility
	protectedMux.HandleFunc("POST /api/v1/versions/{id}/send-back", h.SendBack)
	protectedMux.HandleFunc("POST /api/v1/versions/{id}/publish", h.PublishVersion)
	protectedMux.HandleFunc("POST /api/v1/versions/{id}/archive", h.ArchiveVersion)

	// Manual Edits (Move & Swap)
	protectedMux.HandleFunc("POST /api/v1/versions/{id}/assignments/move", h.MoveAssignment)
	protectedMux.HandleFunc("POST /api/v1/versions/{id}/assignments/swap", h.SwapAssignments)

	// Verification
	protectedMux.HandleFunc("POST /api/v1/verify", h.Verify)

	// Phase 1 Vertical Slice — minimal end-to-end solve job API
	protectedMux.HandleFunc("POST /api/v1/solve-jobs", h.CreateSolveJob)
	protectedMux.HandleFunc("GET /api/v1/solve-jobs/{id}", h.GetSolveJob)
	protectedMux.HandleFunc("GET /api/v1/solve-jobs/{id}/result", h.GetSolveJobResult)
	protectedMux.HandleFunc("POST /api/v1/solve-jobs/{id}/cancel", h.CancelSolveJob)

	// Academic Catalog (Direct Routes)
	protectedMux.HandleFunc("GET /api/v1/departments", h.ListDepartments)
	protectedMux.HandleFunc("POST /api/v1/departments", h.CreateDepartment)
	protectedMux.HandleFunc("GET /api/v1/programs", h.ListPrograms)
	protectedMux.HandleFunc("GET /api/v1/classes", h.ListClasses)
	protectedMux.HandleFunc("GET /api/v1/student-groups", h.ListStudentGroups)
	protectedMux.HandleFunc("GET /api/v1/subjects", h.ListSubjects)
	protectedMux.HandleFunc("GET /api/v1/faculty", h.ListFaculty)
	protectedMux.HandleFunc("GET /api/v1/rooms", h.ListRooms)
	protectedMux.HandleFunc("GET /api/v1/time-slots", h.ListTimeSlots)

	// Academic Catalog (OpenAPI /institutions/{instId}/... Compatibility Routes)
	protectedMux.HandleFunc("GET /api/v1/institutions/{instId}/departments", h.ListDepartments)
	protectedMux.HandleFunc("POST /api/v1/institutions/{instId}/departments", h.CreateDepartment)
	protectedMux.HandleFunc("GET /api/v1/institutions/{instId}/programs", h.ListPrograms)
	protectedMux.HandleFunc("GET /api/v1/institutions/{instId}/classes", h.ListClasses)
	protectedMux.HandleFunc("GET /api/v1/institutions/{instId}/student-groups", h.ListStudentGroups)
	protectedMux.HandleFunc("GET /api/v1/institutions/{instId}/subjects", h.ListSubjects)
	protectedMux.HandleFunc("GET /api/v1/institutions/{instId}/faculty", h.ListFaculty)
	protectedMux.HandleFunc("GET /api/v1/institutions/{instId}/rooms", h.ListRooms)
	protectedMux.HandleFunc("GET /api/v1/institutions/{instId}/time-slots", h.ListTimeSlots)

	// Mount protected routes behind AuthMiddleware
	mux.Handle("/api/", auth.Handle(protectedMux))

	return corsMiddleware(mux)
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "http://localhost:5173" || origin == "http://127.0.0.1:5173" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, If-Match, X-Institution-ID, X-User-ID, X-Role")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
