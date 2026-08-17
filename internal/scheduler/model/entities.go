package model

type SessionType string

const (
	SessionTypeTheory SessionType = "THEORY"
	SessionTypeLab    SessionType = "LAB"
)

type Department struct {
	ID       DepartmentID
	TenantID TenantID
	Name     string
}

type Program struct {
	ID           ProgramID
	DepartmentID DepartmentID
	Name         string
}

type Class struct {
	ID        ClassID
	ProgramID ProgramID
	Name      string
	// WholeGroupID is the StudentGroup representing the full class cohort.
	WholeGroupID StudentGroupID
	// StudentGroupIDs includes WholeGroupID and any smaller lab/tutorial groups.
	StudentGroupIDs []StudentGroupID
}

type StudentGroup struct {
	ID      StudentGroupID
	ClassID ClassID
	Name    string
	Size    int
}

type Subject struct {
	ID   SubjectID
	Code string
	Name string
}

type SessionRequirement struct {
	ID                     SessionRequirementID
	CourseOfferingID       CourseOfferingID
	Type                   SessionType
	SessionsPerWeek        int
	Duration               int
	Consecutive            bool
	RequiredRoomFeatureIDs []RoomFeatureID
}

type CourseOffering struct {
	ID                     CourseOfferingID
	TermID                 TermID
	ClassID                ClassID
	SubjectID              SubjectID
	StudentGroupID         StudentGroupID
	FacultyID              FacultyID
	RequiredRoomFeatureIDs []RoomFeatureID
	SessionRequirementIDs  []SessionRequirementID
}

type Faculty struct {
	ID   FacultyID
	Name string
}

type FacultyAvailability struct {
	FacultyID  FacultyID
	TimeSlotID TimeSlotID
}

type FacultyPreference struct {
	FacultyID  FacultyID
	TimeSlotID TimeSlotID
	Weight     int
}

type RoomFeature struct {
	ID   RoomFeatureID
	Name string
}

type Room struct {
	ID         RoomID
	Name       string
	Capacity   int
	FeatureIDs []RoomFeatureID
}

type RoomAvailability struct {
	RoomID     RoomID
	TimeSlotID TimeSlotID
}

type Term struct {
	ID       TermID
	TenantID TenantID
	Name     string
}
