package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/sPreetham42/timetable-platform/application/internal/database/repositories"
	"github.com/sPreetham42/timetable-platform/application/internal/domain"
)

type SnapshotService struct {
	repos *repositories.Repos
}

func NewSnapshotService(repos *repositories.Repos) *SnapshotService {
	return &SnapshotService{repos: repos}
}

func (s *SnapshotService) CreateSnapshot(ctx context.Context, timetableID uuid.UUID, createdBy uuid.UUID) (domain.ProblemSnapshot, error) {
	tt, err := s.repos.Timetables.GetByID(ctx, timetableID)
	if err != nil {
		return domain.ProblemSnapshot{}, err
	}
	if err := RequireTenantMatch(ctx, tt.InstitutionID); err != nil {
		return domain.ProblemSnapshot{}, err
	}
	if err := RequireRole(ctx, domain.RoleInstitutionAdmin, domain.RoleScheduler); err != nil {
		return domain.ProblemSnapshot{}, err
	}

	instID := tt.InstitutionID

	// Load live academic catalog for this institution
	depts, _ := s.repos.Departments.ListByInstitution(ctx, instID)
	progs, _ := s.repos.Programs.ListByInstitution(ctx, instID)
	cls, _ := s.repos.Classes.ListByInstitution(ctx, instID)
	sgs, _ := s.repos.StudentGroups.ListByInstitution(ctx, instID)
	subj, _ := s.repos.Subjects.ListByInstitution(ctx, instID)
	fac, _ := s.repos.Faculty.ListByInstitution(ctx, instID)
	rooms, _ := s.repos.Rooms.ListByInstitution(ctx, instID)
	rf, _ := s.repos.RoomFeatures.ListByInstitution(ctx, instID)
	ts, _ := s.repos.TimeSlots.ListByInstitution(ctx, instID)
	terms, _ := s.repos.Terms.ListByInstitution(ctx, instID)
	cos, _ := s.repos.CourseOfferings.ListByInstitution(ctx, instID)
	srs, _ := s.repos.SessionRequirements.ListByInstitution(ctx, instID)
	faList, _ := s.repos.FacultyAvailability.ListByInstitution(ctx, instID)
	raList, _ := s.repos.RoomAvailability.ListByInstitution(ctx, instID)
	fpList, _ := s.repos.FacultyPreferences.ListByInstitution(ctx, instID)

	termName := "Term 1"
	termID := "term-1"
	if len(terms) > 0 {
		termID = terms[0].ID.String()
		termName = terms[0].Name
	}

	// Build canonical problem payload matching CURRA model
	problemData := map[string]any{
		"TenantID":      instID.String(),
		"PeriodsPerDay": 8,
		"Term": map[string]any{
			"ID":       termID,
			"TenantID": instID.String(),
			"Name":     termName,
		},
		"Departments":           mapFromDepts(depts),
		"Programs":              mapFromProgs(progs),
		"Classes":               mapFromClasses(cls, sgs),
		"StudentGroups":         mapFromSGs(sgs),
		"Subjects":              mapFromSubjs(subj),
		"Faculty":               mapFromFac(fac),
		"Rooms":                 mapFromRooms(rooms),
		"RoomFeatures":          mapFromRF(rf),
		"TimeSlots":             mapFromTS(ts),
		"CourseOfferings":       mapFromCOs(cos, srs),
		"SessionRequirements":   mapFromSRs(srs),
		"FacultyAvailabilities": mapFromFA(faList),
		"RoomAvailabilities":    mapFromRA(raList),
		"FacultyPreferences":    mapFromFP(fpList),
		"LockedAssignments":     []any{},
	}

	problemJSON, err := json.Marshal(problemData)
	if err != nil {
		return domain.ProblemSnapshot{}, fmt.Errorf("marshal problem JSON: %w", err)
	}

	constraintJSON := json.RawMessage("[]")
	solverConfigJSON := json.RawMessage(`{"searchMode":"HEURISTIC_LCV","maxNodes":100000}`)
	objectiveConfigJSON := json.RawMessage(`{"components":[{"id":"StudentGapPenalty","weight":1}]}`)

	// Compute input hash
	h := sha256.New()
	h.Write(problemJSON)
	h.Write(constraintJSON)
	h.Write(solverConfigJSON)
	h.Write(objectiveConfigJSON)
	inputHash := hex.EncodeToString(h.Sum(nil))

	snap := domain.ProblemSnapshot{
		ID:                  uuid.New(),
		TimetableID:         timetableID,
		InstitutionID:       instID,
		SchemaVersion:       1,
		ProblemJSON:         problemJSON,
		ConstraintInstances: constraintJSON,
		SolverConfig:        solverConfigJSON,
		ObjectiveConfig:     objectiveConfigJSON,
		InputHash:           inputHash,
		CreatedBy:           createdBy,
		CreatedAt:           time.Now(),
	}

	if err := s.repos.Snapshots.Create(ctx, snap); err != nil {
		return domain.ProblemSnapshot{}, fmt.Errorf("persist snapshot: %w", err)
	}

	return snap, nil
}

func (s *SnapshotService) GetSnapshot(ctx context.Context, id uuid.UUID) (domain.ProblemSnapshot, error) {
	snap, err := s.repos.Snapshots.GetByID(ctx, id)
	if err != nil {
		return domain.ProblemSnapshot{}, err
	}
	if err := RequireTenantMatch(ctx, snap.InstitutionID); err != nil {
		return domain.ProblemSnapshot{}, err
	}
	return snap, nil
}

func (s *SnapshotService) ListSnapshots(ctx context.Context, timetableID uuid.UUID) ([]domain.ProblemSnapshot, error) {
	tt, err := s.repos.Timetables.GetByID(ctx, timetableID)
	if err != nil {
		return nil, err
	}
	if err := RequireTenantMatch(ctx, tt.InstitutionID); err != nil {
		return nil, err
	}
	return s.repos.Snapshots.ListByTimetable(ctx, timetableID)
}

func (s *SnapshotService) GetProblemJSON(ctx context.Context, id uuid.UUID) ([]byte, error) {
	snap, err := s.GetSnapshot(ctx, id)
	if err != nil {
		return nil, err
	}
	return snap.ProblemJSON, nil
}

// Helpers for catalog transformation to CURRA model structure
func mapFromDepts(items []domain.Department) map[string]any {
	res := make(map[string]any, len(items))
	for _, d := range items {
		res[d.ID.String()] = map[string]any{"ID": d.ID.String(), "TenantID": d.InstitutionID.String(), "Name": d.Name}
	}
	return res
}

func mapFromProgs(items []domain.Program) map[string]any {
	res := make(map[string]any, len(items))
	for _, p := range items {
		res[p.ID.String()] = map[string]any{"ID": p.ID.String(), "DepartmentID": p.DepartmentID.String(), "Name": p.Name}
	}
	return res
}

func mapFromClasses(items []domain.Class, sgs []domain.StudentGroup) map[string]any {
	res := make(map[string]any, len(items))
	for _, c := range items {
		var sgIDs []string
		var wholeGroupID string
		for _, g := range sgs {
			if g.ClassID == c.ID {
				sgIDs = append(sgIDs, g.ID.String())
				if g.IsWholeGroup || wholeGroupID == "" {
					wholeGroupID = g.ID.String()
				}
			}
		}
		if sgIDs == nil {
			sgIDs = []string{}
		}
		res[c.ID.String()] = map[string]any{
			"ID":              c.ID.String(),
			"ProgramID":       c.ProgramID.String(),
			"Name":            c.Name,
			"WholeGroupID":    wholeGroupID,
			"StudentGroupIDs": sgIDs,
		}
	}
	return res
}

func mapFromSGs(items []domain.StudentGroup) map[string]any {
	res := make(map[string]any, len(items))
	for _, g := range items {
		res[g.ID.String()] = map[string]any{
			"ID":      g.ID.String(),
			"ClassID": g.ClassID.String(),
			"Name":    g.Name,
			"Size":    g.Size,
		}
	}
	return res
}

func mapFromSubjs(items []domain.Subject) map[string]any {
	res := make(map[string]any, len(items))
	for _, s := range items {
		res[s.ID.String()] = map[string]any{
			"ID":   s.ID.String(),
			"Code": s.Code,
			"Name": s.Name,
		}
	}
	return res
}

func mapFromFac(items []domain.Faculty) map[string]any {
	res := make(map[string]any, len(items))
	for _, f := range items {
		res[f.ID.String()] = map[string]any{
			"ID":       f.ID.String(),
			"TenantID": f.InstitutionID.String(),
			"Name":     f.Name,
		}
	}
	return res
}

func mapFromRooms(items []domain.Room) map[string]any {
	res := make(map[string]any, len(items))
	for _, r := range items {
		res[r.ID.String()] = map[string]any{
			"ID":         r.ID.String(),
			"TenantID":   r.InstitutionID.String(),
			"Name":       r.Name,
			"Capacity":   r.Capacity,
			"FeatureIDs": []string{},
		}
	}
	return res
}

func mapFromRF(items []domain.RoomFeature) map[string]any {
	res := make(map[string]any, len(items))
	for _, rf := range items {
		res[rf.ID.String()] = map[string]any{
			"ID":   rf.ID.String(),
			"Name": rf.Name,
		}
	}
	return res
}

func mapFromTS(items []domain.TimeSlot) map[string]any {
	res := make(map[string]any, len(items))
	for _, t := range items {
		res[t.ID.String()] = map[string]any{
			"ID":     t.ID.String(),
			"Day":    parseDay(t.Day),
			"Period": t.Period,
			"Label":  t.Label,
		}
	}
	return res
}

func parseDay(dayStr string) int {
	switch strings.ToLower(dayStr) {
	case "sunday", "sun", "0":
		return 0
	case "monday", "mon", "1":
		return 1
	case "tuesday", "tue", "tues", "2":
		return 2
	case "wednesday", "wed", "3":
		return 3
	case "thursday", "thu", "thur", "thurs", "4":
		return 4
	case "friday", "fri", "5":
		return 5
	case "saturday", "sat", "6":
		return 6
	default:
		return 1
	}
}

func mapFromCOs(items []domain.CourseOffering, srs []domain.SessionRequirement) map[string]any {
	res := make(map[string]any, len(items))
	for _, co := range items {
		var srIDs []string
		for _, sr := range srs {
			if sr.CourseOfferingID == co.ID {
				srIDs = append(srIDs, sr.ID.String())
			}
		}
		if srIDs == nil {
			srIDs = []string{}
		}
		res[co.ID.String()] = map[string]any{
			"ID":                     co.ID.String(),
			"TermID":                 co.TermID.String(),
			"ClassID":                co.ClassID.String(),
			"SubjectID":              co.SubjectID.String(),
			"StudentGroupID":         co.StudentGroupID.String(),
			"FacultyID":              co.FacultyID.String(),
			"RequiredRoomFeatureIDs": []string{},
			"SessionRequirementIDs":  srIDs,
		}
	}
	return res
}

func mapFromSRs(items []domain.SessionRequirement) map[string]any {
	res := make(map[string]any, len(items))
	for _, sr := range items {
		res[sr.ID.String()] = map[string]any{
			"ID":                     sr.ID.String(),
			"CourseOfferingID":       sr.CourseOfferingID.String(),
			"Type":                   sr.Type,
			"SessionsPerWeek":        sr.SessionsPerWeek,
			"Duration":               sr.Duration,
			"Consecutive":            sr.Consecutive,
			"RequiredRoomFeatureIDs": []string{},
		}
	}
	return res
}

func mapFromFA(items []domain.FacultyAvailability) []any {
	res := make([]any, len(items))
	for i, a := range items {
		res[i] = map[string]any{"FacultyID": a.FacultyID.String(), "TimeSlotID": a.TimeSlotID.String()}
	}
	return res
}

func mapFromRA(items []domain.RoomAvailability) []any {
	res := make([]any, len(items))
	for i, a := range items {
		res[i] = map[string]any{"RoomID": a.RoomID.String(), "TimeSlotID": a.TimeSlotID.String()}
	}
	return res
}

func mapFromFP(items []domain.FacultyPreference) []any {
	res := make([]any, len(items))
	for i, a := range items {
		res[i] = map[string]any{"FacultyID": a.FacultyID.String(), "TimeSlotID": a.TimeSlotID.String(), "Weight": a.Weight}
	}
	return res
}
