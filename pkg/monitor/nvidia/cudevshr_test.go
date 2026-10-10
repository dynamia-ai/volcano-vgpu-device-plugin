package nvidia

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	v1 "volcano.sh/k8s-device-plugin/pkg/monitor/nvidia/v1"
)

func TestLoadCacheV12Size(t *testing.T) {
	for _, tc := range []struct {
		name      string
		size      int
		minor     uint32
		wantError string
	}{
		{name: "valid", size: int(v1.SharedRegionSize) + 1, minor: 2},
		{name: "truncated", size: int(v1.SharedRegionSize) - 1, minor: 2, wantError: "too small for v1.2"},
		{name: "unknown minor", size: int(v1.SharedRegionSize) + 1, minor: 3, wantError: "unknown cache file size"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			data := make([]byte, tc.size)
			binary.LittleEndian.PutUint32(data[0:], SharedRegionMagicFlag)
			binary.LittleEndian.PutUint32(data[4:], 1)
			binary.LittleEndian.PutUint32(data[8:], tc.minor)
			if err := os.WriteFile(filepath.Join(dir, "usage.cache"), data, 0600); err != nil {
				t.Fatal(err)
			}
			usage, err := loadCache(dir)
			if tc.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantError) {
					t.Fatalf("loadCache error = %v, want %q", err, tc.wantError)
				}
				return
			}
			if err != nil || usage == nil || usage.Info == nil {
				t.Fatalf("loadCache = %v, %v", usage, err)
			}
			if err := syscall.Munmap(usage.data); err != nil {
				t.Fatal(err)
			}
		})
	}
}
