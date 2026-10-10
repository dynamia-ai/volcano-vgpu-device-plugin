package nvidia

import (
	"compress/gzip"
	"encoding/binary"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	v0 "volcano.sh/k8s-device-plugin/pkg/monitor/nvidia/v0"
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
		{name: "v1 truncated to v0 length", size: v0.CacheFileSize, minor: 2, wantError: "too small for v1.2"},
		{name: "unknown v1 truncated to v0 length", size: v0.CacheFileSize, minor: 3, wantError: "unknown cache file size"},
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

func TestLoadLegacyV0Fixture(t *testing.T) {
	f, err := os.Open("testdata/legacy_v0.cache.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	data, err := io.ReadAll(z)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != v0.CacheFileSize || !v0.ValidCache(data) {
		t.Fatalf("legacy C fixture failed layout validation: size=%d", len(data))
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "legacy.cache"), data, 0600); err != nil {
		t.Fatal(err)
	}
	usage, err := loadCache(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := usage.Info.DeviceNum(); got != 1 {
		t.Errorf("device count = %d, want 1", got)
	}
	if got := usage.Info.DeviceMemoryLimit(0); got != 1073741824 {
		t.Errorf("memory limit = %d", got)
	}
	if err := syscall.Munmap(usage.data); err != nil {
		t.Fatal(err)
	}
}
