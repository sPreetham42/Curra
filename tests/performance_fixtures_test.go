package tests

import (
	"fmt"
	"math/rand"
	"time"

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
