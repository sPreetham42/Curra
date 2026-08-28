package testutil

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"
	"time"

	"github.com/sPreetham42/timetable-platform/internal/scheduler/diagnostics"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/model"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/problem"
)

// SyntheticProblemConfig specifies parameters for generating realistic, deterministic benchmark problems.
type SyntheticProblemConfig struct {
	Seed              int64
	TenantID          string
	NumDepartments    int
	ProgramsPerDept   int
	ClassesPerProgram int
	SubgroupsPerClass int
	NumFaculty        int
	NumRooms          int
	NumLabRooms       int
	DaysCount         int
	PeriodsPerDay     int
	OfferingsPerClass int
	SessionsPerWeek   int
	LabRatio          float64 // fraction of offerings requiring lab features and duration=2
}

// DefaultSmallProblemConfig creates a valid ~24 session problem.
func DefaultSmallProblemConfig() SyntheticProblemConfig {
	return SyntheticProblemConfig{
		Seed:              42,
		TenantID:          "tenant-bench-small",
		NumDepartments:    1,
		ProgramsPerDept:   1,
		ClassesPerProgram: 2,
		SubgroupsPerClass: 2,
		NumFaculty:        6,
		NumRooms:          4,
		NumLabRooms:       1,
		DaysCount:         5,
		PeriodsPerDay:     6,
		OfferingsPerClass: 4,
		SessionsPerWeek:   3,
		LabRatio:          0.25,
	}
}

// DefaultMediumProblemConfig creates a valid ~300 session problem.
func DefaultMediumProblemConfig() SyntheticProblemConfig {
	return SyntheticProblemConfig{
		Seed:              101,
		TenantID:          "tenant-bench-medium",
		NumDepartments:    2,
		ProgramsPerDept:   2,
		ClassesPerProgram: 5,
		SubgroupsPerClass: 2,
		NumFaculty:        50,
		NumRooms:          30,
		NumLabRooms:       5,
		DaysCount:         5,
		PeriodsPerDay:     6,
		OfferingsPerClass: 5,
		SessionsPerWeek:   3,
		LabRatio:          0.2,
	}
}

// DefaultLargeProblemConfig creates a valid ~3000 session problem.
func DefaultLargeProblemConfig() SyntheticProblemConfig {
	return SyntheticProblemConfig{
		Seed:              202,
		TenantID:          "tenant-bench-large",
		NumDepartments:    5,
		ProgramsPerDept:   4,
		ClassesPerProgram: 5,
		SubgroupsPerClass: 2,
		NumFaculty:        300,
		NumRooms:          150,
		NumLabRooms:       25,
		DaysCount:         5,
		PeriodsPerDay:     8,
		OfferingsPerClass: 10,
		SessionsPerWeek:   3,
		LabRatio:          0.2,
	}
}

// GenerateSyntheticProblem builds a fully validated Problem graph according to the config.
func GenerateSyntheticProblem(cfg SyntheticProblemConfig) problem.Problem {
	rng := rand.New(rand.NewSource(cfg.Seed))
	labFeatureID := model.RoomFeatureID("feature-lab")

	p := problem.Problem{
		TenantID: model.TenantID(cfg.TenantID),
		Term: model.Term{
			ID:       model.TermID(fmt.Sprintf("term-%s", cfg.TenantID)),
			TenantID: model.TenantID(cfg.TenantID),
			Name:     fmt.Sprintf("Benchmark Term for %s", cfg.TenantID),
		},
		Departments:         make(map[model.DepartmentID]model.Department),
		Programs:            make(map[model.ProgramID]model.Program),
		Classes:             make(map[model.ClassID]model.Class),
		StudentGroups:       make(map[model.StudentGroupID]model.StudentGroup),
		Subjects:            make(map[model.SubjectID]model.Subject),
		CourseOfferings:     make(map[model.CourseOfferingID]model.CourseOffering),
		SessionRequirements: make(map[model.SessionRequirementID]model.SessionRequirement),
		Faculty:             make(map[model.FacultyID]model.Faculty),
		Rooms:               make(map[model.RoomID]model.Room),
		RoomFeatures: map[model.RoomFeatureID]model.RoomFeature{
			labFeatureID: {ID: labFeatureID, Name: "Computer / Scientific Lab"},
		},
		TimeSlots:     make(map[model.TimeSlotID]model.TimeSlot),
		PeriodsPerDay: cfg.PeriodsPerDay,
	}

	// 1. TimeSlots
	weekdays := []time.Weekday{time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday, time.Saturday, time.Sunday}
	dayCount := cfg.DaysCount
	if dayCount > len(weekdays) {
		dayCount = len(weekdays)
	}

	for d := 0; d < dayCount; d++ {
		day := weekdays[d]
		for period := 1; period <= cfg.PeriodsPerDay; period++ {
			slotID := model.TimeSlotID(fmt.Sprintf("slot-%s-p%d", day.String()[:3], period))
			p.TimeSlots[slotID] = model.TimeSlot{
				ID:     slotID,
				Day:    day,
				Period: period,
				Label:  fmt.Sprintf("%s P%d", day.String()[:3], period),
			}
		}
	}

	// 2. Rooms
	for r := 0; r < cfg.NumRooms; r++ {
		isLab := r < cfg.NumLabRooms
		roomID := model.RoomID(fmt.Sprintf("room-%03d", r+1))
		var features []model.RoomFeatureID
		capacity := 60
		if isLab {
			features = []model.RoomFeatureID{labFeatureID}
			capacity = 40
		}
		p.Rooms[roomID] = model.Room{
			ID:         roomID,
			Name:       fmt.Sprintf("Room %03d", r+1),
			Capacity:   capacity,
			FeatureIDs: features,
		}
		for slotID := range p.TimeSlots {
			p.RoomAvailabilities = append(p.RoomAvailabilities, model.RoomAvailability{
				RoomID:     roomID,
				TimeSlotID: slotID,
			})
		}
	}

	// 3. Faculty
	for f := 0; f < cfg.NumFaculty; f++ {
		facultyID := model.FacultyID(fmt.Sprintf("faculty-%03d", f+1))
		p.Faculty[facultyID] = model.Faculty{
			ID:   facultyID,
			Name: fmt.Sprintf("Faculty %03d", f+1),
		}
		for slotID := range p.TimeSlots {
			p.FacultyAvailabilities = append(p.FacultyAvailabilities, model.FacultyAvailability{
				FacultyID:  facultyID,
				TimeSlotID: slotID,
			})
		}
	}

	// 4. Hierarchy: Departments -> Programs -> Classes -> StudentGroups
	var allClasses []model.ClassID
	facultyIdx := 0
	subjIdx := 0
	offeringIdx := 0

	for d := 0; d < cfg.NumDepartments; d++ {
		deptID := model.DepartmentID(fmt.Sprintf("dept-%02d", d+1))
		p.Departments[deptID] = model.Department{
			ID:       deptID,
			TenantID: p.TenantID,
			Name:     fmt.Sprintf("Department %02d", d+1),
		}

		for pr := 0; pr < cfg.ProgramsPerDept; pr++ {
			progID := model.ProgramID(fmt.Sprintf("prog-%02d-%02d", d+1, pr+1))
			p.Programs[progID] = model.Program{
				ID:           progID,
				DepartmentID: deptID,
				Name:         fmt.Sprintf("Program %02d-%02d", d+1, pr+1),
			}

			for c := 0; c < cfg.ClassesPerProgram; c++ {
				classID := model.ClassID(fmt.Sprintf("class-%02d-%02d-%02d", d+1, pr+1, c+1))
				wholeGroupID := model.StudentGroupID(fmt.Sprintf("group-%s-whole", classID))

				groupIDs := []model.StudentGroupID{wholeGroupID}
				p.StudentGroups[wholeGroupID] = model.StudentGroup{
					ID:      wholeGroupID,
					ClassID: classID,
					Name:    fmt.Sprintf("Whole Class %s", classID),
					Size:    40,
				}

				for sg := 0; sg < cfg.SubgroupsPerClass; sg++ {
					subGroupID := model.StudentGroupID(fmt.Sprintf("group-%s-sub-%d", classID, sg+1))
					groupIDs = append(groupIDs, subGroupID)
					p.StudentGroups[subGroupID] = model.StudentGroup{
						ID:      subGroupID,
						ClassID: classID,
						Name:    fmt.Sprintf("Subgroup %s #%d", classID, sg+1),
						Size:    20,
					}
				}

				class := model.Class{
					ID:              classID,
					ProgramID:       progID,
					Name:            fmt.Sprintf("Class %s", classID),
					WholeGroupID:    wholeGroupID,
					StudentGroupIDs: groupIDs,
				}
				p.Classes[classID] = class
				allClasses = append(allClasses, classID)
			}
		}
	}

	// 5. Subjects & CourseOfferings & SessionRequirements
	for _, classID := range allClasses {
		class := p.Classes[classID]
		for o := 0; o < cfg.OfferingsPerClass; o++ {
			offeringIdx++
			subjIdx++

			isLab := rng.Float64() < cfg.LabRatio
			subjID := model.SubjectID(fmt.Sprintf("subj-%04d", subjIdx))
			p.Subjects[subjID] = model.Subject{
				ID:   subjID,
				Code: fmt.Sprintf("SUBJ%04d", subjIdx),
				Name: fmt.Sprintf("Subject %04d", subjIdx),
			}

			offeringID := model.CourseOfferingID(fmt.Sprintf("offering-%04d", offeringIdx))
			reqID := model.SessionRequirementID(fmt.Sprintf("req-%04d", offeringIdx))

			facultyID := model.FacultyID(fmt.Sprintf("faculty-%03d", (facultyIdx%cfg.NumFaculty)+1))
			facultyIdx++

			targetGroup := class.WholeGroupID
			reqDuration := 1
			sessionType := model.SessionTypeTheory
			var reqFeatures []model.RoomFeatureID

			if isLab {
				sessionType = model.SessionTypeLab
				reqDuration = 2
				if reqDuration > cfg.PeriodsPerDay {
					reqDuration = cfg.PeriodsPerDay
				}
				reqFeatures = []model.RoomFeatureID{labFeatureID}
				if len(class.StudentGroupIDs) > 1 {
					subIdx := 1 + (o % (len(class.StudentGroupIDs) - 1))
					targetGroup = class.StudentGroupIDs[subIdx]
				}
			}

			p.SessionRequirements[reqID] = model.SessionRequirement{
				ID:                     reqID,
				CourseOfferingID:       offeringID,
				Type:                   sessionType,
				SessionsPerWeek:        cfg.SessionsPerWeek,
				Duration:               reqDuration,
				Consecutive:            true,
				RequiredRoomFeatureIDs: reqFeatures,
			}

			p.CourseOfferings[offeringID] = model.CourseOffering{
				ID:                     offeringID,
				TermID:                 p.Term.ID,
				ClassID:                classID,
				SubjectID:              subjID,
				StudentGroupID:         targetGroup,
				FacultyID:              facultyID,
				RequiredRoomFeatureIDs: reqFeatures,
				SessionRequirementIDs:  []model.SessionRequirementID{reqID},
			}
		}
	}

	p.Prepare()
	return p
}

// CountTotalSessions calculates the total scheduled session instances for a Problem.
func CountTotalSessions(p *problem.Problem) int {
	total := 0
	for _, req := range p.SessionRequirements {
		total += req.SessionsPerWeek
	}
	return total
}

// LocalSearchTestProblem returns a deterministic problem and initial solution for local search & constraint testing.
func LocalSearchTestProblem() (problem.Problem, problem.Solution) {
	labFeatureID := model.RoomFeatureID("feature-lab")
	p := problem.Problem{
		TenantID: "tenant-ls",
		Term:     model.Term{ID: "term-ls", TenantID: "tenant-ls", Name: "LS Term"},
		Departments: map[model.DepartmentID]model.Department{
			"dept-1": {ID: "dept-1", TenantID: "tenant-ls", Name: "Dept 1"},
		},
		Programs: map[model.ProgramID]model.Program{
			"prog-1": {ID: "prog-1", DepartmentID: "dept-1", Name: "Prog 1"},
		},
		Classes: map[model.ClassID]model.Class{
			"class-a": {
				ID:              "class-a",
				ProgramID:       "prog-1",
				Name:            "Class A",
				WholeGroupID:    "group-a-whole",
				StudentGroupIDs: []model.StudentGroupID{"group-a-whole", "group-a-lab1", "group-a-lab2"},
			},
			"class-b": {
				ID:              "class-b",
				ProgramID:       "prog-1",
				Name:            "Class B",
				WholeGroupID:    "group-b-whole",
				StudentGroupIDs: []model.StudentGroupID{"group-b-whole"},
			},
		},
		StudentGroups: map[model.StudentGroupID]model.StudentGroup{
			"group-a-whole": {ID: "group-a-whole", ClassID: "class-a", Name: "A Whole", Size: 40},
			"group-a-lab1":  {ID: "group-a-lab1", ClassID: "class-a", Name: "A Lab 1", Size: 20},
			"group-a-lab2":  {ID: "group-a-lab2", ClassID: "class-a", Name: "A Lab 2", Size: 20},
			"group-b-whole": {ID: "group-b-whole", ClassID: "class-b", Name: "B Whole", Size: 30},
		},
		Subjects: map[model.SubjectID]model.Subject{
			"subj-theory": {ID: "subj-theory", Code: "T101", Name: "Theory"},
			"subj-lab":    {ID: "subj-lab", Code: "L101", Name: "Lab"},
		},
		CourseOfferings: map[model.CourseOfferingID]model.CourseOffering{
			"offering-a-theory": {
				ID:                    "offering-a-theory",
				TermID:                "term-ls",
				ClassID:               "class-a",
				SubjectID:             "subj-theory",
				StudentGroupID:        "group-a-whole",
				FacultyID:             "faculty-1",
				SessionRequirementIDs: []model.SessionRequirementID{"req-a-theory"},
			},
			"offering-a-lab1": {
				ID:                    "offering-a-lab1",
				TermID:                "term-ls",
				ClassID:               "class-a",
				SubjectID:             "subj-lab",
				StudentGroupID:        "group-a-lab1",
				FacultyID:             "faculty-2",
				SessionRequirementIDs: []model.SessionRequirementID{"req-a-lab1"},
			},
			"offering-b-theory": {
				ID:                    "offering-b-theory",
				TermID:                "term-ls",
				ClassID:               "class-b",
				SubjectID:             "subj-theory",
				StudentGroupID:        "group-b-whole",
				FacultyID:             "faculty-3",
				SessionRequirementIDs: []model.SessionRequirementID{"req-b-theory"},
			},
		},
		SessionRequirements: map[model.SessionRequirementID]model.SessionRequirement{
			"req-a-theory": {ID: "req-a-theory", CourseOfferingID: "offering-a-theory", Type: model.SessionTypeTheory, SessionsPerWeek: 2, Duration: 1, Consecutive: true},
			"req-a-lab1":   {ID: "req-a-lab1", CourseOfferingID: "offering-a-lab1", Type: model.SessionTypeLab, SessionsPerWeek: 1, Duration: 2, Consecutive: true, RequiredRoomFeatureIDs: []model.RoomFeatureID{labFeatureID}},
			"req-b-theory": {ID: "req-b-theory", CourseOfferingID: "offering-b-theory", Type: model.SessionTypeTheory, SessionsPerWeek: 1, Duration: 1, Consecutive: true},
		},
		Faculty: map[model.FacultyID]model.Faculty{
			"faculty-1": {ID: "faculty-1", Name: "Faculty 1"},
			"faculty-2": {ID: "faculty-2", Name: "Faculty 2"},
			"faculty-3": {ID: "faculty-3", Name: "Faculty 3"},
		},
		Rooms: map[model.RoomID]model.Room{
			"room-lecture-1": {ID: "room-lecture-1", Name: "Lecture 1", Capacity: 60},
			"room-lecture-2": {ID: "room-lecture-2", Name: "Lecture 2", Capacity: 60},
			"room-lab-1":     {ID: "room-lab-1", Name: "Lab 1", Capacity: 30, FeatureIDs: []model.RoomFeatureID{labFeatureID}},
			"room-small":     {ID: "room-small", Name: "Small", Capacity: 15},
		},
		RoomFeatures: map[model.RoomFeatureID]model.RoomFeature{
			labFeatureID: {ID: labFeatureID, Name: "Lab"},
		},
		TimeSlots: map[model.TimeSlotID]model.TimeSlot{
			"mon-1": {ID: "mon-1", Day: time.Monday, Period: 1, Label: "Mon P1"},
			"mon-2": {ID: "mon-2", Day: time.Monday, Period: 2, Label: "Mon P2"},
			"mon-3": {ID: "mon-3", Day: time.Monday, Period: 3, Label: "Mon P3"},
			"mon-4": {ID: "mon-4", Day: time.Monday, Period: 4, Label: "Mon P4"},
			"tue-1": {ID: "tue-1", Day: time.Tuesday, Period: 1, Label: "Tue P1"},
			"tue-2": {ID: "tue-2", Day: time.Tuesday, Period: 2, Label: "Tue P2"},
		},
		PeriodsPerDay: 4,
		FacultyAvailabilities: []model.FacultyAvailability{
			{FacultyID: "faculty-1", TimeSlotID: "mon-1"}, {FacultyID: "faculty-1", TimeSlotID: "mon-2"}, {FacultyID: "faculty-1", TimeSlotID: "mon-3"}, {FacultyID: "faculty-1", TimeSlotID: "mon-4"}, {FacultyID: "faculty-1", TimeSlotID: "tue-1"}, {FacultyID: "faculty-1", TimeSlotID: "tue-2"},
			{FacultyID: "faculty-2", TimeSlotID: "mon-1"}, {FacultyID: "faculty-2", TimeSlotID: "mon-2"}, {FacultyID: "faculty-2", TimeSlotID: "mon-3"}, {FacultyID: "faculty-2", TimeSlotID: "mon-4"}, {FacultyID: "faculty-2", TimeSlotID: "tue-1"}, {FacultyID: "faculty-2", TimeSlotID: "tue-2"},
			{FacultyID: "faculty-3", TimeSlotID: "mon-1"}, {FacultyID: "faculty-3", TimeSlotID: "mon-2"}, {FacultyID: "faculty-3", TimeSlotID: "mon-3"}, {FacultyID: "faculty-3", TimeSlotID: "mon-4"}, {FacultyID: "faculty-3", TimeSlotID: "tue-1"},
		},
		RoomAvailabilities: []model.RoomAvailability{
			{RoomID: "room-lecture-1", TimeSlotID: "mon-1"}, {RoomID: "room-lecture-1", TimeSlotID: "mon-2"}, {RoomID: "room-lecture-1", TimeSlotID: "mon-3"}, {RoomID: "room-lecture-1", TimeSlotID: "mon-4"}, {RoomID: "room-lecture-1", TimeSlotID: "tue-1"}, {RoomID: "room-lecture-1", TimeSlotID: "tue-2"},
			{RoomID: "room-lecture-2", TimeSlotID: "mon-1"}, {RoomID: "room-lecture-2", TimeSlotID: "mon-2"}, {RoomID: "room-lecture-2", TimeSlotID: "mon-3"}, {RoomID: "room-lecture-2", TimeSlotID: "mon-4"}, {RoomID: "room-lecture-2", TimeSlotID: "tue-1"}, {RoomID: "room-lecture-2", TimeSlotID: "tue-2"},
			{RoomID: "room-lab-1", TimeSlotID: "mon-1"}, {RoomID: "room-lab-1", TimeSlotID: "mon-2"}, {RoomID: "room-lab-1", TimeSlotID: "mon-3"}, {RoomID: "room-lab-1", TimeSlotID: "mon-4"}, {RoomID: "room-lab-1", TimeSlotID: "tue-1"}, {RoomID: "room-lab-1", TimeSlotID: "tue-2"},
			{RoomID: "room-small", TimeSlotID: "mon-1"}, {RoomID: "room-small", TimeSlotID: "mon-2"}, {RoomID: "room-small", TimeSlotID: "mon-3"}, {RoomID: "room-small", TimeSlotID: "mon-4"}, {RoomID: "room-small", TimeSlotID: "tue-1"},
		},
	}
	p.Prepare()

	solution := problem.NewSolution()
	_ = solution.AddAssignment(&p, problem.Assignment{ID: "a-theory-1", CourseOfferingID: "offering-a-theory", StudentGroupID: "group-a-whole", FacultyID: "faculty-1", RoomID: "room-lecture-1", TimeSlotID: "mon-1", SessionRequirementID: "req-a-theory", Instance: 0})
	_ = solution.AddAssignment(&p, problem.Assignment{ID: "a-theory-2", CourseOfferingID: "offering-a-theory", StudentGroupID: "group-a-whole", FacultyID: "faculty-1", RoomID: "room-lecture-1", TimeSlotID: "mon-3", SessionRequirementID: "req-a-theory", Instance: 1})
	_ = solution.AddAssignment(&p, problem.Assignment{ID: "a-lab-1", CourseOfferingID: "offering-a-lab1", StudentGroupID: "group-a-lab1", FacultyID: "faculty-2", RoomID: "room-lab-1", TimeSlotID: "tue-1", SessionRequirementID: "req-a-lab1", Instance: 0})
	_ = solution.AddAssignment(&p, problem.Assignment{ID: "b-theory", CourseOfferingID: "offering-b-theory", StudentGroupID: "group-b-whole", FacultyID: "faculty-3", RoomID: "room-lecture-2", TimeSlotID: "mon-1", SessionRequirementID: "req-b-theory", Instance: 0})

	return p, solution
}

// RandomFeasibleProblem generates a randomized feasible problem instance for testing.
func RandomFeasibleProblem(seed int64) problem.Problem {
	r := rand.New(rand.NewSource(seed))
	labFeatureID := model.RoomFeatureID("feature-lab")
	classCount := 2 + r.Intn(2)

	p := problem.Problem{
		TenantID: "tenant-random",
		Term:     model.Term{ID: "term-random", TenantID: "tenant-random", Name: "Random Term"},
		Departments: map[model.DepartmentID]model.Department{
			"dept-random": {ID: "dept-random", TenantID: "tenant-random", Name: "Random Department"},
		},
		Programs: map[model.ProgramID]model.Program{
			"program-random": {ID: "program-random", DepartmentID: "dept-random", Name: "Random Program"},
		},
		Classes:             make(map[model.ClassID]model.Class),
		StudentGroups:       make(map[model.StudentGroupID]model.StudentGroup),
		Subjects:            make(map[model.SubjectID]model.Subject),
		CourseOfferings:     make(map[model.CourseOfferingID]model.CourseOffering),
		SessionRequirements: make(map[model.SessionRequirementID]model.SessionRequirement),
		Faculty:             make(map[model.FacultyID]model.Faculty),
		Rooms:               make(map[model.RoomID]model.Room),
		RoomFeatures: map[model.RoomFeatureID]model.RoomFeature{
			labFeatureID: {ID: labFeatureID, Name: "Lab"},
		},
		TimeSlots: make(map[model.TimeSlotID]model.TimeSlot),
	}

	for day := time.Monday; day <= time.Friday; day++ {
		for period := 1; period <= 6; period++ {
			id := model.TimeSlotID(fmt.Sprintf("%s-%d", day.String()[:3], period))
			p.TimeSlots[id] = model.TimeSlot{ID: id, Day: day, Period: period, Label: string(id)}
		}
	}
	for i := 0; i < classCount+2; i++ {
		id := model.RoomID(fmt.Sprintf("room-lecture-%d", i))
		p.Rooms[id] = model.Room{ID: id, Name: string(id), Capacity: 120}
	}
	for i := 0; i < classCount*2+2; i++ {
		id := model.RoomID(fmt.Sprintf("room-lab-%d", i))
		p.Rooms[id] = model.Room{ID: id, Name: string(id), Capacity: 40, FeatureIDs: []model.RoomFeatureID{labFeatureID}}
	}

	p.Subjects["subject-theory"] = model.Subject{ID: "subject-theory", Code: "THEORY", Name: "Theory"}
	p.Subjects["subject-lab"] = model.Subject{ID: "subject-lab", Code: "LAB", Name: "Lab"}

	for c := 0; c < classCount; c++ {
		classID := model.ClassID(fmt.Sprintf("class-%d", c))
		wholeID := model.StudentGroupID(fmt.Sprintf("%s-whole", classID))
		subgroup1 := model.StudentGroupID(fmt.Sprintf("%s-lab-1", classID))
		subgroup2 := model.StudentGroupID(fmt.Sprintf("%s-lab-2", classID))
		p.Classes[classID] = model.Class{
			ID:              classID,
			ProgramID:       "program-random",
			Name:            string(classID),
			WholeGroupID:    wholeID,
			StudentGroupIDs: []model.StudentGroupID{wholeID, subgroup1, subgroup2},
		}
		p.StudentGroups[wholeID] = model.StudentGroup{ID: wholeID, ClassID: classID, Name: string(wholeID), Size: 60 + r.Intn(20)}
		p.StudentGroups[subgroup1] = model.StudentGroup{ID: subgroup1, ClassID: classID, Name: string(subgroup1), Size: 25 + r.Intn(10)}
		p.StudentGroups[subgroup2] = model.StudentGroup{ID: subgroup2, ClassID: classID, Name: string(subgroup2), Size: 25 + r.Intn(10)}

		addRandomOffering(&p, fmt.Sprintf("c%d-theory", c), classID, wholeID, "subject-theory", model.SessionTypeTheory, 1+r.Intn(2), 1, nil)
		addRandomOffering(&p, fmt.Sprintf("c%d-lab-1", c), classID, subgroup1, "subject-lab", model.SessionTypeLab, 1, 1+r.Intn(2), []model.RoomFeatureID{labFeatureID})
		addRandomOffering(&p, fmt.Sprintf("c%d-lab-2", c), classID, subgroup2, "subject-lab", model.SessionTypeLab, 1, 1+r.Intn(2), []model.RoomFeatureID{labFeatureID})
	}

	for facultyID := range p.Faculty {
		for slotID := range p.TimeSlots {
			p.FacultyAvailabilities = append(p.FacultyAvailabilities, model.FacultyAvailability{FacultyID: facultyID, TimeSlotID: slotID})
		}
	}
	for roomID := range p.Rooms {
		for slotID := range p.TimeSlots {
			p.RoomAvailabilities = append(p.RoomAvailabilities, model.RoomAvailability{RoomID: roomID, TimeSlotID: slotID})
		}
	}
	p.PeriodsPerDay = 6
	return p
}

func addRandomOffering(p *problem.Problem, suffix string, classID model.ClassID, groupID model.StudentGroupID, subjectID model.SubjectID, sessionType model.SessionType, sessions int, duration int, features []model.RoomFeatureID) {
	offeringID := model.CourseOfferingID("offering-" + suffix)
	requirementID := model.SessionRequirementID("req-" + suffix)
	facultyID := model.FacultyID("faculty-" + suffix)
	p.Faculty[facultyID] = model.Faculty{ID: facultyID, Name: string(facultyID)}
	p.CourseOfferings[offeringID] = model.CourseOffering{
		ID:                    offeringID,
		TermID:                p.Term.ID,
		ClassID:               classID,
		SubjectID:             subjectID,
		StudentGroupID:        groupID,
		FacultyID:             facultyID,
		RequiredRoomFeatureIDs: features,
		SessionRequirementIDs:  []model.SessionRequirementID{requirementID},
	}
	p.SessionRequirements[requirementID] = model.SessionRequirement{
		ID:                     requirementID,
		CourseOfferingID:       offeringID,
		Type:                   sessionType,
		SessionsPerWeek:        sessions,
		Duration:               duration,
		Consecutive:            true,
		RequiredRoomFeatureIDs: features,
	}
}

// MediumTestProblem returns a deterministic medium-size problem for local search & scoring tests.
func MediumTestProblem() problem.Problem {
	p := problem.Problem{
		TenantID: "tenant-medium",
		Term:     model.Term{ID: "term-m", TenantID: "tenant-medium", Name: "Medium Term"},
		Departments: map[model.DepartmentID]model.Department{
			"dept-1": {ID: "dept-1", TenantID: "tenant-medium", Name: "CS Dept"},
		},
		Programs: map[model.ProgramID]model.Program{
			"prog-1": {ID: "prog-1", DepartmentID: "dept-1", Name: "BS CS"},
		},
		Classes: map[model.ClassID]model.Class{
			"class-1": {ID: "class-1", ProgramID: "prog-1", Name: "Year 1", WholeGroupID: "g1-whole", StudentGroupIDs: []model.StudentGroupID{"g1-whole"}},
			"class-2": {ID: "class-2", ProgramID: "prog-1", Name: "Year 2", WholeGroupID: "g2-whole", StudentGroupIDs: []model.StudentGroupID{"g2-whole"}},
		},
		StudentGroups: map[model.StudentGroupID]model.StudentGroup{
			"g1-whole": {ID: "g1-whole", ClassID: "class-1", Name: "G1 Whole", Size: 30},
			"g2-whole": {ID: "g2-whole", ClassID: "class-2", Name: "G2 Whole", Size: 30},
		},
		Subjects: map[model.SubjectID]model.Subject{
			"subj-1": {ID: "subj-1", Code: "CS101", Name: "Intro CS"},
			"subj-2": {ID: "subj-2", Code: "CS201", Name: "Data Struct"},
		},
		CourseOfferings: map[model.CourseOfferingID]model.CourseOffering{
			"co-1": {ID: "co-1", TermID: "term-m", ClassID: "class-1", SubjectID: "subj-1", StudentGroupID: "g1-whole", FacultyID: "f-1", SessionRequirementIDs: []model.SessionRequirementID{"req-1"}},
			"co-2": {ID: "co-2", TermID: "term-m", ClassID: "class-2", SubjectID: "subj-2", StudentGroupID: "g2-whole", FacultyID: "f-2", SessionRequirementIDs: []model.SessionRequirementID{"req-2"}},
		},
		SessionRequirements: map[model.SessionRequirementID]model.SessionRequirement{
			"req-1": {ID: "req-1", CourseOfferingID: "co-1", Type: model.SessionTypeTheory, SessionsPerWeek: 3, Duration: 1, Consecutive: true},
			"req-2": {ID: "req-2", CourseOfferingID: "co-2", Type: model.SessionTypeTheory, SessionsPerWeek: 3, Duration: 1, Consecutive: true},
		},
		Faculty: map[model.FacultyID]model.Faculty{
			"f-1": {ID: "f-1", Name: "Prof Alpha"},
			"f-2": {ID: "f-2", Name: "Prof Beta"},
		},
		Rooms: map[model.RoomID]model.Room{
			"r-1": {ID: "r-1", Name: "Room 1", Capacity: 40},
			"r-2": {ID: "r-2", Name: "Room 2", Capacity: 40},
		},
		TimeSlots: map[model.TimeSlotID]model.TimeSlot{
			"m-1": {ID: "m-1", Day: time.Monday, Period: 1, Label: "Mon P1"},
			"m-2": {ID: "m-2", Day: time.Monday, Period: 2, Label: "Mon P2"},
			"m-3": {ID: "m-3", Day: time.Monday, Period: 3, Label: "Mon P3"},
			"t-1": {ID: "t-1", Day: time.Tuesday, Period: 1, Label: "Tue P1"},
			"t-2": {ID: "t-2", Day: time.Tuesday, Period: 2, Label: "Tue P2"},
			"t-3": {ID: "t-3", Day: time.Tuesday, Period: 3, Label: "Tue P3"},
		},
		PeriodsPerDay: 3,
		FacultyAvailabilities: []model.FacultyAvailability{
			{FacultyID: "f-1", TimeSlotID: "m-1"}, {FacultyID: "f-1", TimeSlotID: "m-2"}, {FacultyID: "f-1", TimeSlotID: "m-3"}, {FacultyID: "f-1", TimeSlotID: "t-1"}, {FacultyID: "f-1", TimeSlotID: "t-2"}, {FacultyID: "f-1", TimeSlotID: "t-3"},
			{FacultyID: "f-2", TimeSlotID: "m-1"}, {FacultyID: "f-2", TimeSlotID: "m-2"}, {FacultyID: "f-2", TimeSlotID: "m-3"}, {FacultyID: "f-2", TimeSlotID: "t-1"}, {FacultyID: "f-2", TimeSlotID: "t-2"}, {FacultyID: "f-2", TimeSlotID: "t-3"},
		},
		RoomAvailabilities: []model.RoomAvailability{
			{RoomID: "r-1", TimeSlotID: "m-1"}, {RoomID: "r-1", TimeSlotID: "m-2"}, {RoomID: "r-1", TimeSlotID: "m-3"}, {RoomID: "r-1", TimeSlotID: "t-1"}, {RoomID: "r-1", TimeSlotID: "t-2"}, {RoomID: "r-1", TimeSlotID: "t-3"},
			{RoomID: "r-2", TimeSlotID: "m-1"}, {RoomID: "r-2", TimeSlotID: "m-2"}, {RoomID: "r-2", TimeSlotID: "m-3"}, {RoomID: "r-2", TimeSlotID: "t-1"}, {RoomID: "r-2", TimeSlotID: "t-2"}, {RoomID: "r-2", TimeSlotID: "t-3"},
		},
	}
	p.Prepare()
	return p
}

// FeasibleProblem returns a minimal valid, feasible problem.
func FeasibleProblem() problem.Problem {
	groupID := model.StudentGroupID("group-a")
	facultyID := model.FacultyID("faculty-a")
	labFeatureID := model.RoomFeatureID("feature-lab")

	p := problem.Problem{
		TenantID: "tenant-a",
		Term: model.Term{
			ID:       "term-a",
			TenantID: "tenant-a",
			Name:     "Term A",
		},
		Departments: map[model.DepartmentID]model.Department{
			"dept-a": {ID: "dept-a", TenantID: "tenant-a", Name: "Engineering"},
		},
		Programs: map[model.ProgramID]model.Program{
			"program-a": {ID: "program-a", DepartmentID: "dept-a", Name: "B.Tech"},
		},
		Classes: map[model.ClassID]model.Class{
			"class-a": {
				ID:              "class-a",
				ProgramID:       "program-a",
				Name:            "A",
				WholeGroupID:    groupID,
				StudentGroupIDs: []model.StudentGroupID{groupID},
			},
		},
		StudentGroups: map[model.StudentGroupID]model.StudentGroup{
			groupID: {ID: groupID, ClassID: "class-a", Name: "A", Size: 30},
		},
		Subjects: map[model.SubjectID]model.Subject{
			"subject-theory": {ID: "subject-theory", Code: "T101", Name: "Theory"},
			"subject-lab":    {ID: "subject-lab", Code: "L101", Name: "Lab"},
		},
		CourseOfferings: map[model.CourseOfferingID]model.CourseOffering{
			"offering-theory": {
				ID:                    "offering-theory",
				TermID:                "term-a",
				ClassID:               "class-a",
				SubjectID:             "subject-theory",
				StudentGroupID:        groupID,
				FacultyID:             facultyID,
				SessionRequirementIDs: []model.SessionRequirementID{"req-theory"},
			},
			"offering-lab": {
				ID:                    "offering-lab",
				TermID:                "term-a",
				ClassID:               "class-a",
				SubjectID:             "subject-lab",
				StudentGroupID:        groupID,
				FacultyID:             facultyID,
				SessionRequirementIDs: []model.SessionRequirementID{"req-lab"},
			},
		},
		SessionRequirements: map[model.SessionRequirementID]model.SessionRequirement{
			"req-theory": {
				ID:               "req-theory",
				CourseOfferingID: "offering-theory",
				Type:             model.SessionTypeTheory,
				SessionsPerWeek:  1,
				Duration:         1,
				Consecutive:      true,
			},
			"req-lab": {
				ID:                     "req-lab",
				CourseOfferingID:       "offering-lab",
				Type:                   model.SessionTypeLab,
				SessionsPerWeek:        1,
				Duration:               2,
				Consecutive:            true,
				RequiredRoomFeatureIDs: []model.RoomFeatureID{labFeatureID},
			},
		},
		Faculty: map[model.FacultyID]model.Faculty{
			facultyID: {ID: facultyID, Name: "Faculty A"},
		},
		Rooms: map[model.RoomID]model.Room{
			"room-lecture": {ID: "room-lecture", Name: "Lecture", Capacity: 60},
			"room-lab":     {ID: "room-lab", Name: "Lab", Capacity: 40, FeatureIDs: []model.RoomFeatureID{labFeatureID}},
		},
		RoomFeatures: map[model.RoomFeatureID]model.RoomFeature{
			labFeatureID: {ID: labFeatureID, Name: "Lab"},
		},
		TimeSlots: map[model.TimeSlotID]model.TimeSlot{
			"mon-1": {ID: "mon-1", Day: time.Monday, Period: 1, Label: "Mon P1"},
			"mon-2": {ID: "mon-2", Day: time.Monday, Period: 2, Label: "Mon P2"},
			"mon-3": {ID: "mon-3", Day: time.Monday, Period: 3, Label: "Mon P3"},
			"tue-1": {ID: "tue-1", Day: time.Tuesday, Period: 1, Label: "Tue P1"},
			"tue-2": {ID: "tue-2", Day: time.Tuesday, Period: 2, Label: "Tue P2"},
		},
		FacultyAvailabilities: AvailabilityForFaculty(facultyID),
		RoomAvailabilities:    AvailabilityForRooms("room-lecture", "room-lab"),
		PeriodsPerDay:         3,
	}
	p.Prepare()
	return p
}

// OverlapProblem returns a problem with overlapping student sub-groups.
func OverlapProblem() problem.Problem {
	p := FeasibleProblem()
	p.Classes = map[model.ClassID]model.Class{
		"class-a": {
			ID:              "class-a",
			ProgramID:       "program-a",
			Name:            "A",
			WholeGroupID:    "class-a-whole",
			StudentGroupIDs: []model.StudentGroupID{"class-a-whole", "class-a-lab-1", "class-a-lab-2"},
		},
		"class-b": {
			ID:              "class-b",
			ProgramID:       "program-a",
			Name:            "B",
			WholeGroupID:    "class-b-whole",
			StudentGroupIDs: []model.StudentGroupID{"class-b-whole"},
		},
	}
	p.StudentGroups = map[model.StudentGroupID]model.StudentGroup{
		"class-a-whole": {ID: "class-a-whole", ClassID: "class-a", Name: "A whole", Size: 40},
		"class-a-lab-1": {ID: "class-a-lab-1", ClassID: "class-a", Name: "A lab 1", Size: 20},
		"class-a-lab-2": {ID: "class-a-lab-2", ClassID: "class-a", Name: "A lab 2", Size: 20},
		"class-b-whole": {ID: "class-b-whole", ClassID: "class-b", Name: "B whole", Size: 30},
	}
	p.Faculty["faculty-a1"] = model.Faculty{ID: "faculty-a1", Name: "Faculty A1"}
	p.Faculty["faculty-a2"] = model.Faculty{ID: "faculty-a2", Name: "Faculty A2"}
	p.Faculty["faculty-a3"] = model.Faculty{ID: "faculty-a3", Name: "Faculty A3"}
	p.Rooms["room-lab-1"] = model.Room{ID: "room-lab-1", Name: "Lab 1", Capacity: 30, FeatureIDs: []model.RoomFeatureID{"feature-lab"}}
	p.Rooms["room-lab-2"] = model.Room{ID: "room-lab-2", Name: "Lab 2", Capacity: 30, FeatureIDs: []model.RoomFeatureID{"feature-lab"}}
	p.Rooms["room-lecture"] = model.Room{ID: "room-lecture", Name: "Lecture", Capacity: 80}
	p.CourseOfferings = map[model.CourseOfferingID]model.CourseOffering{
		"offering-a-lab-1": {ID: "offering-a-lab-1", TermID: "term-a", ClassID: "class-a", SubjectID: "subject-lab", StudentGroupID: "class-a-lab-1", FacultyID: "faculty-a1", SessionRequirementIDs: []model.SessionRequirementID{"req-a-lab-1"}},
		"offering-a-lab-2": {ID: "offering-a-lab-2", TermID: "term-a", ClassID: "class-a", SubjectID: "subject-lab", StudentGroupID: "class-a-lab-2", FacultyID: "faculty-a2", SessionRequirementIDs: []model.SessionRequirementID{"req-a-lab-2"}},
		"offering-a-whole": {ID: "offering-a-whole", TermID: "term-a", ClassID: "class-a", SubjectID: "subject-theory", StudentGroupID: "class-a-whole", FacultyID: "faculty-a3", SessionRequirementIDs: []model.SessionRequirementID{"req-a-whole"}},
	}
	p.SessionRequirements = map[model.SessionRequirementID]model.SessionRequirement{
		"req-a-lab-1": {ID: "req-a-lab-1", CourseOfferingID: "offering-a-lab-1", Type: model.SessionTypeLab, SessionsPerWeek: 1, Duration: 1, Consecutive: true, RequiredRoomFeatureIDs: []model.RoomFeatureID{"feature-lab"}},
		"req-a-lab-2": {ID: "req-a-lab-2", CourseOfferingID: "offering-a-lab-2", Type: model.SessionTypeLab, SessionsPerWeek: 1, Duration: 1, Consecutive: true, RequiredRoomFeatureIDs: []model.RoomFeatureID{"feature-lab"}},
		"req-a-whole": {ID: "req-a-whole", CourseOfferingID: "offering-a-whole", Type: model.SessionTypeTheory, SessionsPerWeek: 1, Duration: 1, Consecutive: true},
	}
	p.FacultyAvailabilities = append(p.FacultyAvailabilities, AvailabilityForFaculty("faculty-a1")...)
	p.FacultyAvailabilities = append(p.FacultyAvailabilities, AvailabilityForFaculty("faculty-a2")...)
	p.FacultyAvailabilities = append(p.FacultyAvailabilities, AvailabilityForFaculty("faculty-a3")...)
	p.RoomAvailabilities = AvailabilityForRooms("room-lab-1", "room-lab-2", "room-lecture")
	p.FacultyAvailable = nil
	p.RoomAvailable = nil
	p.Prepare()
	return p
}

// AvailabilityForFaculty creates full availability for given faculty across standard slots.
func AvailabilityForFaculty(facultyID model.FacultyID) []model.FacultyAvailability {
	slotIDs := []model.TimeSlotID{"mon-1", "mon-2", "mon-3", "tue-1", "tue-2"}
	availability := make([]model.FacultyAvailability, 0, len(slotIDs))
	for _, slotID := range slotIDs {
		availability = append(availability, model.FacultyAvailability{FacultyID: facultyID, TimeSlotID: slotID})
	}
	return availability
}

// AvailabilityForRooms creates full availability for given rooms across standard slots.
func AvailabilityForRooms(roomIDs ...model.RoomID) []model.RoomAvailability {
	slotIDs := []model.TimeSlotID{"mon-1", "mon-2", "mon-3", "tue-1", "tue-2"}
	availability := make([]model.RoomAvailability, 0, len(slotIDs)*len(roomIDs))
	for _, roomID := range roomIDs {
		for _, slotID := range slotIDs {
			availability = append(availability, model.RoomAvailability{RoomID: roomID, TimeSlotID: slotID})
		}
	}
	return availability
}

// HasViolation checks if a violation of constraintName exists.
func HasViolation(violations []diagnostics.Violation, constraintName string) bool {
	for _, v := range violations {
		if v.ConstraintName == constraintName {
			return true
		}
	}
	return false
}

// HasViolationMessageContaining checks if any violation message contains phrase.
func HasViolationMessageContaining(violations []diagnostics.Violation, phrase string) bool {
	for _, v := range violations {
		if strings.Contains(v.Message, phrase) {
			return true
		}
	}
	return false
}

// AssertAllRequiredSessionsScheduled verifies that all required sessions are present in solution.
func AssertAllRequiredSessionsScheduled(t testing.TB, p *problem.Problem, sol *problem.Solution) {
	t.Helper()
	p.Prepare()
	expected := 0
	for _, offering := range p.CourseOfferings {
		for _, requirementID := range offering.SessionRequirementIDs {
			requirement := p.SessionRequirements[requirementID]
			expected += requirement.SessionsPerWeek
		}
	}
	if len(sol.Assignments) != expected {
		t.Fatalf("expected %d assignments scheduled, got %d", expected, len(sol.Assignments))
	}
}

// AssertSolutionIndexConsistent verifies integrity between solution.Assignments and solution.Index.
func AssertSolutionIndexConsistent(t testing.TB, p *problem.Problem, sol *problem.Solution) {
	t.Helper()
	if sol == nil {
		t.Fatal("AssertSolutionIndexConsistent: solution is nil")
	}

	expectedCounts := make(map[model.SessionRequirementID]int)
	expectedFacultySlots := make(map[string]problem.AssignmentID)
	expectedRoomSlots := make(map[string]problem.AssignmentID)
	expectedGroupSlots := make(map[string]problem.AssignmentID)

	for _, a := range sol.Assignments {
		indexedA, ok := sol.Index.AssignmentByID(a.ID)
		if !ok {
			t.Fatalf("Index missing assignment by ID: %s", a.ID)
		}
		if indexedA != a {
			t.Fatalf("Index byID mismatch for %s:\n  got:  %+v\n  want: %+v", a.ID, indexedA, a)
		}

		expectedCounts[a.SessionRequirementID]++

		slotIDs, fits := a.OccupiedSlotIDs(p)
		if !fits {
			continue
		}
		for _, sid := range slotIDs {
			fKey := fmt.Sprintf("%s|%s", a.FacultyID, sid)
			rKey := fmt.Sprintf("%s|%s", a.RoomID, sid)
			gKey := fmt.Sprintf("%s|%s", a.StudentGroupID, sid)

			expectedFacultySlots[fKey] = a.ID
			expectedRoomSlots[rKey] = a.ID
			expectedGroupSlots[gKey] = a.ID

			if gotID, ok := sol.Index.FacultyConflict(a.FacultyID, []model.TimeSlotID{sid}); !ok || gotID != a.ID {
				t.Fatalf("Index.FacultyConflict mismatch for %s at %s: got (%s, %v), want (%s, true)", a.FacultyID, sid, gotID, ok, a.ID)
			}
			if gotID, ok := sol.Index.RoomConflict(a.RoomID, []model.TimeSlotID{sid}); !ok || gotID != a.ID {
				t.Fatalf("Index.RoomConflict mismatch for %s at %s: got (%s, %v), want (%s, true)", a.RoomID, sid, gotID, ok, a.ID)
			}
			if gotID, ok := sol.Index.StudentGroupConflict(a.StudentGroupID, []model.TimeSlotID{sid}); !ok || gotID != a.ID {
				t.Fatalf("Index.StudentGroupConflict mismatch for %s at %s: got (%s, %v), want (%s, true)", a.StudentGroupID, sid, gotID, ok, a.ID)
			}
		}
	}

	for reqID, wantCount := range expectedCounts {
		if gotCount := sol.Index.ScheduledCount(reqID); gotCount != wantCount {
			t.Fatalf("Index.ScheduledCount mismatch for %s: got %d, want %d", reqID, gotCount, wantCount)
		}
	}
}

// StressBaseProblem builds a parameterized stress/pathological test problem.
func StressBaseProblem(roomCount, slotCount, sessionCount int) problem.Problem {
	tenantID := model.TenantID("tenant-stress")
	deptID := model.DepartmentID("dept-1")
	progID := model.ProgramID("prog-1")
	classID := model.ClassID("class-1")
	groupID := model.StudentGroupID("group-1")
	subjID := model.SubjectID("subj-1")
	offeringID := model.CourseOfferingID("offering-1")
	reqID := model.SessionRequirementID("req-1")
	facID := model.FacultyID("faculty-1")

	p := problem.Problem{
		TenantID:      tenantID,
		Term:          model.Term{ID: "term-1", TenantID: tenantID, Name: "Term 1"},
		PeriodsPerDay: slotCount,
		Departments: map[model.DepartmentID]model.Department{
			deptID: {ID: deptID, TenantID: tenantID, Name: "Dept"},
		},
		Programs: map[model.ProgramID]model.Program{
			progID: {ID: progID, DepartmentID: deptID, Name: "Prog"},
		},
		Classes: map[model.ClassID]model.Class{
			classID: {ID: classID, ProgramID: progID, Name: "Class", WholeGroupID: groupID, StudentGroupIDs: []model.StudentGroupID{groupID}},
		},
		StudentGroups: map[model.StudentGroupID]model.StudentGroup{
			groupID: {ID: groupID, ClassID: classID, Name: "Group", Size: 30},
		},
		Subjects: map[model.SubjectID]model.Subject{
			subjID: {ID: subjID, Name: "Subj", Code: "S1"},
		},
		Faculty: map[model.FacultyID]model.Faculty{
			facID: {ID: facID, Name: "Prof 1"},
		},
		CourseOfferings: map[model.CourseOfferingID]model.CourseOffering{
			offeringID: {
				ID:                    offeringID,
				TermID:                "term-1",
				ClassID:               classID,
				SubjectID:             subjID,
				StudentGroupID:        groupID,
				FacultyID:             facID,
				SessionRequirementIDs: []model.SessionRequirementID{reqID},
			},
		},
		SessionRequirements: map[model.SessionRequirementID]model.SessionRequirement{
			reqID: {
				ID:               reqID,
				CourseOfferingID: offeringID,
				Type:             model.SessionTypeTheory,
				Duration:         1,
				SessionsPerWeek:  sessionCount,
			},
		},
		Rooms:     make(map[model.RoomID]model.Room),
		TimeSlots: make(map[model.TimeSlotID]model.TimeSlot),
	}

	for r := 1; r <= roomCount; r++ {
		rID := model.RoomID(fmt.Sprintf("room-%d", r))
		p.Rooms[rID] = model.Room{ID: rID, Name: fmt.Sprintf("Room %d", r), Capacity: 50}
	}
	for s := 1; s <= slotCount; s++ {
		sID := model.TimeSlotID(fmt.Sprintf("slot-%d", s))
		p.TimeSlots[sID] = model.TimeSlot{ID: sID, Day: time.Monday, Period: s}
		p.FacultyAvailabilities = append(p.FacultyAvailabilities, model.FacultyAvailability{
			FacultyID:  facID,
			TimeSlotID: sID,
		})
		for r := 1; r <= roomCount; r++ {
			rID := model.RoomID(fmt.Sprintf("room-%d", r))
			p.RoomAvailabilities = append(p.RoomAvailabilities, model.RoomAvailability{
				RoomID:     rID,
				TimeSlotID: sID,
			})
		}
	}

	return p
}
