---
name: k3s-mokka-volcano-smoke
description: Prepare an isolated CPU-only K3s cluster with Mokka, the unmodified Volcano scheduler, and volcano-vgpu-device-plugin for vGPU scheduling and Allocate tests. Use for mock GPU integration work; not for real GPU, CUDA, or isolation certification.
---

# K3s + Mokka + Volcano vGPU

Prepare a reusable test environment without requiring an NVIDIA GPU. The default outcome is a running cluster, dedicated kubeconfig, healthy components, and registered mock vGPU capacity. Leave the cluster running for the requester; do not create test Pods or destroy the cluster unless requested.

1. Read [environment preparation](references/environment.md) before creating resources. Use the bundled values and manifest only in a new, task-owned cluster. The pinned version combination is an observed baseline, not a compatibility claim for other releases.
2. If the requester asks for scheduling, Allocate, capacity, or regression evidence, read [optional validation](references/validation.md) and run only the requested checks.
3. Report the exact versions, image IDs, cluster name, kubeconfig path, observed registration, and any failing layer. Keep mock NVML evidence separate from real GPU/CUDA claims.

The assets are `assets/mokka-values.yaml`, `assets/volcano-values.yaml`, `assets/plugin-manifest.yaml`, and `assets/single-container-pod.yaml`. They contain no Enterprise application configuration or license bypass. Do not deploy HAMi or another NVIDIA device plugin alongside this test unless the requester explicitly expands the scope.
