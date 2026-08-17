package constraints

import (
	"sort"

	"github.com/sPreetham42/timetable-platform/internal/scheduler/model"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/problem"
)

// MemberRef represents an abstract member reference (e.g. student or subgroup reference).
type MemberRef string

// MembershipIndex defines the interface for student group membership and overlap queries.
type MembershipIndex interface {
	GroupsOverlap(a, b model.StudentGroupID) bool
	Members(g model.StudentGroupID) MemberSet
}

// MemberSet represents a set of member references.
type MemberSet interface {
	Contains(m MemberRef) bool
	Iterate(func(MemberRef))
	Cardinality() int
}

// HierarchyMembershipIndex implements MembershipIndex using the problem's hierarchy/group data.
type HierarchyMembershipIndex struct {
	p *problem.Problem
}

// NewHierarchyMembershipIndex creates a MembershipIndex backed by existing group/hierarchy data.
func NewHierarchyMembershipIndex(p *problem.Problem) *HierarchyMembershipIndex {
	return &HierarchyMembershipIndex{p: p}
}

// GroupsOverlap returns true if group a and group b share students according to hierarchy data.
func (h *HierarchyMembershipIndex) GroupsOverlap(a, b model.StudentGroupID) bool {
	if a == b {
		return true
	}
	if h.p == nil {
		return false
	}
	if overlaps, ok := h.p.StudentGroupOverlaps[a]; ok {
		_, ok := overlaps[b]
		return ok
	}
	return false
}

// Members returns the MemberSet for student group g.
func (h *HierarchyMembershipIndex) Members(g model.StudentGroupID) MemberSet {
	members := make(map[MemberRef]struct{})
	members[MemberRef(g)] = struct{}{}
	if h.p != nil {
		if group, ok := h.p.StudentGroups[g]; ok {
			if class, ok := h.p.Classes[group.ClassID]; ok && class.WholeGroupID == g {
				for _, sgID := range class.StudentGroupIDs {
					members[MemberRef(sgID)] = struct{}{}
				}
			}
		}
	}
	return &hierarchyMemberSet{members: members}
}

type hierarchyMemberSet struct {
	members map[MemberRef]struct{}
}

func (s *hierarchyMemberSet) Contains(m MemberRef) bool {
	_, ok := s.members[m]
	return ok
}

func (s *hierarchyMemberSet) Iterate(fn func(MemberRef)) {
	sorted := make([]MemberRef, 0, len(s.members))
	for m := range s.members {
		sorted = append(sorted, m)
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i] < sorted[j]
	})
	for _, m := range sorted {
		fn(m)
	}
}

func (s *hierarchyMemberSet) Cardinality() int {
	return len(s.members)
}
