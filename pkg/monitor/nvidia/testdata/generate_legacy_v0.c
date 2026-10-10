// Reproduces the unversioned shared_region_t layout from HAMi-core 7a9c88b
// src/multiprocess/multiprocess_memory_limit.h on Linux amd64.
// Build: cc -std=c11 -o /tmp/gen-v0 generate_legacy_v0.c
// Run: /tmp/gen-v0 | gzip -n > legacy_v0.cache.gz
#include <stdint.h>
#include <semaphore.h>
#include <stdio.h>
#include <string.h>

typedef struct { uint64_t context_size, module_size, data_size, offset, total; } device_memory_t;
typedef struct { uint64_t dec_util, enc_util, sm_util; } device_util_t;
typedef struct {
    int32_t pid, hostpid;
    device_memory_t used[16];
    uint64_t monitorused[16];
    device_util_t device_util[16];
    int32_t status;
} shrreg_proc_slot_t;
typedef char uuid[96];
typedef struct {
    int32_t initialized_flag, sm_init_flag;
    size_t owner_pid;
    sem_t sem;
    uint64_t num;
    uuid uuids[16];
    uint64_t limit[16], sm_limit[16];
    shrreg_proc_slot_t procs[1024];
    int proc_num, utilization_switch, recent_kernel, priority;
} shared_region_t;
int main(void) {
    shared_region_t region;
    memset(&region, 0, sizeof(region));
    region.initialized_flag = 19920718;
    region.sm_init_flag = 1;
    region.num = 1;
    region.proc_num = 0;
    region.utilization_switch = 1;
    region.recent_kernel = 2;
    region.priority = 1;
    memcpy(region.uuids[0], "GPU-legacy-fixture", 18);
    region.limit[0] = 1073741824;
    if (fwrite(&region, 1, sizeof(region), stdout) != sizeof(region)) return 1;
    if (putchar(0) == EOF) return 1; // legacy cache writer appends one byte
    return 0;
}
