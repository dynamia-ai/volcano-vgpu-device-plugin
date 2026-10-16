# Optional scheduling and Allocate validation

Read [environment preparation](environment.md) first. Run these checks only when the requester wants functional evidence. The single-container Pod is a smoke fixture, not a GPU isolation or CUDA test.

Use the dedicated `volcano_run_dir`, `volcano_context`, and `volcano_skill_dir` values established during preparation. The fixture creates `volcano-smoke/vgpu-smoke`; choose a different name or namespace if that object already exists. Do not delete another test owner's Pod.

```bash
kubectl --kubeconfig "$volcano_run_dir/kubeconfig" --context "$volcano_context" \
  apply -f "$volcano_skill_dir/assets/single-container-pod.yaml"
kubectl --kubeconfig "$volcano_run_dir/kubeconfig" --context "$volcano_context" \
  wait -n volcano-smoke --for=condition=Ready pod/vgpu-smoke --timeout=150s
kubectl --kubeconfig "$volcano_run_dir/kubeconfig" --context "$volcano_context" \
  get pod vgpu-smoke -n volcano-smoke -o yaml
kubectl --kubeconfig "$volcano_run_dir/kubeconfig" --context "$volcano_context" \
  logs -n volcano-vgpu daemonset/volcano-device-plugin -c volcano-device-plugin --tail=100
kubectl --kubeconfig "$volcano_run_dir/kubeconfig" --context "$volcano_context" \
  exec -n volcano-smoke vgpu-smoke -- /bin/sh -c 'printenv | grep -E "CUDA_DEVICE_|NVIDIA_VISIBLE_DEVICES"'
```

Require evidence at each layer: `schedulerName: volcano`; binding to the mock worker; a Volcano allocation annotation with one GPU UUID, 4096 MiB, and 50% cores; a corresponding kubelet `Allocate Response` in the plugin log; the matching environment variables and mounts in the running container. Check that the monitor recognizes the container cache directory without repeated path errors. Preserve raw Pod JSON and relevant logs if reporting a pass or failure.

Do not infer hard isolation from `nvidia-smi -L` in this mock environment. A running Pod does not establish real CUDA execution, GPU memory limits, or SM throttling. The plugin's existing Pod-selection contract can fail when kubelet invokes Allocate separately for multiple containers; do not report multi-container support from this single-container fixture. Test capacity exhaustion/return, concurrent Pods, Xid handling, or plugin restarts only as separate, requested scenarios, with their own before/after evidence.

Leave the fixture running if the requester needs to inspect it. When cleanup is explicitly requested, identify the exact task-owned Pod, namespace, releases, and k3d cluster before removing anything; do not infer that another retained cluster is disposable.
