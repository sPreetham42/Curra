package services

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/sPreetham42/timetable-platform/application/internal/database/repositories"
	"github.com/sPreetham42/timetable-platform/application/internal/domain"
)

type resourceMockRepos struct {
	rooms                  map[uuid.UUID]domain.Room
	roomFeatures           map[uuid.UUID]domain.RoomFeature
	roomFeatureAssignments map[string]bool
}

func newResourceMockRepos() *resourceMockRepos {
	return &resourceMockRepos{
		rooms:                  make(map[uuid.UUID]domain.Room),
		roomFeatures:           make(map[uuid.UUID]domain.RoomFeature),
		roomFeatureAssignments: make(map[string]bool),
	}
}

type mockRoomRepo struct{ m *resourceMockRepos }

func (r *mockRoomRepo) Create(ctx context.Context, room domain.Room) error {
	r.m.rooms[room.ID] = room
	return nil
}

func (r *mockRoomRepo) GetByID(ctx context.Context, id uuid.UUID) (domain.Room, error) {
	room, ok := r.m.rooms[id]
	if !ok {
		return domain.Room{}, repositories.ErrNotFound
	}
	return room, nil
}

func (r *mockRoomRepo) ListByInstitution(ctx context.Context, instID uuid.UUID) ([]domain.Room, error) {
	var out []domain.Room
	for _, room := range r.m.rooms {
		if room.InstitutionID == instID {
			out = append(out, room)
		}
	}
	return out, nil
}

func (r *mockRoomRepo) Update(ctx context.Context, room domain.Room) error {
	existing, ok := r.m.rooms[room.ID]
	if !ok {
		return repositories.ErrNotFound
	}
	if existing.Version != room.Version {
		return repositories.ErrOptimisticLock
	}
	room.Version++
	r.m.rooms[room.ID] = room
	return nil
}

func (r *mockRoomRepo) Delete(ctx context.Context, id uuid.UUID) error {
	if _, ok := r.m.rooms[id]; !ok {
		return repositories.ErrNotFound
	}
	delete(r.m.rooms, id)
	return nil
}

type mockRoomFeatureRepo struct{ m *resourceMockRepos }

func (r *mockRoomFeatureRepo) Create(ctx context.Context, rf domain.RoomFeature) error {
	r.m.roomFeatures[rf.ID] = rf
	return nil
}

func (r *mockRoomFeatureRepo) GetByID(ctx context.Context, id uuid.UUID) (domain.RoomFeature, error) {
	feature, ok := r.m.roomFeatures[id]
	if !ok {
		return domain.RoomFeature{}, repositories.ErrNotFound
	}
	return feature, nil
}

func (r *mockRoomFeatureRepo) ListByInstitution(ctx context.Context, instID uuid.UUID) ([]domain.RoomFeature, error) {
	var out []domain.RoomFeature
	for _, feature := range r.m.roomFeatures {
		if feature.InstitutionID == instID {
			out = append(out, feature)
		}
	}
	return out, nil
}

func (r *mockRoomFeatureRepo) Update(ctx context.Context, rf domain.RoomFeature) error {
	existing, ok := r.m.roomFeatures[rf.ID]
	if !ok {
		return repositories.ErrNotFound
	}
	if existing.Version != rf.Version {
		return repositories.ErrOptimisticLock
	}
	rf.Version++
	r.m.roomFeatures[rf.ID] = rf
	return nil
}

func (r *mockRoomFeatureRepo) Delete(ctx context.Context, id uuid.UUID) error {
	if _, ok := r.m.roomFeatures[id]; !ok {
		return repositories.ErrNotFound
	}
	delete(r.m.roomFeatures, id)
	return nil
}

type mockRoomFeatureAssignmentRepo struct{ m *resourceMockRepos }

func (r *mockRoomFeatureAssignmentRepo) Create(ctx context.Context, roomID, featureID uuid.UUID) error {
	key := roomID.String() + ":" + featureID.String()
	r.m.roomFeatureAssignments[key] = true
	return nil
}

func (r *mockRoomFeatureAssignmentRepo) Delete(ctx context.Context, roomID, featureID uuid.UUID) error {
	key := roomID.String() + ":" + featureID.String()
	if _, ok := r.m.roomFeatureAssignments[key]; !ok {
		return repositories.ErrNotFound
	}
	delete(r.m.roomFeatureAssignments, key)
	return nil
}

func (r *mockRoomFeatureAssignmentRepo) ListByRoom(ctx context.Context, roomID uuid.UUID) ([]domain.RoomFeature, error) {
	var out []domain.RoomFeature
	prefix := roomID.String() + ":"
	for _, feature := range r.m.roomFeatures {
		key := prefix + feature.ID.String()
		if r.m.roomFeatureAssignments[key] {
			out = append(out, feature)
		}
	}
	return out, nil
}

func buildResourceRepos(m *resourceMockRepos) *repositories.Repos {
	return &repositories.Repos{
		Rooms:                  &mockRoomRepo{m: m},
		RoomFeatures:           &mockRoomFeatureRepo{m: m},
		RoomFeatureAssignments: &mockRoomFeatureAssignmentRepo{m: m},
	}
}

func resourceCtx(instID, userID uuid.UUID, role domain.Role) context.Context {
	ctx := context.Background()
	ctx = ContextWithInstitution(ctx, domain.Institution{ID: instID, Name: "Test University", Slug: "test-univ"})
	ctx = ContextWithUser(ctx, domain.User{ID: userID, Email: "user@test.edu", Name: "Test User"})
	ctx = ContextWithRole(ctx, domain.UserRole{UserID: userID, InstitutionID: instID, Role: role})
	return ctx
}

func TestPhase4B_Room_CRUD(t *testing.T) {
	m := newResourceMockRepos()
	repos := buildResourceRepos(m)
	catalogSvc := NewCatalogService(repos)

	instID := uuid.New()
	userID := uuid.New()
	ctx := resourceCtx(instID, userID, domain.RoleInstitutionAdmin)

	roomID := uuid.New()
	room := domain.Room{
		ID:            roomID,
		InstitutionID: instID,
		Name:          "Room A101",
		Capacity:      40,
		Version:       1,
	}

	if err := repos.Rooms.Create(ctx, room); err != nil {
		t.Fatalf("create room failed: %v", err)
	}

	got, err := catalogSvc.GetRoom(ctx, instID, roomID)
	if err != nil {
		t.Fatalf("get room failed: %v", err)
	}
	if got.Name != "Room A101" {
		t.Fatalf("expected room name 'Room A101', got '%s'", got.Name)
	}
	if got.Capacity != 40 {
		t.Fatalf("expected capacity 40, got %d", got.Capacity)
	}

	updated, err := catalogSvc.UpdateRoom(ctx, instID, roomID, "Room A102", 50, 1)
	if err != nil {
		t.Fatalf("update room failed: %v", err)
	}
	if updated.Name != "Room A102" {
		t.Fatalf("expected updated name 'Room A102', got '%s'", updated.Name)
	}
	if updated.Capacity != 50 {
		t.Fatalf("expected updated capacity 50, got %d", updated.Capacity)
	}
	if updated.Version != 2 {
		t.Fatalf("expected version 2 after update, got %d", updated.Version)
	}

	if err := catalogSvc.DeleteRoom(ctx, instID, roomID, 2); err != nil {
		t.Fatalf("delete room failed: %v", err)
	}

	_, err = catalogSvc.GetRoom(ctx, instID, roomID)
	if !errors.Is(err, repositories.ErrNotFound) {
		t.Fatalf("expected ErrNotFound after delete, got: %v", err)
	}
}

func TestPhase4B_Room_VersionConflict(t *testing.T) {
	m := newResourceMockRepos()
	repos := buildResourceRepos(m)
	catalogSvc := NewCatalogService(repos)

	instID := uuid.New()
	userID := uuid.New()
	ctx := resourceCtx(instID, userID, domain.RoleInstitutionAdmin)

	roomID := uuid.New()
	room := domain.Room{
		ID:            roomID,
		InstitutionID: instID,
		Name:          "Room B201",
		Capacity:      30,
		Version:       5,
	}
	if err := repos.Rooms.Create(ctx, room); err != nil {
		t.Fatalf("create room failed: %v", err)
	}

	_, err := catalogSvc.UpdateRoom(ctx, instID, roomID, "Room B202", 35, 4)
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("expected ErrConflict on stale update, got: %v", err)
	}

	err = catalogSvc.DeleteRoom(ctx, instID, roomID, 4)
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("expected ErrConflict on stale delete, got: %v", err)
	}
}

func TestPhase4B_RoomFeature_CRUD(t *testing.T) {
	m := newResourceMockRepos()
	repos := buildResourceRepos(m)
	catalogSvc := NewCatalogService(repos)

	instID := uuid.New()
	userID := uuid.New()
	ctx := resourceCtx(instID, userID, domain.RoleInstitutionAdmin)

	featureID := uuid.New()
	feature := domain.RoomFeature{
		ID:            featureID,
		InstitutionID: instID,
		Name:          "PROJECTOR",
		Version:       1,
	}
	if err := repos.RoomFeatures.Create(ctx, feature); err != nil {
		t.Fatalf("create room feature failed: %v", err)
	}

	got, err := catalogSvc.GetRoomFeature(ctx, instID, featureID)
	if err != nil {
		t.Fatalf("get room feature failed: %v", err)
	}
	if got.Name != "PROJECTOR" {
		t.Fatalf("expected name 'PROJECTOR', got '%s'", got.Name)
	}

	updated, err := catalogSvc.UpdateRoomFeature(ctx, instID, featureID, "LCD_PROJECTOR", 1)
	if err != nil {
		t.Fatalf("update room feature failed: %v", err)
	}
	if updated.Name != "LCD_PROJECTOR" {
		t.Fatalf("expected updated name 'LCD_PROJECTOR', got '%s'", updated.Name)
	}
	if updated.Version != 2 {
		t.Fatalf("expected version 2 after update, got %d", updated.Version)
	}

	if err := catalogSvc.DeleteRoomFeature(ctx, instID, featureID, 2); err != nil {
		t.Fatalf("delete room feature failed: %v", err)
	}

	_, err = catalogSvc.GetRoomFeature(ctx, instID, featureID)
	if !errors.Is(err, repositories.ErrNotFound) {
		t.Fatalf("expected ErrNotFound after delete, got: %v", err)
	}
}

func TestPhase4B_RoomFeatureAssignment(t *testing.T) {
	m := newResourceMockRepos()
	repos := buildResourceRepos(m)
	catalogSvc := NewCatalogService(repos)

	instID := uuid.New()
	userID := uuid.New()
	ctx := resourceCtx(instID, userID, domain.RoleInstitutionAdmin)

	roomID := uuid.New()
	room := domain.Room{ID: roomID, InstitutionID: instID, Name: "Lab 101", Capacity: 20, Version: 1}
	if err := repos.Rooms.Create(ctx, room); err != nil {
		t.Fatalf("create room failed: %v", err)
	}

	featureID := uuid.New()
	feature := domain.RoomFeature{ID: featureID, InstitutionID: instID, Name: "LAB_EQUIPMENT", Version: 1}
	if err := repos.RoomFeatures.Create(ctx, feature); err != nil {
		t.Fatalf("create room feature failed: %v", err)
	}

	if err := catalogSvc.AssignRoomFeature(ctx, instID, roomID, featureID); err != nil {
		t.Fatalf("assign room feature failed: %v", err)
	}

	features, err := catalogSvc.GetRoomFeatures(ctx, instID, roomID)
	if err != nil {
		t.Fatalf("get room features failed: %v", err)
	}
	if len(features) != 1 {
		t.Fatalf("expected 1 feature, got %d", len(features))
	}
	if features[0].Name != "LAB_EQUIPMENT" {
		t.Fatalf("expected feature name 'LAB_EQUIPMENT', got '%s'", features[0].Name)
	}

	if err := catalogSvc.UnassignRoomFeature(ctx, instID, roomID, featureID); err != nil {
		t.Fatalf("unassign room feature failed: %v", err)
	}

	features, err = catalogSvc.GetRoomFeatures(ctx, instID, roomID)
	if err != nil {
		t.Fatalf("get room features after unassign failed: %v", err)
	}
	if len(features) != 0 {
		t.Fatalf("expected 0 features after unassign, got %d", len(features))
	}
}

func TestPhase4B_RoomFeatureAssignment_CrossTenant(t *testing.T) {
	m := newResourceMockRepos()
	repos := buildResourceRepos(m)
	catalogSvc := NewCatalogService(repos)

	instA := uuid.New()
	userID := uuid.New()
	ctxA := resourceCtx(instA, userID, domain.RoleInstitutionAdmin)

	roomB := uuid.New()
	repos.Rooms.Create(ctxA, domain.Room{ID: roomB, InstitutionID: instA, Name: "Room B", Capacity: 10, Version: 1})

	featureA := uuid.New()
	repos.RoomFeatures.Create(ctxA, domain.RoomFeature{ID: featureA, InstitutionID: instA, Name: "PROJECTOR", Version: 1})

	// Feature belongs to another tenant
	otherInst := uuid.New()
	foreignFeature := uuid.New()
	repos.RoomFeatures.Create(ctxA, domain.RoomFeature{ID: foreignFeature, InstitutionID: otherInst, Name: "FOREIGN", Version: 1})

	err := catalogSvc.AssignRoomFeature(ctxA, instA, roomB, foreignFeature)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound when feature belongs to a different tenant, got: %v", err)
	}

	// Room belongs to another tenant
	foreignRoom := uuid.New()
	repos.Rooms.Create(ctxA, domain.Room{ID: foreignRoom, InstitutionID: otherInst, Name: "Foreign Room", Capacity: 10, Version: 1})
	err = catalogSvc.AssignRoomFeature(ctxA, instA, foreignRoom, featureA)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound when room belongs to a different tenant, got: %v", err)
	}
}

func TestPhase4B_RoomFeatureAssignment_NotFound(t *testing.T) {
	m := newResourceMockRepos()
	repos := buildResourceRepos(m)
	catalogSvc := NewCatalogService(repos)

	instID := uuid.New()
	userID := uuid.New()
	ctx := resourceCtx(instID, userID, domain.RoleInstitutionAdmin)

	roomID := uuid.New()
	nonExistentFeature := uuid.New()

	_, err := catalogSvc.GetRoomFeatures(ctx, instID, roomID)
	if !errors.Is(err, repositories.ErrNotFound) {
		t.Fatalf("expected ErrNotFound when room doesn't exist, got: %v", err)
	}

	err = catalogSvc.AssignRoomFeature(ctx, instID, roomID, nonExistentFeature)
	if !errors.Is(err, repositories.ErrNotFound) {
		t.Fatalf("expected ErrNotFound when feature doesn't exist, got: %v", err)
	}

	err = catalogSvc.UnassignRoomFeature(ctx, instID, roomID, nonExistentFeature)
	if !errors.Is(err, repositories.ErrNotFound) {
		t.Fatalf("expected ErrNotFound when unassign non-existent assignment, got: %v", err)
	}
}

func TestPhase4B_RBAC_RoomManagement(t *testing.T) {
	m := newResourceMockRepos()
	repos := buildResourceRepos(m)
	catalogSvc := NewCatalogService(repos)

	instID := uuid.New()
	userID := uuid.New()

	viewerCtx := resourceCtx(instID, userID, domain.RoleViewer)
	schedulerCtx := resourceCtx(instID, userID, domain.RoleScheduler)
	adminCtx := resourceCtx(instID, userID, domain.RoleInstitutionAdmin)

	roomID := uuid.New()
	if err := repos.Rooms.Create(adminCtx, domain.Room{ID: roomID, InstitutionID: instID, Name: "Room C301", Capacity: 25, Version: 1}); err != nil {
		t.Fatalf("create room failed: %v", err)
	}

	_, err := catalogSvc.CreateRoom(viewerCtx, instID, "Viewer Room", 10)
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected ErrForbidden for viewer creating room, got: %v", err)
	}

	_, err = catalogSvc.UpdateRoom(viewerCtx, instID, roomID, "Updated", 30, 1)
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected ErrForbidden for viewer updating room, got: %v", err)
	}

	err = catalogSvc.DeleteRoom(viewerCtx, instID, roomID, 1)
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected ErrForbidden for viewer deleting room, got: %v", err)
	}

	_, err = catalogSvc.CreateRoom(schedulerCtx, instID, "Scheduler Room", 15)
	if err != nil {
		t.Fatalf("scheduler should be able to create room, got: %v", err)
	}

	_, err = catalogSvc.CreateRoom(adminCtx, instID, "Admin Room", 20)
	if err != nil {
		t.Fatalf("admin should be able to create room, got: %v", err)
	}
}