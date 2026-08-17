package project

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/gofrs/flock"
)

const (
	catalogLockName    = ".catalog.lock"
	defaultLockTimeout = 10 * time.Second
	lockRetryInterval  = 20 * time.Millisecond
)

func (s *Store) lockPath() string {
	return filepath.Join(s.configDir, catalogLockName)
}

func (s *Store) withLock(ctx context.Context, fn func() error) error {
	if err := ctx.Err(); err != nil {
		return &Error{Code: CodeLockTimeout, Message: err.Error()}
	}
	if err := os.MkdirAll(s.configDir, 0700); err != nil {
		return &Error{Code: CodeInternalError, Message: err.Error()}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	fl := flock.New(s.lockPath())
	deadline := time.Now().Add(defaultLockTimeout)
	if dl, ok := ctx.Deadline(); ok && dl.Before(deadline) {
		deadline = dl
	}

	for {
		if err := ctx.Err(); err != nil {
			return &Error{Code: CodeLockTimeout, Message: err.Error()}
		}
		locked, err := fl.TryLock()
		if err != nil {
			return &Error{Code: CodeInternalError, Message: "catalog lock: " + err.Error()}
		}
		if locked {
			defer func() { _ = fl.Unlock() }()
			return fn()
		}
		if time.Now().After(deadline) {
			return &Error{Code: CodeLockTimeout, Message: "catalog lock timeout"}
		}
		select {
		case <-ctx.Done():
			return &Error{Code: CodeLockTimeout, Message: ctx.Err().Error()}
		case <-time.After(lockRetryInterval):
		}
	}
}
