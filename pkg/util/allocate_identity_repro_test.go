package util

import (
	"reflect"
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
)

// These are diagnostic reproductions, not evidence of a production fix.
func identityReproPod(name, allocation string) v1.Pod {
	return v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test", Name: name, UID: types.UID(name + "-uid"),
			Annotations: map[string]string{
				AssignedNodeAnnotations: "node-a", AssignedTimeAnnotations: "100",
				AssignedIDsToAllocateAnnotations: allocation,
				DeviceBindPhase:                  DeviceBindAllocating,
			},
		},
		Spec:   v1.PodSpec{NodeName: "node-a", Containers: []v1.Container{{Name: "first"}, {Name: "second"}}},
		Status: v1.PodStatus{Phase: v1.PodPending},
	}
}

func TestIdentityReproKnownPodSupportsSequentialPlanning(t *testing.T) {
	pod := identityReproPod("a", "GPU-A,NVIDIA,1000,20:;GPU-B,NVIDIA,2000,30:")
	first, remaining, err := PlanDeviceRequests(NvidiaGPUDevice, pod, []int{1})
	if err != nil {
		t.Fatal(err)
	}
	pod.Annotations[AssignedIDsToAllocateAnnotations] = remaining
	second, remaining, err := PlanDeviceRequests(NvidiaGPUDevice, pod, []int{1})
	if err != nil {
		t.Fatal(err)
	}
	if first[0].Container.Name != "first" || first[0].Devices[0].UUID != "GPU-A" ||
		second[0].Container.Name != "second" || second[0].Devices[0].UUID != "GPU-B" {
		t.Fatalf("unexpected sequential plans: first=%+v second=%+v", first, second)
	}
	_, count := DecodePodDevices(remaining)
	if count != 0 {
		t.Fatalf("devices remain after both containers: %q", remaining)
	}
	t.Log("Once Pod identity is supplied, existing planning handles separate container calls; this does not identify the RPC owner.")
}

// A plausible plugin-only attempt: require a bound, allocating Pod and match
// the next container, rejecting multiple candidates. Kept TEST-ONLY because
// uniqueness does not establish which Pod caused the current RPC.
func identityReproUniqueCandidate(pods []v1.Pod, req *pluginapi.AllocateRequest) *v1.Pod {
	sizes := make([]int, len(req.ContainerRequests))
	for i, request := range req.ContainerRequests {
		sizes[i] = len(request.DevicesIds)
	}
	var selected *v1.Pod
	for i := range pods {
		pod := &pods[i]
		if pod.Spec.NodeName != "node-a" || pod.Status.Phase != v1.PodPending ||
			pod.Annotations[AssignedNodeAnnotations] != "node-a" ||
			pod.Annotations[DeviceBindPhase] != DeviceBindAllocating {
			continue
		}
		if _, _, err := PlanDeviceRequests(NvidiaGPUDevice, *pod, sizes); err != nil {
			continue
		}
		if selected != nil {
			return nil
		}
		selected = pod
	}
	return selected
}

func TestIdentityReproUniqueCandidateCannotDistinguishStaleRPC(t *testing.T) {
	// A was deleted and the kubelet may reuse this advertised virtual device ID
	// for B. A delayed/replayed request and B's first request have identical
	// protobuf data. The current Pod list and even an owner-bearing node lock
	// would also be identical (only B remains) in the two cases.
	staleA := &pluginapi.AllocateRequest{ContainerRequests: []*pluginapi.ContainerAllocateRequest{{DevicesIds: []string{"GPU-A-0"}}}}
	freshB := &pluginapi.AllocateRequest{ContainerRequests: []*pluginapi.ContainerAllocateRequest{{DevicesIds: []string{"GPU-A-0"}}}}
	if !reflect.DeepEqual(staleA, freshB) {
		t.Fatal("counterexample must present identical RPC inputs")
	}
	b := identityReproPod("b", "GPU-A,NVIDIA,7000,40:;GPU-A,NVIDIA,9000,50:")
	pods := []v1.Pod{b}
	for _, request := range []*pluginapi.AllocateRequest{staleA, freshB} {
		got := identityReproUniqueCandidate(pods, request)
		if got == nil || got.UID != b.UID {
			t.Fatal("test-only heuristic no longer reproduces the unique-candidate selection")
		}
	}
	t.Log("REPRODUCED: heuristic selects B for both indistinguishable calls; stale A would consume B's annotation. This candidate is not safe to ship.")

	a := identityReproPod("a", "GPU-A,NVIDIA,1000,20:;GPU-A,NVIDIA,2000,30:")
	if got := identityReproUniqueCandidate([]v1.Pod{a, b}, freshB); got != nil {
		t.Fatal("multiple candidates should fail closed")
	}
	// Rejecting every input avoids misallocation but also rejects B's legitimate
	// first request. An external association is needed to distinguish them.
}
