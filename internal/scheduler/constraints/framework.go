package constraints

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/sPreetham42/timetable-platform/internal/scheduler/diagnostics"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/model"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/problem"
)

type ConstraintKind string

const (
	ConstraintKindHard ConstraintKind = "HARD"
	ConstraintKindSoft ConstraintKind = "SOFT"
)

// SearchCtx provides contextual execution data for constraint evaluation.
type SearchCtx struct {
	Problem    *problem.Problem
	Membership MembershipIndex
}

// NewSearchCtx creates a SearchCtx with hierarchy-backed membership index.
func NewSearchCtx(p *problem.Problem) *SearchCtx {
	return &SearchCtx{
		Problem:    p,
		Membership: NewHierarchyMembershipIndex(p),
	}
}

// ConstraintDef defines the interface for compiled configurable constraints.
type ConstraintDef interface {
	ID() string
	Kind() ConstraintKind
	IsConsistent(ctx *SearchCtx, partial *problem.Solution, candidate problem.Assignment) bool
	ViolatedByMove(ctx *SearchCtx, sol *problem.Solution, mv problem.Move) []diagnostics.Violation
	Evaluate(ctx *SearchCtx, sol *problem.Solution) []diagnostics.Violation
}

// ConstraintInstance represents an uncompiled constraint rule declaration.
type ConstraintInstance struct {
	ID         string         `json:"id"`
	TemplateID string         `json:"templateId"`
	Scope      string         `json:"scope"`
	Params     map[string]any `json:"params"`
	Kind       ConstraintKind `json:"kind"`
	Weight     int            `json:"weight"`
}

// CompiledConstraintSet holds compiled constraints and rule set hash.
type CompiledConstraintSet struct {
	Constraints []ConstraintDef
	Hard        []ConstraintDef
	Soft        []ConstraintDef
	RuleSetHash string
}

// CompileError captures structured details of an invalid constraint instance.
type CompileError struct {
	TemplateID    string `json:"templateId"`
	InstanceIndex int    `json:"instanceIndex"`
	Field         string `json:"field"`
	Message       string `json:"message"`
}

func (e CompileError) Error() string {
	return fmt.Sprintf("compile error for instance [%d] (%s), field '%s': %s",
		e.InstanceIndex, e.TemplateID, e.Field, e.Message)
}

type CompileErrors []CompileError

func (es CompileErrors) Error() string {
	var msgs []string
	for _, e := range es {
		msgs = append(msgs, e.Error())
	}
	return strings.Join(msgs, "; ")
}

// Compile compiles constraint instances into a CompiledConstraintSet and returns (set, hash, compileErrors).
// - p != nil: performs syntax and parameter validation and cross-validates entity references against the problem catalog.
// - p == nil: performs syntax and parameter validation only without catalog cross-validation.
func Compile(p *problem.Problem, instances []ConstraintInstance) (*CompiledConstraintSet, string, []CompileError) {
	sortedInstances := make([]ConstraintInstance, len(instances))
	copy(sortedInstances, instances)

	sort.Slice(sortedInstances, func(i, j int) bool {
		if sortedInstances[i].ID != sortedInstances[j].ID {
			return sortedInstances[i].ID < sortedInstances[j].ID
		}
		return sortedInstances[i].TemplateID < sortedInstances[j].TemplateID
	})

	canonicalBytes, err := json.Marshal(sortedInstances)
	if err != nil {
		return nil, "", []CompileError{
			{
				TemplateID:    "N/A",
				InstanceIndex: -1,
				Field:         "canonicalization",
				Message:       err.Error(),
			},
		}
	}

	hash := fmt.Sprintf("%x", sha256.Sum256(canonicalBytes))

	var compileErrs []CompileError

	for idx, inst := range instances {
		errs := validateInstance(p, idx, inst)
		if len(errs) > 0 {
			compileErrs = append(compileErrs, errs...)
		}
	}

	if len(compileErrs) > 0 {
		return nil, "", compileErrs
	}

	var compiled []ConstraintDef
	var hard []ConstraintDef
	var soft []ConstraintDef

	for _, inst := range sortedInstances {
		def := compileInstanceDef(inst)
		compiled = append(compiled, def)
		if def.Kind() == ConstraintKindHard {
			hard = append(hard, def)
		} else {
			soft = append(soft, def)
		}
	}

	return &CompiledConstraintSet{
		Constraints: compiled,
		Hard:        hard,
		Soft:        soft,
		RuleSetHash: hash,
	}, hash, nil
}

func validateInstance(p *problem.Problem, idx int, inst ConstraintInstance) []CompileError {
	var errs []CompileError

	// Soft constraint compilation is intentionally disabled until
	// the scoring bridge is implemented.
	if inst.Kind == ConstraintKindSoft {
		errs = append(errs, CompileError{
			TemplateID:    inst.TemplateID,
			InstanceIndex: idx,
			Field:         "kind",
			Message:       "soft constraints are not supported by the current scoring engine",
		})
	} else if inst.Kind != ConstraintKindHard && inst.Kind != "" {

		errs = append(errs, CompileError{
			TemplateID:    inst.TemplateID,
			InstanceIndex: idx,
			Field:         "kind",
			Message:       fmt.Sprintf("invalid constraint kind: %s", inst.Kind),
		})
	}

	switch inst.TemplateID {
	case "FacultyConflict":
		// No extra parameters required for FacultyConflict
	case "FacultyAvailability":
		// No extra parameters required for FacultyAvailability
	case "RoomConflict":
		// No extra parameters required for RoomConflict
	case "RoomCapacity":
		// No extra parameters required for RoomCapacity
	case "RoomAvailability":
		// No extra parameters required for RoomAvailability
	case "RoomFeatureCompatibility":
		// No extra parameters required for RoomFeatureCompatibility
	case "StudentGroupConflict":
		// No extra parameters required for StudentGroupConflict
	case "SubjectMaxPerDay":
		errs = append(errs, validateSubjectMaxPerDay(p, idx, inst)...)
	default:
		errs = append(errs, CompileError{
			TemplateID:    inst.TemplateID,
			InstanceIndex: idx,
			Field:         "templateId",
			Message:       fmt.Sprintf("unknown constraint template: %s", inst.TemplateID),
		})
	}

	return errs
}

func validateSubjectMaxPerDay(p *problem.Problem, idx int, inst ConstraintInstance) []CompileError {
	var errs []CompileError
	params := inst.Params

	if params == nil {
		return []CompileError{
			{
				TemplateID:    inst.TemplateID,
				InstanceIndex: idx,
				Field:         "params",
				Message:       "params map is missing",
			},
		}
	}

	subjRaw, hasSubj := params["subjectId"]
	offeringRaw, hasOffering := params["courseOfferingId"]

	var subjectID string
	var offeringID string

	if hasSubj {
		s, ok := subjRaw.(string)
		if !ok || s == "" {
			errs = append(errs, CompileError{
				TemplateID:    inst.TemplateID,
				InstanceIndex: idx,
				Field:         "subjectId",
				Message:       "subjectId must be a non-empty string",
			})
		} else {
			subjectID = s
		}
	}

	if hasOffering {
		o, ok := offeringRaw.(string)
		if !ok || o == "" {
			errs = append(errs, CompileError{
				TemplateID:    inst.TemplateID,
				InstanceIndex: idx,
				Field:         "courseOfferingId",
				Message:       "courseOfferingId must be a non-empty string",
			})
		} else {
			offeringID = o
		}
	}

	if subjectID == "" && offeringID == "" {
		errs = append(errs, CompileError{
			TemplateID:    inst.TemplateID,
			InstanceIndex: idx,
			Field:         "subjectId/courseOfferingId",
			Message:       "required subjectId OR courseOfferingId",
		})
	}

	if p != nil {
		if subjectID != "" {
			if _, ok := p.Subjects[model.SubjectID(subjectID)]; !ok {
				errs = append(errs, CompileError{
					TemplateID:    inst.TemplateID,
					InstanceIndex: idx,
					Field:         "subjectId",
					Message:       fmt.Sprintf("referenced subjectId '%s' does not exist in problem", subjectID),
				})
			}
		}
		if offeringID != "" {
			if _, ok := p.CourseOfferings[model.CourseOfferingID(offeringID)]; !ok {
				errs = append(errs, CompileError{
					TemplateID:    inst.TemplateID,
					InstanceIndex: idx,
					Field:         "courseOfferingId",
					Message:       fmt.Sprintf("referenced courseOfferingId '%s' does not exist in problem", offeringID),
				})
			}
		}
	}

	maxRaw, hasMax := params["maxPerDay"]
	if !hasMax || maxRaw == nil {
		errs = append(errs, CompileError{
			TemplateID:    inst.TemplateID,
			InstanceIndex: idx,
			Field:         "maxPerDay",
			Message:       "missing required parameter maxPerDay",
		})
	} else {
		var maxVal int
		validNum := false

		switch v := maxRaw.(type) {
		case int:
			maxVal = v
			validNum = true
		case float64:
			if v == float64(int(v)) {
				maxVal = int(v)
				validNum = true
			}
		}

		if !validNum {
			errs = append(errs, CompileError{
				TemplateID:    inst.TemplateID,
				InstanceIndex: idx,
				Field:         "maxPerDay",
				Message:       "maxPerDay must be a numeric integer",
			})
		} else if maxVal < 1 {
			errs = append(errs, CompileError{
				TemplateID:    inst.TemplateID,
				InstanceIndex: idx,
				Field:         "maxPerDay",
				Message:       "maxPerDay must be >= 1",
			})
		}
	}

	return errs
}

func compileInstanceDef(inst ConstraintInstance) ConstraintDef {
	switch inst.TemplateID {
	case "FacultyConflict":
		return NewFacultyConflictConstraint(inst)
	case "FacultyAvailability":
		return NewFacultyAvailabilityConstraint(inst)
	case "RoomConflict":
		return NewRoomConflictConstraint(inst)
	case "RoomCapacity":
		return NewRoomCapacityConstraint(inst)
	case "RoomAvailability":
		return NewRoomAvailabilityConstraint(inst)
	case "RoomFeatureCompatibility":
		return NewRoomFeatureCompatibilityConstraint(inst)
	case "StudentGroupConflict":
		return NewStudentGroupConflictConstraint(inst)
	case "SubjectMaxPerDay":
		return NewSubjectMaxPerDayConstraint(inst)
	default:
		return nil
	}
}
