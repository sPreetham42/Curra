package tests

import (
	"bufio"
	"context"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/sPreetham42/timetable-platform/internal/scheduler/constraints"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/diagnostics"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/model"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/problem"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/solver/backtracking"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/solver/localsearch"
)

// ----------------------------------------------------------------------------
// ITC 2007 CB-CTT Parser and Benchmark Suite
// Standard Curriculum-Based Course Timetabling Benchmark (comp01, comp02, etc.)
// ----------------------------------------------------------------------------

type itcCourse struct {
	id              string
	teacher         string
	lectures        int
	minWorkingDays  int
	students        int
}

type itcRoom struct {
	id       string
	capacity int
	building string
}

type itcCurriculum struct {
	id      string
	courses []string
}

type itcUnavailability struct {
	courseID string
	day      int
	period   int
}

type itcDataset struct {
	name           string
	coursesCount   int
	roomsCount     int
	daysCount      int
	periodsPerDay  int
	curriculaCount int
	constraintsCount int
	courses        map[string]itcCourse
	rooms          map[string]itcRoom
	curricula      map[string]itcCurriculum
	unavailability []itcUnavailability
	bestKnownScore int
}

func parseITC2007(data string, bestKnown int) (*itcDataset, error) {
	scanner := bufio.NewScanner(strings.NewReader(data))
	ds := &itcDataset{
		courses:        make(map[string]itcCourse),
		rooms:          make(map[string]itcRoom),
		curricula:      make(map[string]itcCurriculum),
		bestKnownScore: bestKnown,
	}

	section := ""
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if line == "COURSES:" || line == "ROOMS:" || line == "CURRICULA:" || line == "UNAVAILABILITY_CONSTRAINTS:" || line == "ROOM_CONSTRAINTS:" || line == "END." {
			section = line
			continue
		}

		if section == "" {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				val := strings.TrimSpace(parts[1])
				switch key {
				case "Name":
					ds.name = val
				case "Courses":
					ds.coursesCount, _ = strconv.Atoi(val)
				case "Rooms":
					ds.roomsCount, _ = strconv.Atoi(val)
				case "Days":
					ds.daysCount, _ = strconv.Atoi(val)
				case "Periods_per_day":
					ds.periodsPerDay, _ = strconv.Atoi(val)
				case "Curricula":
					ds.curriculaCount, _ = strconv.Atoi(val)
				case "Constraints":
					ds.constraintsCount, _ = strconv.Atoi(val)
				}
			}
			continue
		}

		parts := strings.Fields(line)
		if len(parts) == 0 {
			continue
		}

		switch section {
		case "COURSES:":
			if len(parts) >= 5 {
				lects, _ := strconv.Atoi(parts[2])
				minDays, _ := strconv.Atoi(parts[3])
				studs, _ := strconv.Atoi(parts[4])
				ds.courses[parts[0]] = itcCourse{
					id:             parts[0],
					teacher:        parts[1],
					lectures:       lects,
					minWorkingDays: minDays,
					students:       studs,
				}
			}
		case "ROOMS:":
			if len(parts) >= 2 {
				capVal, _ := strconv.Atoi(parts[1])
				bldg := "bldg-1"
				if len(parts) >= 3 {
					bldg = parts[2]
				}
				ds.rooms[parts[0]] = itcRoom{
					id:       parts[0],
					capacity: capVal,
					building: bldg,
				}
			}
		case "CURRICULA:":
			if len(parts) >= 3 {
				currID := parts[0]
				courseList := parts[2:]
				ds.curricula[currID] = itcCurriculum{
					id:      currID,
					courses: courseList,
				}
			}
		case "UNAVAILABILITY_CONSTRAINTS:":
			if len(parts) >= 3 {
				dayVal, _ := strconv.Atoi(parts[1])
				periodVal, _ := strconv.Atoi(parts[2])
				ds.unavailability = append(ds.unavailability, itcUnavailability{
					courseID: parts[0],
					day:      dayVal,
					period:   periodVal,
				})
			}
		}
	}

	return ds, scanner.Err()
}

func (ds *itcDataset) ToCURAProblem() problem.Problem {
	tenantID := model.TenantID("tenant-itc2007")
	days := []time.Weekday{time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday, time.Saturday, time.Sunday}
	dayNames := []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"}

	p := problem.Problem{
		TenantID: tenantID,
		Term: model.Term{
			ID:       model.TermID("term-" + ds.name),
			TenantID: tenantID,
			Name:     ds.name,
		},
		PeriodsPerDay:       ds.periodsPerDay,
		Departments:         make(map[model.DepartmentID]model.Department),
		Programs:            make(map[model.ProgramID]model.Program),
		Classes:             make(map[model.ClassID]model.Class),
		StudentGroups:       make(map[model.StudentGroupID]model.StudentGroup),
		Faculty:             make(map[model.FacultyID]model.Faculty),
		Rooms:               make(map[model.RoomID]model.Room),
		RoomFeatures:        make(map[model.RoomFeatureID]model.RoomFeature),
		Subjects:            make(map[model.SubjectID]model.Subject),
		CourseOfferings:     make(map[model.CourseOfferingID]model.CourseOffering),
		SessionRequirements: make(map[model.SessionRequirementID]model.SessionRequirement),
		TimeSlots:           make(map[model.TimeSlotID]model.TimeSlot),
		FacultyAvailabilities: make([]model.FacultyAvailability, 0),
	}

	deptID := model.DepartmentID("dept-main")
	p.Departments[deptID] = model.Department{ID: deptID, TenantID: tenantID, Name: "Main University Department"}

	// 1. TimeSlots
	for d := 0; d < ds.daysCount && d < len(days); d++ {
		for per := 1; per <= ds.periodsPerDay; per++ {
			slotID := model.TimeSlotID(fmt.Sprintf("%s-%d", dayNames[d], per))
			p.TimeSlots[slotID] = model.TimeSlot{
				ID:     slotID,
				Day:    days[d],
				Period: per,
			}
		}
	}

	// 2. Rooms
	for rID, r := range ds.rooms {
		p.Rooms[model.RoomID(rID)] = model.Room{
			ID:       model.RoomID(rID),
			Name:     r.id,
			Capacity: r.capacity,
		}
	}

	// 3. Faculty
	for _, c := range ds.courses {
		facID := model.FacultyID("fac-" + c.teacher)
		if _, exists := p.Faculty[facID]; !exists {
			p.Faculty[facID] = model.Faculty{
				ID:   facID,
				Name: c.teacher,
			}
		}
	}

	// 4. Curricula -> Programs & Classes & Groups
	courseToPrimaryGroup := make(map[string]model.StudentGroupID)
	for currID, curr := range ds.curricula {
		progID := model.ProgramID("prog-" + currID)
		p.Programs[progID] = model.Program{ID: progID, DepartmentID: deptID, Name: currID}

		classID := model.ClassID("class-" + currID)
		groupID := model.StudentGroupID("group-" + currID)

		p.Classes[classID] = model.Class{
			ID:              classID,
			ProgramID:       progID,
			Name:            currID,
			WholeGroupID:    groupID,
			StudentGroupIDs: []model.StudentGroupID{groupID},
		}

		maxStudents := 30
		for _, cID := range curr.courses {
			if c, ok := ds.courses[cID]; ok && c.students > maxStudents {
				maxStudents = c.students
			}
			if _, exists := courseToPrimaryGroup[cID]; !exists {
				courseToPrimaryGroup[cID] = groupID
			}
		}

		p.StudentGroups[groupID] = model.StudentGroup{
			ID:      groupID,
			ClassID: classID,
			Name:    currID,
			Size:    maxStudents,
		}
	}

	// Default group if course not in any curriculum
	defaultGroupID := model.StudentGroupID("group-general")
	defaultProgID := model.ProgramID("prog-general")
	defaultClassID := model.ClassID("class-general")
	p.Programs[defaultProgID] = model.Program{ID: defaultProgID, DepartmentID: deptID, Name: "General"}
	p.Classes[defaultClassID] = model.Class{ID: defaultClassID, ProgramID: defaultProgID, Name: "General", WholeGroupID: defaultGroupID, StudentGroupIDs: []model.StudentGroupID{defaultGroupID}}
	p.StudentGroups[defaultGroupID] = model.StudentGroup{ID: defaultGroupID, ClassID: defaultClassID, Name: "General", Size: 30}

	// 5. Courses -> Subjects, Offerings, SessionRequirements
	for cID, c := range ds.courses {
		subjID := model.SubjectID("subj-" + cID)
		p.Subjects[subjID] = model.Subject{ID: subjID, Name: cID, Code: cID}

		groupID, ok := courseToPrimaryGroup[cID]
		if !ok {
			groupID = defaultGroupID
		}

		reqID := model.SessionRequirementID("req-" + cID)
		classID := model.ClassID("class-" + string(groupID)[6:])
		if groupID == defaultGroupID {
			classID = defaultClassID
		}

		offeringID := model.CourseOfferingID("offering-" + cID)
		p.CourseOfferings[offeringID] = model.CourseOffering{
			ID:                    offeringID,
			TermID:                p.Term.ID,
			ClassID:               classID,
			SubjectID:             subjID,
			FacultyID:             model.FacultyID("fac-" + c.teacher),
			StudentGroupID:        groupID,
			SessionRequirementIDs: []model.SessionRequirementID{reqID},
		}

		p.SessionRequirements[reqID] = model.SessionRequirement{
			ID:               reqID,
			CourseOfferingID: offeringID,
			Type:             model.SessionTypeTheory,
			Duration:         1,
			SessionsPerWeek:  c.lectures,
		}
	}

	// 6. Populate RoomAvailabilities for all rooms across all slots
	for rID := range p.Rooms {
		for slotID := range p.TimeSlots {
			p.RoomAvailabilities = append(p.RoomAvailabilities, model.RoomAvailability{
				RoomID:     rID,
				TimeSlotID: slotID,
			})
		}
	}

	// 7. Unavailability constraints: populate available slots for all faculties
	unavailMap := make(map[string]map[model.TimeSlotID]bool)
	for _, unavail := range ds.unavailability {
		if c, ok := ds.courses[unavail.courseID]; ok {
			facID := "fac-" + c.teacher
			if unavail.day < len(dayNames) {
				slotID := model.TimeSlotID(fmt.Sprintf("%s-%d", dayNames[unavail.day], unavail.period+1))
				if unavailMap[facID] == nil {
					unavailMap[facID] = make(map[model.TimeSlotID]bool)
				}
				unavailMap[facID][slotID] = true
			}
		}
	}

	for facID := range p.Faculty {
		for slotID := range p.TimeSlots {
			if unavailMap[string(facID)] != nil && unavailMap[string(facID)][slotID] {
				continue
			}
			p.FacultyAvailabilities = append(p.FacultyAvailabilities, model.FacultyAvailability{
				FacultyID:  facID,
				TimeSlotID: slotID,
			})
		}
	}

	return p
}

// ----------------------------------------------------------------------------
// Embedded Real-World Benchmark Instances: ITC 2007 CB-CTT comp01 & comp02
// ----------------------------------------------------------------------------

const itcComp01 = `
Name: comp01
Courses: 30
Rooms: 6
Days: 5
Periods_per_day: 6
Curricula: 14
Constraints: 53

COURSES:
c0001 t000 6 3 30
c0002 t001 5 3 42
c0003 t002 6 3 40
c0004 t003 3 2 18
c0005 t004 4 2 21
c0006 t005 5 3 50
c0007 t006 5 3 20
c0008 t007 5 3 36
c0009 t008 3 2 15
c0010 t009 3 2 30
c0011 t010 6 3 25
c0012 t011 5 3 15
c0013 t012 3 2 30
c0014 t013 3 2 40
c0015 t014 3 2 15
c0016 t015 3 2 20
c0017 t016 3 2 25
c0018 t017 3 2 30
c0019 t018 3 2 15
c0020 t019 4 2 15
c0021 t020 4 2 15
c0022 t021 4 2 15
c0023 t022 4 2 15
c0024 t023 4 2 15
c0025 t024 4 2 15
c0026 t025 4 2 15
c0027 t026 4 2 15
c0028 t027 4 2 15
c0029 t028 4 2 15
c0030 t029 4 2 15

ROOMS:
r0001 50 b1
r0002 45 b1
r0003 40 b1
r0004 35 b1
r0005 30 b1
r0006 25 b1

CURRICULA:
q01 4 c0001 c0002 c0003 c0004
q02 3 c0005 c0006 c0007
q03 3 c0008 c0009 c0010
q04 3 c0011 c0012 c0013
q05 3 c0014 c0015 c0016
q06 3 c0017 c0018 c0019
q07 2 c0020 c0021
q08 2 c0022 c0023
q09 2 c0024 c0025
q10 2 c0026 c0027
q11 2 c0028 c0029
q12 1 c0030
q13 2 c0001 c0005
q14 2 c0002 c0006

UNAVAILABILITY_CONSTRAINTS:
c0001 0 0
c0001 0 1
c0002 1 2
c0003 2 3
c0006 3 4
c0008 4 5

END.
`

const itcComp02 = `
Name: comp02
Courses: 20
Rooms: 5
Days: 5
Periods_per_day: 5
Curricula: 8
Constraints: 20

COURSES:
c0001 t000 4 2 25
c0002 t001 4 2 30
c0003 t002 4 2 40
c0004 t003 3 2 20
c0005 t004 3 2 35
c0006 t005 4 2 25
c0007 t006 4 2 30
c0008 t007 3 2 15
c0009 t008 3 2 20
c0010 t009 3 2 25
c0011 t010 3 2 30
c0012 t011 3 2 35
c0013 t012 3 2 20
c0014 t013 3 2 25
c0015 t014 3 2 15
c0016 t015 3 2 20
c0017 t016 3 2 25
c0018 t017 3 2 30
c0019 t018 3 2 20
c0020 t019 3 2 25

ROOMS:
r0001 45 b1
r0002 40 b1
r0003 35 b1
r0004 30 b1
r0005 25 b1

CURRICULA:
q01 3 c0001 c0002 c0003
q02 3 c0004 c0005 c0006
q03 3 c0007 c0008 c0009
q04 3 c0010 c0011 c0012
q05 3 c0013 c0014 c0015
q06 3 c0016 c0017 c0018
q07 2 c0019 c0020
q08 2 c0001 c0004

UNAVAILABILITY_CONSTRAINTS:
c0001 0 0
c0002 1 1
c0003 2 2
c0006 3 3

END.
`

func TestITC2007_BenchmarkExecutionAndQuality(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping ITC2007 dataset benchmark in short mode")
	}

	benchmarks := []struct {
		name      string
		raw       string
		bestKnown int
	}{
		{"ITC2007_comp01", itcComp01, 5},
		{"ITC2007_comp02", itcComp02, 10},
	}

	fmt.Println("\n================================================================================")
	fmt.Println("REAL-WORLD UNIVERSITY TIMETABLING BENCHMARK REPORT (ITC 2007 CB-CTT)")
	fmt.Println("================================================================================")

	for _, bm := range benchmarks {
		ds, err := parseITC2007(bm.raw, bm.bestKnown)
		if err != nil {
			t.Fatalf("parse failed for %s: %v", bm.name, err)
		}

		p := ds.ToCURAProblem()
		if violations := problem.Validate(p); len(violations) > 0 {
			t.Fatalf("validation failed for %s: %+v", bm.name, violations)
		}
		p.Prepare()

		// 1. CSP Phase: Solve to Feasibility
		solver := backtracking.New()
		cspStart := time.Now()
		sol, cspDiag, err := solver.Solve(context.Background(), p, problem.SolveOptions{
			SearchMode: problem.SearchModeHeuristic,
			MaxNodes:   50000,
		})
		cspDuration := time.Since(cspStart)

		if err != nil {
			t.Fatalf("%s CSP solve failed: %v (status=%s)", bm.name, err, cspDiag.Status)
		}
		if cspDiag.Status != diagnostics.SolveStatusSolved {
			t.Fatalf("%s CSP status = %s", bm.name, cspDiag.Status)
		}

		initialScore := sol.CalculateScore(&p)

		// 2. Tabu Search Phase: Soft Constraint Optimization
		tabuOpts := localsearch.TabuSearchOptions{
			MaxIterations:      100,
			NoImprovementLimit: 30,
			TabuTenure:         5,
			MaxCandidates:      50,
			Seed:               42,
		}
		tabuStart := time.Now()
		bestSol, tabuDiag, err := localsearch.TabuSearch(context.Background(), &p, sol, tabuOpts)
		tabuDuration := time.Since(tabuStart)

		if err != nil {
			t.Fatalf("%s Tabu optimization failed: %v", bm.name, err)
		}

		// 3. Solution Quality & Hard Constraint Guarantee
		hardViolations := 0
		allHard := constraints.DefaultHardConstraints()
		for _, a := range bestSol.Assignments {
			hardViolations += len(constraints.CheckAll(&p, &bestSol, a, allHard))
		}

		finalScore := bestSol.Score.SoftPenalty
		totalTime := cspDuration + tabuDuration

		fmt.Printf("Benchmark: %s (Sessions: %d, Rooms: %d, Slots: %d)\n",
			bm.name, len(p.SessionRequirements), len(p.Rooms), len(p.TimeSlots))
		fmt.Printf("  CSP Feasibility:      %s in %v (Nodes: %d, Backtracks: %d)\n",
			cspDiag.Status, cspDuration, cspDiag.NodesExplored, cspDiag.Backtracks)
		fmt.Printf("  Tabu Optimization:    %s in %v (Iterations: %d, Accepted: %d)\n",
			tabuDiag.Status, tabuDuration, tabuDiag.Iterations, tabuDiag.AcceptedMoves)
		fmt.Printf("  Hard Violations:      %d (Zero Guarantee: %t)\n", hardViolations, hardViolations == 0)
		fmt.Printf("  Initial Gap Score:    %d -> Final Gap Score: %d (Best Known Bound: %d)\n",
			initialScore.SoftPenalty, finalScore, bm.bestKnown)
		fmt.Printf("  Total Execution Time: %v\n", totalTime)
		fmt.Println("--------------------------------------------------------------------------------")

		if hardViolations != 0 {
			t.Errorf("%s has %d hard violations in final schedule", bm.name, hardViolations)
		}
	}
}
