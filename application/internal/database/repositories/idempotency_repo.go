package repositories

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sPreetham42/timetable-platform/application/internal/domain"
)

var ErrIdempotencyConflict = errors.New("concurrent request in progress for this idempotency key")

type IdempotencyRepo interface {
	// Acquire attempts to acquire an idempotency lock for (institution_id, key).
	// If the key is already COMPLETED, isCompleted=true and keyRecord contains the saved response.
	// If acquired (new row or reclaimed stale lock), isCompleted=false and keyRecord is returned with its LockToken.
	// If an active in-progress lock exists with a different lock token, returns ErrIdempotencyConflict.
	Acquire(ctx context.Context, instID uuid.UUID, key, resourceType string) (keyRecord *domain.IdempotencyKey, isCompleted bool, err error)

	// Complete transitions the key to COMPLETED with response_code and response_body matching the lockToken.
	Complete(ctx context.Context, instID uuid.UUID, key string, lockToken *uuid.UUID, resourceID uuid.UUID, responseCode int, responseBody json.RawMessage) error

	// Release removes the idempotency key if resource creation fails so subsequent retries are unblocked immediately.
	Release(ctx context.Context, instID uuid.UUID, key string, lockToken *uuid.UUID) error

	// Get retrieves the existing idempotency key record if present.
	Get(ctx context.Context, instID uuid.UUID, key string) (*domain.IdempotencyKey, error)
}

type idempotencyRepo struct {
	pool *pgxpool.Pool
}

func NewIdempotencyRepo(pool *pgxpool.Pool) IdempotencyRepo {
	return &idempotencyRepo{pool: pool}
}

func (r *idempotencyRepo) Acquire(ctx context.Context, instID uuid.UUID, key, resourceType string) (*domain.IdempotencyKey, bool, error) {
	newID := uuid.New()
	myLockToken := uuid.New()
	var ik domain.IdempotencyKey

	// Atomic insert or conditional update of stale lock (> 1 minute)
	err := r.pool.QueryRow(ctx,
		`INSERT INTO idempotency_keys
		 (id, institution_id, idempotency_key, status, resource_type, lock_token, locked_at, created_at, updated_at)
		 VALUES ($1, $2, $3, 'IN_PROGRESS', $4, $5, now(), now(), now())
		 ON CONFLICT (institution_id, idempotency_key) DO UPDATE
		   SET lock_token = CASE
		         WHEN idempotency_keys.status = 'IN_PROGRESS' AND idempotency_keys.locked_at < now() - INTERVAL '1 minute'
		         THEN $5
		         ELSE idempotency_keys.lock_token
		       END,
		       locked_at = CASE
		         WHEN idempotency_keys.status = 'IN_PROGRESS' AND idempotency_keys.locked_at < now() - INTERVAL '1 minute'
		         THEN now()
		         ELSE idempotency_keys.locked_at
		       END,
		       updated_at = now()
		 RETURNING id, institution_id, idempotency_key, status, resource_type,
		           resource_id, response_code, response_body, lock_token, locked_at, created_at, updated_at`,
		newID, instID, key, resourceType, myLockToken).Scan(
		&ik.ID, &ik.InstitutionID, &ik.IdempotencyKey, &ik.Status, &ik.ResourceType,
		&ik.ResourceID, &ik.ResponseCode, &ik.ResponseBody, &ik.LockToken, &ik.LockedAt, &ik.CreatedAt, &ik.UpdatedAt)

	if err != nil {
		return nil, false, err
	}

	if ik.Status == domain.IdempotencyStatusCompleted {
		return &ik, true, nil
	}

	// Exact ownership identity check: this request owns the lock iff returned LockToken matches myLockToken
	if ik.LockToken != nil && *ik.LockToken == myLockToken {
		return &ik, false, nil
	}

	return nil, false, ErrIdempotencyConflict
}

func (r *idempotencyRepo) Complete(
	ctx context.Context,
	instID uuid.UUID,
	key string,
	lockToken *uuid.UUID,
	resourceID uuid.UUID,
	responseCode int,
	responseBody json.RawMessage,
) error {
	var err error
	if lockToken != nil {
		_, err = r.pool.Exec(ctx,
			`UPDATE idempotency_keys
			 SET status = 'COMPLETED',
			     resource_id = $1,
			     response_code = $2,
			     response_body = $3,
			     updated_at = now()
			 WHERE institution_id = $4 AND idempotency_key = $5 AND (lock_token = $6 OR lock_token IS NULL)`,
			resourceID, responseCode, responseBody, instID, key, *lockToken)
	} else {
		_, err = r.pool.Exec(ctx,
			`UPDATE idempotency_keys
			 SET status = 'COMPLETED',
			     resource_id = $1,
			     response_code = $2,
			     response_body = $3,
			     updated_at = now()
			 WHERE institution_id = $4 AND idempotency_key = $5`,
			resourceID, responseCode, responseBody, instID, key)
	}
	return err
}

func (r *idempotencyRepo) Release(ctx context.Context, instID uuid.UUID, key string, lockToken *uuid.UUID) error {
	var err error
	if lockToken != nil {
		_, err = r.pool.Exec(ctx,
			`DELETE FROM idempotency_keys
			 WHERE institution_id = $1 AND idempotency_key = $2 AND status = 'IN_PROGRESS' AND (lock_token = $3 OR lock_token IS NULL)`,
			instID, key, *lockToken)
	} else {
		_, err = r.pool.Exec(ctx,
			`DELETE FROM idempotency_keys
			 WHERE institution_id = $1 AND idempotency_key = $2 AND status = 'IN_PROGRESS'`,
			instID, key)
	}
	return err
}

func (r *idempotencyRepo) Get(ctx context.Context, instID uuid.UUID, key string) (*domain.IdempotencyKey, error) {
	var ik domain.IdempotencyKey
	err := r.pool.QueryRow(ctx,
		`SELECT id, institution_id, idempotency_key, status, resource_type, resource_id,
		        response_code, response_body, lock_token, locked_at, created_at, updated_at
		 FROM idempotency_keys
		 WHERE institution_id = $1 AND idempotency_key = $2`,
		instID, key).Scan(
		&ik.ID, &ik.InstitutionID, &ik.IdempotencyKey, &ik.Status, &ik.ResourceType,
		&ik.ResourceID, &ik.ResponseCode, &ik.ResponseBody, &ik.LockToken, &ik.LockedAt, &ik.CreatedAt, &ik.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &ik, nil
}
