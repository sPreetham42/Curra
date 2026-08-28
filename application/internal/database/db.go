package database

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
)

type DB struct {
	Pool   *pgxpool.Pool
	logger *slog.Logger
}

func New(ctx context.Context, cfg Config, logger *slog.Logger) (*DB, error) {
	poolCfg, err := pgxpool.ParseConfig(cfg.URL())
	if err != nil {
		return nil, fmt.Errorf("parse database config: %w", err)
	}

	poolCfg.MaxConns = 20
	poolCfg.MinConns = 2

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("create connection pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	logger.Info("database connected", "host", cfg.Host, "name", cfg.Name)
	return &DB{Pool: pool, logger: logger}, nil
}

func (db *DB) Close() {
	db.Pool.Close()
}

func (db *DB) Ping(ctx context.Context) error {
	return db.Pool.Ping(ctx)
}

func (db *DB) RunMigrations(ctx context.Context) error {
	migrations := []string{
		createInstitutions,
		createUsers,
		createUserRoles,
		createDepartments,
		createPrograms,
		createClasses,
		createStudentGroups,
		createSubjects,
		createFaculty,
		createRooms,
		createRoomFeatures,
		createRoomFeatureAssignments,
		createTimeSlots,
		createAcademicYears,
		createTerms,
		createCourseOfferings,
		createCourseOfferingFeatures,
		createSessionRequirements,
		createSessionRequirementFeatures,
		createFacultyAvailability,
		createFacultyPreferences,
		createRoomAvailability,
		createTimetables,
		createProblemSnapshots,
		createSnapshotImmutabilityTrigger,
		createConstraintRulesets,
		createScheduleRuns,
		createScheduleVersions,
		createScheduleAssignments,
		createAssignmentPins,
		createImportBatches,
		createImportRows,
		createAuditEvents,
		createIdempotencyKeys,
	}

	for i, m := range migrations {
		if _, err := db.Pool.Exec(ctx, m); err != nil {
			return fmt.Errorf("migration %d failed: %w", i, err)
		}
	}

	db.logger.Info("database migrations complete", "count", len(migrations))
	return nil
}
