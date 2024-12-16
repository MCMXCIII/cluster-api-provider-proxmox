//go:build e2e
// +build e2e

/*
Copyright 2024 IONOS Cloud.

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

package e2e

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	"sigs.k8s.io/cluster-api/util"
)

var _ = Describe("Workload cluster creation", func() {
	var (
		ctx                 = context.TODO()
		specName            = "create-workload-cluster"
		namespace           *corev1.Namespace
		cancelWatches       context.CancelFunc
		result              *clusterctl.ApplyClusterTemplateAndWaitResult
		clusterName         string
		clusterctlLogFolder string
	)

	BeforeEach(func() {
		// Validate environment setup and create necessary directories
		Expect(e2eConfig).ToNot(BeNil(), "Invalid argument. e2eConfig can't be nil when calling %s spec", specName)
		Expect(clusterctlConfigPath).To(BeAnExistingFile(), "Invalid argument. clusterctlConfigPath must be an existing file when calling %s spec", specName)
		Expect(bootstrapClusterProxy).ToNot(BeNil(), "Invalid argument. bootstrapClusterProxy can't be nil when calling %s spec", specName)
		Expect(os.MkdirAll(artifactFolder, 0755)).To(Succeed(), "Invalid argument. artifactFolder can't be created for %s spec", specName)

		Expect(e2eConfig.Variables).To(HaveKey(KubernetesVersion))

		clusterName = fmt.Sprintf("capmox-e2e-%s", util.RandomString(6))

		// Setup a Namespace where to host objects for this spec and create a watcher for the namespace events.
		namespace, cancelWatches = setupSpecNamespace(ctx, specName, bootstrapClusterProxy, artifactFolder)

		result = new(clusterctl.ApplyClusterTemplateAndWaitResult)

		// We need to override clusterctl apply log folder to avoid getting our credentials exposed.
		clusterctlLogFolder = filepath.Join(os.TempDir(), "clusters", bootstrapClusterProxy.GetName())
	})

	AfterEach(func() {
		cleanInput := cleanupInput{
			SpecName:        specName,
			Cluster:         result.Cluster,
			ClusterProxy:    bootstrapClusterProxy,
			Namespace:       namespace,
			CancelWatches:   cancelWatches,
			IntervalsGetter: e2eConfig.GetIntervals,
			SkipCleanup:     skipCleanup,
			ArtifactFolder:  artifactFolder,
		}

		dumpSpecResourcesAndCleanup(ctx, cleanInput)
	})

	// Helper function to apply cluster templates
	applyClusterTemplate := func(clusterName, flavor string, controlPlaneCount, workerCount int64) *clusterctl.ApplyClusterTemplateAndWaitResult {
		return clusterctl.ApplyClusterTemplateAndWait(ctx, clusterctl.ApplyClusterTemplateAndWaitInput{
			ClusterProxy: bootstrapClusterProxy,
			ConfigCluster: clusterctl.ConfigClusterInput{
				LogFolder:                clusterctlLogFolder,
				ClusterctlConfigPath:     clusterctlConfigPath,
				KubeconfigPath:           bootstrapClusterProxy.GetKubeconfigPath(),
				InfrastructureProvider:   clusterctl.DefaultInfrastructureProvider,
				Flavor:                   flavor,
				Namespace:                namespace.Name,
				ClusterName:              clusterName,
				KubernetesVersion:        e2eConfig.GetVariable(KubernetesVersion),
				ControlPlaneMachineCount: pointer.Int64Ptr(controlPlaneCount),
				WorkerMachineCount:       pointer.Int64Ptr(workerCount),
			},
			WaitForClusterIntervals:      e2eConfig.GetIntervals(specName, "wait-cluster"),
			WaitForControlPlaneIntervals: e2eConfig.GetIntervals(specName, "wait-control-plane"),
			WaitForMachineDeployments:    e2eConfig.GetIntervals(specName, "wait-worker-nodes"),
		}, result)
	}

	// Tests for a generic workload cluster with a single control-plane node and worker scaling
	Context("[Generic] Creating a single control-plane cluster", func() {
		It("Should create a cluster with 1 worker node and can be scaled", func() {
			By("Initializes with 1 worker node")
			result := applyClusterTemplate(clusterName, clusterctl.DefaultFlavor, 1, 1)

			By("Scaling worker node to 3")
			result = applyClusterTemplate(clusterName, clusterctl.DefaultFlavor, 1, 3)
		})
	})

	// Tests for a highly available cluster setup with 3 control-plane nodes and 2 worker nodes
	Context("[Generic] Creating a highly available control-plane cluster", func() {
		It("Should create a cluster with 3 control-plane and 2 worker nodes", func() {
			By("Creating a highly available cluster")
			result := applyClusterTemplate(clusterName, clusterctl.DefaultFlavor, 3, 2)
		})
	})

	// Tests for a highly available cluster setup with Flatcar OS
	Context("[Flatcar] Creating a highly available control-plane cluster with Flatcar", func() {
		It("Should create a HA cluster with Flatcar", func() {
			By("Creating a flatcar high available cluster")
			result := applyClusterTemplate(clusterName, "flatcar", 3, 2)
		})
	})

	// Tests for creating a single control-plane cluster with multiple configurations
	Context("[Advanced] Creating a workload cluster with multiple configurations", func() {
		It("Should create a workload cluster with different configurations", func() {
			By("Creating a cluster with control-plane count of 2 and worker count of 3")
			result := applyClusterTemplate(clusterName, clusterctl.DefaultFlavor, 2, 3)

			By("Creating a cluster with control-plane count of 1 and worker count of 5")
			result = applyClusterTemplate(clusterName, "flatcar", 1, 5)
		})
	})

})

