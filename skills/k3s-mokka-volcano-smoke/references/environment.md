# Prepare the isolated environment

This procedure was exercised on Linux/amd64 with K3s `v1.32.5+k3s1`, Mokka Chart `0.4.0`, Volcano Chart `1.15.0`, and `volcano-vgpu-device-plugin:v1.12.0`. The [plugin compatibility table](https://github.com/Project-HAMi/volcano-vgpu-device-plugin#version-compatibility-matrix) lists plugin `v1.12.0` with Volcano `v1.15.0` or earlier. Recheck compatibility and adapt the manifests before changing versions. The Mokka image in the asset is the public, mock-only UBI8 build `ghcr.io/dynamia-ai/mokka-ubi8:0.4.0`, pinned to an observed amd64 digest; it is not the upstream NVIDIA image.

## Preconditions and ownership

Use Docker, k3d, kubectl, and Helm on a host where creating a task-owned cluster is authorized. Record `docker version`, `k3d version`, `kubectl version --client`, `helm version`, available disk/memory, and `k3d cluster list`. Choose a unique cluster name. Do not touch existing clusters, the default kubeconfig, production credentials, or a shared GPU node.

Set `volcano_skill_dir` to this skill's absolute directory and `volcano_run_dir` to a private, persistent, task-owned directory outside the repository. Keep the kubeconfig private; never include it in a public artifact. The example name is not a command to reuse if already present:

```bash
volcano_skill_dir=/absolute/path/to/repo/skills/k3s-mokka-volcano-smoke
volcano_run_dir=/absolute/path/to/task-owned-directory
volcano_cluster=volcano-mokka-unique-name
mkdir -p -m 700 "$volcano_run_dir"
k3d cluster list
k3d cluster create "$volcano_cluster" --image rancher/k3s:v1.32.5-k3s1 \
  --servers 1 --agents 1 \
  --k3s-node-label 'mokka.nvidia.com/type=sgpu@agent:0' \
  --k3s-node-label 'gpu=on@agent:0' \
  --kubeconfig-update-default=false --kubeconfig-switch-context=false
k3d kubeconfig get "$volcano_cluster" > "$volcano_run_dir/kubeconfig"
chmod 600 "$volcano_run_dir/kubeconfig"
volcano_context="k3d-$volcano_cluster"
kubectl --kubeconfig "$volcano_run_dir/kubeconfig" --context "$volcano_context" get nodes -o wide
```

Stop if the chosen name already exists, either node is not Ready, or the context points outside this task's cluster. Do not reconfigure the host's default context to work around a failure.

## Install Mokka

The bundled values expose two mock T4 devices on the labeled worker and enable Node Resource Interface (NRI) injection. NRI can affect other Pods in this isolated cluster. The values exclude `volcano-system` but **not** `volcano-vgpu`, because the device plugin needs the mock NVML libraries. Verify the pinned image is anonymously pullable before installation; if it is unavailable, stop and report the dependency rather than substituting an untested image.

```bash
helm pull oci://ghcr.io/nvidia/k8s-test-infra/chart/nvml-mock \
  --version 0.4.0 --destination "$volcano_run_dir"
helm lint "$volcano_run_dir/nvml-mock-0.4.0.tgz" \
  -f "$volcano_skill_dir/assets/mokka-values.yaml" --strict
helm template mokka "$volcano_run_dir/nvml-mock-0.4.0.tgz" \
  -n mokka -f "$volcano_skill_dir/assets/mokka-values.yaml"
helm install mokka "$volcano_run_dir/nvml-mock-0.4.0.tgz" \
  -n mokka --create-namespace -f "$volcano_skill_dir/assets/mokka-values.yaml" \
  --kubeconfig "$volcano_run_dir/kubeconfig" --kube-context "$volcano_context" \
  --wait --timeout 5m
```

Confirm the Mokka agent and NRI Pods are Ready on the labeled worker. Check NRI's connection to containerd. On that worker, `/var/lib/nvml-mock/driver/config/config.yaml` should contain `system.num_devices: 2` and the mock driver tree should contain `dev/nvidia0` and `dev/nvidia1`. These files are inside the k3d worker container, not necessarily at `/dev` on the host. Mokka management-Pod `nvidia-smi` output may show the profile's original device count; use the generated config and later plugin registration to judge the exposed count.

## Install Volcano and the plugin

The Volcano values keep the standard scheduler tiers and add `deviceshare` with `VGPUEnable`, `NodeLockEnable`, and `binpack`. Inspect the rendered ConfigMap before installing. The plugin manifest is a task-specific adaptation of the plugin's v1.12.0 static manifest: the device config ConfigMap must be in `volcano-system` because that release looks there, while the DaemonSet runs in `volcano-vgpu` for Mokka injection. The DaemonSet is limited to the labeled worker, mounts the mock driver tree, and gives its monitor the actual host cache path. Do not apply it to an existing GPU cluster.

```bash
helm pull volcano --repo https://volcano-sh.github.io/helm-charts \
  --version 1.15.0 --destination "$volcano_run_dir"
helm lint "$volcano_run_dir/volcano-1.15.0.tgz" \
  -f "$volcano_skill_dir/assets/volcano-values.yaml" --strict
helm template volcano "$volcano_run_dir/volcano-1.15.0.tgz" \
  -n volcano-system -f "$volcano_skill_dir/assets/volcano-values.yaml"
helm install volcano "$volcano_run_dir/volcano-1.15.0.tgz" \
  -n volcano-system --create-namespace \
  -f "$volcano_skill_dir/assets/volcano-values.yaml" \
  --kubeconfig "$volcano_run_dir/kubeconfig" --kube-context "$volcano_context" \
  --wait --timeout 5m
kubectl --kubeconfig "$volcano_run_dir/kubeconfig" --context "$volcano_context" \
  apply --dry-run=client -f "$volcano_skill_dir/assets/plugin-manifest.yaml"
kubectl --kubeconfig "$volcano_run_dir/kubeconfig" --context "$volcano_context" \
  apply -f "$volcano_skill_dir/assets/plugin-manifest.yaml"
kubectl --kubeconfig "$volcano_run_dir/kubeconfig" --context "$volcano_context" \
  rollout status daemonset/volcano-device-plugin -n volcano-vgpu --timeout=180s
```

Before handoff, verify Volcano admission/controller/scheduler and both plugin containers are Ready. Read the live `volcano-scheduler-configmap` to confirm `deviceshare.VGPUEnable: true` and `deviceshare.NodeLockEnable: true`. The labeled worker should register two distinct mock GPU UUIDs and `volcano.sh/vgpu-number=20`, `volcano.sh/vgpu-memory=32768`, `volcano.sh/vgpu-cores=200`; the server should not register these GPU resources. Record the actual image IDs and any discrepancies. A successful DaemonSet rollout alone is not enough.

If kubelet rejects `volcano.sh/` as an invalid resource name, check that `volcano-vgpu-device-config` is in `volcano-system` and restart only this task's plugin DaemonSet after correcting it. If monitor repeatedly reports `open /tmp/vgpu/containers: no such file or directory`, check its cache mount and the plugin's host cache directory. If NVML or NRI reports an ABI error, keep the failing logs and stop; do not silently substitute another image or edit the host runtime.

Leave the cluster and private kubeconfig in place for the requester. Cleanup is a separate, explicitly authorized action after confirming no one still uses this cluster.
