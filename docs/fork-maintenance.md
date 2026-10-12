# Maintaining the Dynamia AI fork

Upstream: https://github.com/Project-HAMi/volcano-vgpu-device-plugin

Organization repository: https://github.com/dynamia-ai/volcano-vgpu-device-plugin

## Branches and review

- `main` is the team integration branch. Changes require a pull request, one approval, resolved review conversations, and a passing `targeted-race` check.
- `upstream/main` records the upstream commit used for synchronization. Update it only from the upstream project, then bring updates into `main` through a reviewed pull request.
- `dynamia/hami-alignment` contains the HAMi alignment changes. The original reviewed implementation is `c0faed1`; this branch also incorporates upstream dependency updates from `5283c86` and fork maintenance configuration.
- `research/multi-container-identity` preserves diagnostic tests and investigation. It is not a production fix or a release branch.

Repository access follows the organization's existing member permissions. Use feature branches for daily work and retain upstream provenance and license notices. Send suitable, independently reviewable fixes upstream separately.

## Validation

`Fork CI` runs the targeted Go race tests on pull requests and changes to `main`. It uses read-only repository permissions and does not publish images or charts. The inherited image and chart workflows are restricted to the upstream repository; this fork needs a separate release setup before publishing artifacts.

Local equivalent:

```sh
git submodule update --init --recursive
go test -race ./pkg/plugin ./pkg/util ./pkg/config ./pkg/monitor/nvidia/... ./pkg/vgpucache ./pkg/nodepodinformer ./cmd/vgpu ./cmd/vgpu-monitor
helm lint deployments/helm/volcano-vgpu-device-plugin
```

The full `go test ./...` suite had three replica assertion failures in `api/config/v1` and a vet formatting warning in `pkg/rm/device_map.go` on the original `6063efe` baseline. The targeted check does not mean the full suite passes.

## Current behavior and limits

The alignment updates the HAMi-core pin and cache ABI handling, allocation batch validation, cache and monitor lifetimes, existing MIG/CDI error handling, preferred allocation behavior, and node label configuration.

K3s tests with Mokka and Fake GPU Operator covered the real kubelet control path. With Volcano's existing `deviceshare.NodeLockEnable: true` confirmed active, both the baseline and aligned plugin ran eight concurrent single-container Pods successfully. A Pod with two GPU containers still failed: its first Allocate requested one device while the Pod annotation contained two. This is a pre-existing limitation, not fixed by batch handling. A configuration-transition run without observed locking also failed and remains unexplained.

The mock results do not validate CUDA execution, hardware isolation, or real MIG behavior. The multi-container research branch documents an attempted matching heuristic and a model counterexample; it does not prove that the modeled delayed request timing occurred on a real kubelet.
