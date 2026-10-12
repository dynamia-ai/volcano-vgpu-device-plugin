# Multi-container allocation identity investigation

This research branch contains diagnostic tests, not a production fix. It builds on the HAMi alignment branch and leaves the production Pod matching logic unchanged.

## Observed failure

On K3s with Fake GPU Operator and unmodified Volcano v1.15.1, both the original plugin (`6063efe`) and the aligned implementation (`c0faed1`) rejected a Pod with two GPU containers. The first kubelet Allocate requested one device; the Pod's remaining allocation annotation contained two. Existing GetPendingPod matching compares those totals and fails before either container starts.

Enabling Volcano's existing NodeLockEnable option did not resolve this mismatch. In separate runs with actual lock set/release events, eight concurrent single-container Pods ran successfully on each version. This does not establish correctness for every concurrent or failure timing.

## HAMi history

The inspected HAMi history contains several distinct fixes:

- `20687544`: multi-container device use and device-specific completion.
- `74428e67`: namespace/name ownership in the node lock.
- `0658b5c8`: allocation involving init containers.
- `3616313c`: decode annotations once and consume containers in memory before one patch.
- `52299d44`: avoid transferring an expired lock while its original Pod remains Pending.
- `9308e3f8`: remove the fallback that guesses a Pod from annotations when no owned lock is available.

These changes cannot be reduced to copying a loop into the Volcano plugin. The inspected reference is Project-HAMi/HAMi `3307b6cea526e9b63f5db28ff8dfa2f7b02a1ad1`.

## Diagnostic attempt

Device Plugin v1beta1 Allocate carries device IDs, not a Pod UID or container name. The test-only candidate matcher requires a unique bound, Pending, allocating Pod whose next container matches the requested count.

The tests establish two narrower facts:

1. Given the correct Pod, the existing planning helper can consume two container allocations in separate calls.
2. The candidate heuristic cannot distinguish a modeled delayed request for old Pod A from a current request for Pod B when their device IDs and current observable candidate state are identical.

The second result is a model counterexample. The exact delay/deletion/device-ID-reuse timing was not injected into a real kubelet. It does not prove that every stateful or externally coordinated solution is impossible. It does show that unique current candidacy alone is not sufficient evidence of request ownership.

Run the diagnostics with:

```sh
go test -race ./pkg/util -run TestIdentityRepro -v -count=1
```

Passing means the diagnostic behavior was reproduced, not that multi-container allocation is fixed.

## Next design work

A production proposal needs a verifiable association between requests and allocation records. Investigate owner-bearing locks together with their lifecycle guarantees, or an allocation generation carried by a usable request/device-ID channel. An owner field alone does not automatically authenticate a delayed request.

Validate real kubelet sequential allocation, delayed requests, Pod replacement, lock transfer, ID reuse, idempotent retries, plugin/kubelet restarts, and mixed component versions. The current alignment deliberately does not change Volcano's annotation or node ownership protocol; no such protocol extension is implemented here.
