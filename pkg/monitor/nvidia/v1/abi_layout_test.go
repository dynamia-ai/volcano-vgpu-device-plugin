package v1

import (
	"testing"
	"unsafe"
)

// These offsets are from HAMi-core b216ba1's shared_region_t on Linux.
// The monitor reads the mapped cache directly, so an ABI drift is unsafe.
func TestSharedRegionLayout(t *testing.T) {
	checks := []struct {
		name      string
		got, want uintptr
	}{
		{"region size", SharedRegionSize, 2008952},
		{"semaphore", unsafe.Offsetof(sharedRegionT{}.sem), 24},
		{"device count", unsafe.Offsetof(sharedRegionT{}.num), 56},
		{"process slots", unsafe.Offsetof(sharedRegionT{}.procs), 1856},
		{"process slot size", unsafe.Sizeof(shrregProcSlotT{}), 1960},
		{"process count", unsafe.Offsetof(sharedRegionT{}.procnum), 2008896},
		{"last kernel time", unsafe.Offsetof(sharedRegionT{}.lastKernelTime), 2008912},
		{"postinit semaphore", unsafe.Offsetof(sharedRegionT{}.semPostinit), 2008920},
	}
	for _, check := range checks {
		if check.got != check.want {
			t.Errorf("%s offset/size = %d, want %d", check.name, check.got, check.want)
		}
	}
}
