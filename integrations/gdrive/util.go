package gdrive

import (
	"crypto/rand"
	"encoding/hex"
)

// generateRequestID produces a unique idempotency key for shared-drive
// creation. The Drive API requires `requestId` to be unique per drive
// creation; collisions return the previously-created drive instead of
// erroring. We use 16 hex chars of randomness, which is more than enough
// for the per-user collision space we care about.
func generateRequestID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return "switchboard-" + hex.EncodeToString(b[:])
}
