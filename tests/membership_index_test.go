package tests

import (
	"testing"
	"time"

	"github.com/sPreetham42/timetable-platform/internal/scheduler/constraints"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/model"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/problem"
)

func TestMembershipIndex_HierarchyBackedBehavior(t *testing.T) {
	p := problem.Problem{
		TenantID: "tenant-mem",
		Term:     model.Term{ID: "term-1", TenantID: "tenant-mem", Name: "Term 1"},
		Classes: map[model.ClassID]model.Class{
			"class-1": {
				ID:              "class-1",
				WholeGroupID:    "group-whole",
				StudentGroupIDs: []model.StudentGroupID{"group-whole", "group-lab1", "group-lab2"},
			},
		},
		StudentGroups: map[model.StudentGroupID]model.StudentGroup{
			"group-whole": {ID: "group-whole", ClassID: "class-1", Name: "Whole Group", Size: 40},
			"group-lab1":  {ID: "group-lab1", ClassID: "class-1", Name: "Lab Group 1", Size: 20},
			"group-lab2":  {ID: "group-lab2", ClassID: "class-1", Name: "Lab Group 2", Size: 20},
			"group-other": {ID: "group-other", ClassID: "class-2", Name: "Other Group", Size: 30},
		},
		TimeSlots: map[model.TimeSlotID]model.TimeSlot{
			"slot-1": {ID: "slot-1", Day: time.Monday, Period: 1},
		},
	}
	p.Prepare()

	idx := constraints.NewHierarchyMembershipIndex(&p)

	// 1. Test GroupsOverlap
	if !idx.GroupsOverlap("group-whole", "group-lab1") {
		t.Fatal("expected group-whole and group-lab1 to overlap")
	}
	if !idx.GroupsOverlap("group-lab1", "group-whole") {
		t.Fatal("expected group-lab1 and group-whole to overlap")
	}
	if idx.GroupsOverlap("group-lab1", "group-lab2") {
		t.Fatal("expected disjoint lab groups lab1 and lab2 not to overlap")
	}
	if idx.GroupsOverlap("group-whole", "group-other") {
		t.Fatal("expected group-whole and group-other not to overlap")
	}
	if !idx.GroupsOverlap("group-lab1", "group-lab1") {
		t.Fatal("expected group-lab1 to overlap with itself")
	}

	// 2. Test Members & Cardinality
	wholeSet := idx.Members("group-whole")
	if wholeSet.Cardinality() < 3 {
		t.Fatalf("expected wholeSet cardinality >= 3, got %d", wholeSet.Cardinality())
	}

	lab1Set := idx.Members("group-lab1")
	if lab1Set.Cardinality() < 1 {
		t.Fatalf("expected lab1Set cardinality >= 1, got %d", lab1Set.Cardinality())
	}
	if !lab1Set.Contains(constraints.MemberRef("group-lab1")) {
		t.Fatal("expected lab1Set to contain group-lab1 member ref")
	}

	// 3. Test Iterate
	visited := make(map[constraints.MemberRef]bool)
	wholeSet.Iterate(func(m constraints.MemberRef) {
		visited[m] = true
	})

	if !visited["group-whole"] || !visited["group-lab1"] || !visited["group-lab2"] {
		t.Fatalf("expected iterate to visit all members, visited: %+v", visited)
	}
}
