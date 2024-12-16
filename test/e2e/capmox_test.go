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
See the License for the specific language governing permissions and limitations under the License.
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

	Context("[RBAC] Validate Proxmox RBAC Permissions", func() {
		It("Should validate that Proxmox RBAC roles and permissions are correctly applied", func() {
			// Define a list of roles and their expected paths/permissions
			rbacTests := []struct {
				role        string
				path        string
				expected    string
				propagate   bool
			}{
				{"Sys.Audit", "/nodes", "read", false},
				{"PVEVMAdmin", "/pool/capi", "write", false},
				{"PVETemplateUser", "/pool/templates", "read", false},
				{"PVESDNUser", "/sdn/zones/localnetwork/vmbr0/1234", "read", false},
				{"PVEDataStoreAdmin", "/storage/capi_files", "write", false},
				{"Datastore.AllocateSpace", "/storage/shared_block", "write", false},
			}

			// Iterate over each role and verify permissions
			for _, test := range rbacTests {
				By(fmt.Sprintf("Verifying RBAC permission for role: %s, path: %s", test.role, test.path))
				verifyRBACPermission(test.role, test.path, test.expected, test.propagate)
			}
		})
	})
	
	// Function to verify RBAC permissions for Proxmox
	func verifyRBACPermission(role, path, expected string, propagate bool) {
		// Create a Proxmox API client here
		proxmoxClient := getProxmoxClient()

		// Fetch permissions for the role and path
		permissions, err := proxmoxClient.GetPermissions(role, path)
		Expect(err).NotTo(HaveOccurred(), "Failed to fetch permissions for role %s and path %s", role, path)

		// Check if the permission matches the expected value
		for _, permission := range permissions {
			if permission.Path == path {
				Expect(permission.Role).To(Equal(role), "Expected role %s, but got %s", role, permission.Role)
				Expect(permission.Permission).To(Equal(expected), "Expected permission %s, but got %s", expected, permission.Permission)
				Expect(permission.Propagate).To(Equal(propagate), "Expected propagate %v, but got %v", propagate, permission.Propagate)
			}
		}
	}
})


