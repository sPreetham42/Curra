package repositories

import "errors"

var (
	ErrOptimisticLock = errors.New("optimistic lock conflict: resource was modified by another request")
	ErrNotFound       = errors.New("resource not found")
	ErrStaleWorker    = errors.New("stale worker: lease expired or owned by another worker")
)
