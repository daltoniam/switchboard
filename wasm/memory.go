package wasm

import (
	"context"
	"fmt"

	"github.com/tetratelabs/wazero/api"
)

// writeToGuest allocates memory in the guest via malloc and copies data into it.
// Returns the guest pointer and size. The caller must free the pointer when done.
func writeToGuest(ctx context.Context, mod api.Module, data []byte) (uint32, uint32, error) {
	if len(data) == 0 {
		return 0, 0, nil
	}

	malloc := mod.ExportedFunction("malloc")
	if malloc == nil {
		malloc = mod.ExportedFunction("guest_malloc")
	}
	if malloc == nil {
		return 0, 0, fmt.Errorf("wasm module does not export malloc")
	}

	size := uint64(len(data))
	results, err := malloc.Call(ctx, size)
	if err != nil {
		return 0, 0, fmt.Errorf("malloc(%d): %w", size, err)
	}
	ptr := uint32(results[0])
	if ptr == 0 {
		return 0, 0, fmt.Errorf("malloc(%d) returned null", size)
	}

	if !mod.Memory().Write(ptr, data) {
		return 0, 0, fmt.Errorf("memory write at %d (size %d) out of range", ptr, size)
	}

	return ptr, uint32(len(data)), nil
}

// readFromGuest reads bytes from guest linear memory at the given offset and length.
func readFromGuest(mod api.Module, ptr, size uint32) ([]byte, error) {
	if size == 0 {
		return nil, nil
	}
	data, ok := mod.Memory().Read(ptr, size)
	if !ok {
		return nil, fmt.Errorf("memory read at %d (size %d) out of range", ptr, size)
	}
	cp := make([]byte, len(data))
	copy(cp, data)
	return cp, nil
}

// freeInGuest frees previously allocated guest memory.
func freeInGuest(ctx context.Context, mod api.Module, ptr uint32) {
	if ptr == 0 {
		return
	}
	free := mod.ExportedFunction("free")
	if free == nil {
		free = mod.ExportedFunction("guest_free")
	}
	if free == nil {
		return
	}
	_, _ = free.Call(ctx, uint64(ptr))
}

// packPtrSize packs a pointer and size into a single uint64 (ptr in high 32, size in low 32).
func packPtrSize(ptr, size uint32) uint64 {
	return (uint64(ptr) << 32) | uint64(size)
}

// unpackPtrSize unpacks a uint64 into a pointer and size.
func unpackPtrSize(v uint64) (ptr, size uint32) {
	return uint32(v >> 32), uint32(v)
}
