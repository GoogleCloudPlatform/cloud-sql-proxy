package proxy

import (
	"testing"
	"unsafe"
)

func TestClientUsesSyncAtomicAlignment(t *testing.T) {
	// The sync/atomic pkg has a bug that requires the developer to guarantee
	// 64-bit alignment when using 64-bit functions on 32-bit systems.
	c := &Client{}

	if a := unsafe.Offsetof(c.connCount); a%64 != 0 {
		t.Errorf("Client.ConnectionsCounter is not aligned: want %v, got %v", 0, a)
	}
}
