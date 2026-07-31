// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package test

import (
	"os"
	"path/filepath"

	"github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/gardener/machine-controller-manager/pkg/simulatedprovider/cluster"
	"github.com/gardener/machine-controller-manager/pkg/test/integration/common"
	"github.com/gardener/machine-controller-manager/pkg/test/integration/common/helpers"
	ginkgo "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

const (
	testClusterName = "test-cluster"
	testNamespace   = "test-ns"
)

var _ helpers.ResourcesTrackerInterface = &resourcesTrackerImpl{}

var (
	commons        = common.NewIntegrationTestFramework(&resourcesTrackerImpl{}, 600)
	testClusterEnv = cluster.New(testClusterName, testNamespace)
	_              = ginkgo.BeforeSuite(setup)
	_              = ginkgo.AfterSuite(cleanup)
	_              = ginkgo.Describe("Machine controllers test", func() {
		commons.BeforeEachCheck()
		commons.ControllerTests()
	})
)

func setup() {
	gomega.Expect(testClusterEnv.SetupCluster()).To(gomega.Succeed())
	gomega.Expect(setEnvVarsForFramework()).To(gomega.Succeed())
	commons.SetupBeforeSuite()
}

func cleanup() {
	commons.Cleanup()
	gomega.Expect(testClusterEnv.DeleteCluster()).To(gomega.Succeed())
}

func setEnvVarsForFramework() (err error) {
	vars := map[string]string{
		"KUBECONFIG":                testClusterEnv.Cfg.KubeconfigFile(),
		"CONTROL_KUBECONFIG":        testClusterEnv.Cfg.KubeconfigFile(),
		"CONTROL_NAMESPACE":         testClusterEnv.Namespace,
		"CONTROL_CLUSTER_NAMESPACE": testClusterEnv.Namespace,
		"IS_CONTROL_CLUSTER_SEED":   "true",
		"SIM_PROVIDER":              "true",
		"LEADER_ELECT":              "false",
		"MACHINECLASS_V1":           filepath.Join("test-mcc.yaml"),
	}

	for k, v := range vars {
		if err = os.Setenv(k, v); err != nil {
			return
		}
	}
	return
}

// ResourceTracker is not really useful for the simulated provider, since
// there's no real resources that require tracking and cleanup if left orphan.
// So this just satisfies the ResourceTrackerInterface in order to construct
// test integration testing framework.

type resourcesTrackerImpl struct {
	MachineClass *v1alpha1.MachineClass
	SecretData   map[string][]byte
	ClusterName  string
}

func (r *resourcesTrackerImpl) InitializeResourcesTracker(machineClass *v1alpha1.MachineClass, secretData map[string][]byte, clusterName string) error {
	r.MachineClass = machineClass
	r.SecretData = secretData
	r.ClusterName = clusterName
	return nil
}

func (r *resourcesTrackerImpl) IsOrphanedResourcesAvailable() bool {
	return false
}
