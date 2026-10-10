/*
Copyright 2026 The Volcano Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package plugin

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"

	spec "volcano.sh/k8s-device-plugin/api/config/v1"
	"volcano.sh/k8s-device-plugin/pkg/rm"
	"volcano.sh/k8s-device-plugin/pkg/util"
)

func TestPreferredAllocationIsNotAdvertised(t *testing.T) {
	plugin := &nvidiaDevicePlugin{}
	options, err := plugin.GetDevicePluginOptions(context.Background(), &pluginapi.Empty{})
	require.NoError(t, err)
	require.False(t, options.GetPreferredAllocationAvailable)
	response, err := plugin.GetPreferredAllocation(context.Background(), &pluginapi.PreferredAllocationRequest{})
	require.Nil(t, response)
	require.Equal(t, codes.Unimplemented, status.Code(err))
}

func TestAllocateVolcanoPreservesPhysicalGPUAndLimitOrder(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{UID: "pod-uid", Annotations: map[string]string{
			util.AssignedIDsToAllocateAnnotations: "GPU-B,NVIDIA,2000,30:GPU-A,NVIDIA,1000,30:",
		}},
		Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "worker"}}},
	}
	plugin, ops := newVolcanoTestPlugin(t, pod)
	response, err := plugin.Allocate(context.Background(), &pluginapi.AllocateRequest{ContainerRequests: []*pluginapi.ContainerAllocateRequest{
		{DevicesIds: []string{"replica-A", "replica-B"}},
	}})
	require.NoError(t, err)
	require.Len(t, response.ContainerResponses, 1)
	envs := response.ContainerResponses[0].Envs
	require.Equal(t, "GPU-B,GPU-A", envs["NVIDIA_VISIBLE_DEVICES"])
	require.Equal(t, "2000m", envs["CUDA_DEVICE_MEMORY_LIMIT_0"])
	require.Equal(t, "1000m", envs["CUDA_DEVICE_MEMORY_LIMIT_1"])
	require.Equal(t, "30", envs["CUDA_DEVICE_SM_LIMIT"])
	require.Equal(t, "", ops.patched[util.AssignedIDsToAllocateAnnotations])
}

func TestAllocateVolcanoRejectsUnpairableLimits(t *testing.T) {
	for _, tc := range []struct {
		name       string
		annotation string
		want       string
	}{
		{"duplicate GPU", "GPU-A,NVIDIA,1000,30:GPU-A,NVIDIA,2000,30:", "more than once"},
		{"different core limits", "GPU-A,NVIDIA,1000,30:GPU-B,NVIDIA,2000,20:", "different GPU core limits"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{util.AssignedIDsToAllocateAnnotations: tc.annotation}},
				Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "worker"}}},
			}
			plugin, ops := newVolcanoTestPlugin(t, pod)
			response, err := plugin.Allocate(context.Background(), &pluginapi.AllocateRequest{ContainerRequests: []*pluginapi.ContainerAllocateRequest{
				{DevicesIds: []string{"replica-A", "replica-B"}},
			}})
			require.Nil(t, response)
			require.ErrorContains(t, err, tc.want)
			require.Nil(t, ops.patched)
			require.Equal(t, 1, ops.failed)
		})
	}
}

func TestIndexStrategyUsesAssignedGPUOrder(t *testing.T) {
	plugin := &nvidiaDevicePlugin{
		config: &spec.Config{Flags: spec.Flags{CommandLineFlags: spec.CommandLineFlags{
			Plugin: &spec.PluginCommandLineFlags{DeviceIDStrategy: ptr(spec.DeviceIDStrategyIndex)},
		}}},
		rm: &rm.ResourceManagerMock{DevicesFunc: func() rm.Devices {
			return rm.Devices{
				"GPU-A::0": {Device: pluginapi.Device{ID: "GPU-A::0"}, Index: "0"},
				"GPU-B::0": {Device: pluginapi.Device{ID: "GPU-B::0"}, Index: "1"},
			}
		}},
	}
	for i := 0; i < 20; i++ {
		indices, err := plugin.uniqueDeviceIDsFromAnnotatedDeviceIDs([]string{"GPU-B", "GPU-A"})
		require.NoError(t, err)
		require.Equal(t, []string{"1", "0"}, indices)
	}
	_, err := plugin.uniqueDeviceIDsFromAnnotatedDeviceIDs([]string{"GPU-C"})
	require.ErrorContains(t, err, "not registered")
}
