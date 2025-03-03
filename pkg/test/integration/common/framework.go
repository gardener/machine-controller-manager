// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gardener/machine-controller-manager/pkg/controller"

	"github.com/onsi/ginkgo/v2"

	"github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	appsV1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/util/retry"

	"github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/gardener/machine-controller-manager/pkg/test/integration/common/helpers"
	"github.com/gardener/machine-controller-manager/pkg/test/utils/matchers"
)

const (
	dwdIgnoreScalingAnnotation = "dependency-watchdog.gardener.cloud/ignore-scaling"
	testMachineDeploymentName  = "test-machine-deployment"
	testMachineName            = "test-machine"
)

var (
	// path for storing log files (mcm & mc processes)
	targetDir = filepath.Join("..", "..", "..", ".ci", "controllers-test", "logs")
	// Suffix for the`kubernetes-io-cluster` tag and cluster name for the orphan resource tracker. Used as ResourceGroupName for Azure clusters
	targetClusterName = os.Getenv("TARGET_CLUSTER_NAME")
	// machine-controller-manager log file
	mcmLogFile = filepath.Join(targetDir, "mcm_process.log")

	// machine-controller log file
	mcLogFile = filepath.Join(targetDir, "mc_process.log")

	// relative path to clone machine-controller-manager repo.
	// used when control cluster is shoot cluster
	mcmRepoPath = filepath.Join("..", "..", "..", "dev", "mcm")

	// control cluster namespace to create resources.
	// ignored if the target cluster is a shoot of the control cluster
	controlClusterNamespace = os.Getenv("CONTROL_CLUSTER_NAMESPACE")

	// mcContainerPrefix is the prefix used for the container name of
	// machine controller in the MCM deployment
	// eg: machine-controller-manager-provider-<provider-name>
	mcContainerPrefix = "machine-controller-manager-provider-"

	// make processes/sessions started by gexec. available only if the controllers are running in local setup. updated during runtime
	mcmsession, mcsession *gexec.Session

	// mcmDeploymentOrigObj a placeholder for mcm deployment object running in seed cluster.
	// it will be scaled down to 0 before test starts.
	// also used in cleanup to restore the controllers to its original state.
	// used only if control cluster is seed
	mcmDeploymentOrigObj *appsV1.Deployment

	// machineControllerManagerDeploymentName specifies the name of the deployment
	// running the mc and mcm containers in it.
	machineControllerManagerDeploymentName = os.Getenv("MACHINE_CONTROLLER_MANAGER_DEPLOYMENT_NAME")

	// names of machineclass resource.
	testMachineClassResources = []string{"test-mc-v1", "test-mc-v2"}

	// path for v1machineclass yaml file to be used while creating machine resources
	// name of the machineclass will always be test-mc-v1. overriding the name of machineclass in yaml file
	// ignored if control cluster is seed cluster
	v1MachineClassPath = os.Getenv("MACHINECLASS_V1")

	// path for v1machineclass yaml file to be used while upgrading machine deployment
	// if machineClassV2 is not set then v1MachineClassPath will be used intead for creating test-mc-v2 class
	// ignored if control cluster if seed cluster
	v2MachineClassPath = os.Getenv("MACHINECLASS_V2")

	// if true, means that the tags present on VM are strings not key-value pairs
	isTagsStrings = os.Getenv("TAGS_ARE_STRINGS")

	// if true, control cluster is a seed
	// only set this variable if operating in gardener context
	isControlSeed = os.Getenv("IS_CONTROL_CLUSTER_SEED")

	//values for gardener-node-agent-secret-name
	gnaSecretNameLabelValue = os.Getenv("GNA_SECRET_NAME")

	// Specifies whether the CRDs should be preserved during cleanup at the end
	preserveCRDuringCleanup = os.Getenv("PRESERVE_CRD_AT_END")

	// Are the tests running for the virtual provider
	isVirtualProvider = os.Getenv("IS_VIRTUAL_PROVIDER")
)

// ProviderSpecPatch struct holds tags for provider, which we want to patch the  machineclass with
type ProviderSpecPatch struct {
	Tags []string `json:"tags"`
}

// MachineClassPatch struct holds values of patch for machine class for provider GCP
type MachineClassPatch struct {
	ProviderSpec ProviderSpecPatch `json:"providerSpec"`
}

// IntegrationTestFramework struct holds the values needed by the controller tests
type IntegrationTestFramework struct {
	// Must be rti implementation for the hyperscaler provider.
	// It is used for checking orphan resources.
	resourcesTracker helpers.ResourcesTrackerInterface

	// Control cluster resource containing ClientSets for accessing kubernetes resources
	// And kubeconfig file path for the cluster
	// initialization is done by SetupBeforeSuite
	ControlCluster *helpers.Cluster

	// Target cluster resource containing ClientSets for accessing kubernetes resources
	// And kubeconfig file path for the cluster
	// initialization is done by SetupBeforeSuite
	TargetCluster *helpers.Cluster

	// timeout for Eventually to probe kubernetes cluster resources
	// for machine creation, deletion, machinedeployment update
	// can be different for different cloud providers
	timeout time.Duration

	// pollingInterval for Eventually to probe kubernetes cluster resources
	// for machine creation, deletion, machinedeployment update
	// can be different for different cloud providers
	pollingInterval time.Duration
}

// NewIntegrationTestFramework creates a new IntegrationTestFramework
// initializing resource tracker implementation.
// Optially the timeout and polling interval are configurable as optional arguments
// The default values used for Eventually to probe kubernetes cluster resources is
// 300 seconds for timeout and  2 seconds for polling interval
// for machine creation, deletion, machinedeployment update e.t.c.,
// The first optional argument is the timeoutSeconds
// The second optional argument is the pollingIntervalSeconds
func NewIntegrationTestFramework(
	resourcesTracker helpers.ResourcesTrackerInterface,
	intervals ...int64,
) *IntegrationTestFramework {

	timeout := 300 * time.Second
	if len(intervals) > 0 {
		timeout = time.Duration(intervals[0]) * time.Second
	}

	pollingInterval := 2 * time.Second
	if len(intervals) > 1 {
		pollingInterval = time.Duration(intervals[1]) * time.Second
	}

	return &IntegrationTestFramework{
		resourcesTracker: resourcesTracker,
		timeout:          timeout,
		pollingInterval:  pollingInterval,
	}
}

func (c *IntegrationTestFramework) initalizeClusters() error {
	// checks for the validity of controlKubeConfig and targetKubeConfig clusters
	// and intializes clientsets
	controlKubeConfigPath := os.Getenv("CONTROL_KUBECONFIG")
	targetKubeConfigPath := os.Getenv("TARGET_KUBECONFIG")

	if len(controlKubeConfigPath) != 0 {
		controlKubeConfigPath, _ = filepath.Abs(controlKubeConfigPath)

		// if control cluster config is available but not the target, then set control and target clusters as same
		if len(targetKubeConfigPath) == 0 {
			targetKubeConfigPath = controlKubeConfigPath
			log.Println("Missing targetKubeConfig. control cluster will be set as target too")
		}

		targetKubeConfigPath, _ = filepath.Abs(targetKubeConfigPath)
		log.Printf("Control cluster kube-config - %s\n", controlKubeConfigPath)
		log.Printf("Target cluster kube-config  - %s\n", targetKubeConfigPath)

		// intialize clusters
		var err error
		c.ControlCluster, err = helpers.NewCluster(controlKubeConfigPath)
		if err != nil {
			return err
		}
		c.TargetCluster, err = helpers.NewCluster(targetKubeConfigPath)
		if err != nil {
			return err
		}

		// update clientset and check whether the cluster is accessible
		err = c.ControlCluster.FillClientSets()
		if err != nil {
			log.Println("Failed to check nodes in the cluster")
			return err
		}
		err = c.TargetCluster.FillClientSets()
		if err != nil {
			log.Println("Failed to check nodes in the cluster")
			return err
		}
	}
	// set default control cluster namespace if not specified
	if len(controlClusterNamespace) == 0 {
		controlClusterNamespace = "default"
	}
	// Verify the control cluster namespace
	err := c.ControlCluster.VerifyControlClusterNamespace(isControlSeed, controlClusterNamespace)
	if err != nil {
		return err
	}

	// setting env variable for later use
	err = os.Setenv("CONTROL_CLUSTER_NAMESPACE", controlClusterNamespace)
	if err != nil {
		return err
	}
	return nil
}

func (c *IntegrationTestFramework) scaleMcmDeployment(replicas int32) error {

	ctx := context.Background()

	retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		// Retrieve the latest version of Deployment before attempting update
		// RetryOnConflict uses exponential backoff to avoid exhausting the apiserver
		result, getErr := c.ControlCluster.Clientset.
			AppsV1().
			Deployments(controlClusterNamespace).
			Get(
				ctx,
				machineControllerManagerDeploymentName,
				metav1.GetOptions{},
			)
		if getErr != nil {
			return getErr
		}

		if replicas == 0 {
			result.ObjectMeta.Annotations[dwdIgnoreScalingAnnotation] = "true"
		} else if replicas == 1 {
			delete(result.ObjectMeta.Annotations, dwdIgnoreScalingAnnotation)
		}
		// if number of replicas is not equal to the required replicas then update
		if *result.Spec.Replicas != replicas {
			*result.Spec.Replicas = replicas
			_, updateErr := c.ControlCluster.Clientset.
				AppsV1().
				Deployments(controlClusterNamespace).
				Update(
					ctx,
					result,
					metav1.UpdateOptions{},
				)
			return updateErr
		}
		return nil
	})
	return retryErr
}

func (c *IntegrationTestFramework) updatePatchFile() {
	clusterTag := "kubernetes-io-cluster-" + targetClusterName
	testRoleTag := "kubernetes-io-role-integration-test"

	patchMachineClassData := MachineClassPatch{
		ProviderSpec: ProviderSpecPatch{
			Tags: []string{
				targetClusterName,
				clusterTag,
				testRoleTag,
			},
		},
	}

	data, _ := json.MarshalIndent(patchMachineClassData, "", " ")

	//writing to machine-class-patch.json
	gomega.Expect(os.WriteFile(filepath.Join("..", "..", "..",
		".ci",
		"controllers-test",
		"machine-class-patch.json"), data, 0644)).NotTo(gomega.HaveOccurred()) // #nosec G306 -- Test only
}

func (c *IntegrationTestFramework) patchMachineClass(ctx context.Context, resourceName string) error {

	//adding cluster tag and noderole tag to the patch file if tags are strings
	if isTagsStrings == "true" {
		c.updatePatchFile()
	}

	data, err := os.ReadFile(
		filepath.Join("..", "..", "..",
			".ci",
			"controllers-test",
			"machine-class-patch.json"),
	)
	if err != nil {
		// Error reading file. So skipping patch
		log.Panicln("error while reading patch file. So skipping it")
	}
	retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		_, err := c.ControlCluster.McmClient.
			MachineV1alpha1().
			MachineClasses(controlClusterNamespace).
			Patch(
				ctx,
				resourceName,
				types.MergePatchType,
				data,
				metav1.PatchOptions{},
			)
		return err
	})
	if retryErr != nil {
		return retryErr
	}

	return nil
}

func (c *IntegrationTestFramework) patchIntegrationTestMachineClasses() error {
	for _, machineClassName := range testMachineClassResources {
		if _, err := c.ControlCluster.McmClient.
			MachineV1alpha1().
			MachineClasses(controlClusterNamespace).
			Get(context.Background(), machineClassName,
				metav1.GetOptions{},
			); err == nil {
			//patch
			if err = c.patchMachineClass(context.Background(), machineClassName); err != nil {
				return err
			}
		}
	}
	return nil
}

// setupMachineClass reads the control cluster machineclass resource and creates a duplicate of it.
// Additionally it adds the delta part found in file  .ci/controllers-test/machine-class-patch.json of provider specific repo
// OR
// use machineclass yaml file instead for creating machine class from scratch
func (c *IntegrationTestFramework) setupMachineClass() error {
	// if isControlClusterIsShootsSeed is true and v1 machineClass is not provided by the user, then use machineclass from cluster
	// probe for machine-class in the identified namespace and then creae a copy of this machine-class with
	// additional delta available in machine-class-patch.json
	// eg. tag (providerSpec.tags)  \"mcm-integration-test: "true"\"

	ctx := context.Background()
	if isControlSeed == "true" && len(v1MachineClassPath) == 0 {
		machineClasses, err := c.ControlCluster.McmClient.
			MachineV1alpha1().
			MachineClasses(controlClusterNamespace).
			List(ctx,
				metav1.ListOptions{},
			)
		if err != nil {
			return err
		}
		// Create machine-class using any of existing machineclass resource and yaml combined
		for _, resourceName := range testMachineClassResources {
			// create machine-class
			_, createErr := c.ControlCluster.McmClient.
				MachineV1alpha1().
				MachineClasses(controlClusterNamespace).
				Create(
					ctx,
					&v1alpha1.MachineClass{
						ObjectMeta: metav1.ObjectMeta{
							Name:        resourceName,
							Labels:      machineClasses.Items[0].ObjectMeta.Labels,
							Annotations: machineClasses.Items[0].ObjectMeta.Annotations,
						},
						NodeTemplate:         machineClasses.Items[0].NodeTemplate, // virtual
						ProviderSpec:         machineClasses.Items[0].ProviderSpec,
						SecretRef:            machineClasses.Items[0].SecretRef,
						CredentialsSecretRef: machineClasses.Items[0].CredentialsSecretRef,
						Provider:             machineClasses.Items[0].Provider,
					},
					metav1.CreateOptions{},
				)
			if createErr != nil {
				return createErr
			}
		}
	} else {
		//use yaml files
		if len(v1MachineClassPath) == 0 {
			return fmt.Errorf("no machine class path specified in Makefile")
		}

		//ensuring machineClass name is 'test-mc-v1'
		runtimeobj, _, _ := helpers.ParseK8sYaml(v1MachineClassPath)
		resource := runtimeobj[0].(*v1alpha1.MachineClass)
		if resource.Name != "test-mc-v1" {
			return fmt.Errorf("name of provided machine class is not `test-mc-v1`.Please change the name and run again")
		}

		log.Printf("Applying machineclass yaml file: %s", v1MachineClassPath)
		if err := c.ControlCluster.
			ApplyFiles(
				v1MachineClassPath,
				controlClusterNamespace,
			); err != nil {
			return err
		}

		if len(v2MachineClassPath) != 0 {
			log.Printf("Applying v2 machineclass yaml file: %s", v2MachineClassPath)
			if err := c.ControlCluster.
				ApplyFiles(
					v2MachineClassPath,
					controlClusterNamespace,
				); err != nil {
				return err
			}
		} else {
			// use v1MachineClass but with different name
			machineClass, err := c.ControlCluster.McmClient.
				MachineV1alpha1().
				MachineClasses(controlClusterNamespace).
				Get(
					ctx,
					testMachineClassResources[0],
					metav1.GetOptions{},
				)
			if err != nil {
				return err
			}
			_, createErr := c.ControlCluster.McmClient.
				MachineV1alpha1().
				MachineClasses(controlClusterNamespace).
				Create(ctx,
					&v1alpha1.MachineClass{
						ObjectMeta: metav1.ObjectMeta{
							Name:        testMachineClassResources[1],
							Labels:      machineClass.ObjectMeta.Labels,
							Annotations: machineClass.ObjectMeta.Annotations,
						},
						ProviderSpec:         machineClass.ProviderSpec,
						SecretRef:            machineClass.SecretRef,
						CredentialsSecretRef: machineClass.CredentialsSecretRef,
						Provider:             machineClass.Provider,
					},
					metav1.CreateOptions{})
			if createErr != nil {
				return createErr
			}

		}
	}

	//patching the integration test machineclasses with IT role tag
	if err := c.patchIntegrationTestMachineClasses(); err != nil {
		return err
	}
	return nil
}

// rotateLogFile takes file name as input and returns a file object obtained by os.Create
// If the file exists already then it renames it so that a new file can be created
func rotateOrAppendLogFile(fileName string, shouldRotate bool) (*os.File, error) {
	if !shouldRotate {
		if _, err := os.Stat(fileName); os.IsNotExist(err) {
			return os.Create(fileName) // #nosec G304 -- Test only
		}
		return os.OpenFile(fileName, os.O_APPEND|os.O_WRONLY, 0600) // #nosec G304 -- Test only
	}
	if _, err := os.Stat(fileName); err == nil { // !strings.Contains(err.Error(), "no such file or directory") {
		noOfFiles := 0
		temp := fileName + "." + strconv.Itoa(noOfFiles+1)
		_, err := os.Stat(temp)
		// Finding the log files ending with ".x" where x >= 1 and renaming
		for err == nil {
			noOfFiles++
			temp = fileName + "." + strconv.Itoa(noOfFiles+1)
			_, err = os.Stat(temp)
		}
		for i := noOfFiles; i > 0; i-- {
			f := fmt.Sprintf("%s.%d", fileName, i)
			fNew := fmt.Sprintf("%s.%d", fileName, i+1)
			if err := os.Rename(f, fNew); err != nil {
				return nil, fmt.Errorf("failed to rename file %s to %s: %w", f, fNew, err)
			}
		}
		// Renaming the log file without suffix ".x" to log file ending with ".1"
		fNew := fmt.Sprintf("%s.%d", fileName, 1)
		if err := os.Rename(fileName, fNew); err != nil {
			return nil, fmt.Errorf("failed to rename file %s to %s: %w", fileName, fNew, err)
		}
	}
	// Creating a new log file
	return os.Create(fileName) // #nosec G304 -- Test only
}

// runControllersLocally run the machine controller and machine controller manager binary locally
func (c *IntegrationTestFramework) runControllersLocally() {
	ginkgo.By("Starting Machine Controller ")
	args := strings.Fields(
		fmt.Sprintf(
			"make --directory=%s start CONTROL_KUBECONFIG=%s TARGET_KUBECONFIG=%s CONTROL_NAMESPACE=%s LEADER_ELECT=false ",
			"../../..",
			c.ControlCluster.KubeConfigFilePath,
			c.TargetCluster.KubeConfigFilePath,
			controlClusterNamespace),
	)
	outputFile, err := rotateOrAppendLogFile(mcLogFile, true)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	mcsession, err = gexec.Start(exec.Command(args[0], args[1:]...), outputFile, outputFile) // #nosec G204 -- Test only
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	gomega.Expect(mcsession.ExitCode()).Should(gomega.Equal(-1))

	ginkgo.By("Starting Machine Controller Manager")
	args = strings.Fields(
		fmt.Sprintf(
			"make --directory=%s start CONTROL_KUBECONFIG=%s TARGET_KUBECONFIG=%s CONTROL_NAMESPACE=%s LEADER_ELECT=false MACHINE_SAFETY_OVERSHOOTING_PERIOD=300ms",
			mcmRepoPath,
			c.ControlCluster.KubeConfigFilePath,
			c.TargetCluster.KubeConfigFilePath,
			controlClusterNamespace),
	)
	outputFile, err = rotateOrAppendLogFile(mcmLogFile, true)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	mcmsession, err = gexec.Start(exec.Command(args[0], args[1:]...), outputFile, outputFile) // #nosec G204 -- Test only
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	gomega.Expect(mcmsession.ExitCode()).Should(gomega.Equal(-1))
}

// SetupBeforeSuite performs the initial setup for the test by
// - Checks control cluster and target clusters are accessible and initializes ControlCluster and TargetCluster.
// - Check and optionally create crds (machineclass, machines, machinesets and machine deployment)
// - using kubernetes/crds directory of the mcm repo.
// - Setup controller processes either as a pod in the control cluster or running locally.
// - Setup machineclass to use either by copying existing machineclass in seed cluster or by applying file.
// - invokes InitializeResourcesTracker or rti for orphan resource check.
func (c *IntegrationTestFramework) SetupBeforeSuite() {
	ctx := context.Background()
	log.SetOutput(ginkgo.GinkgoWriter)

	ginkgo.By("Checking for the clusters if provided are available")
	gomega.Expect(c.initalizeClusters()).To(gomega.BeNil())

	ginkgo.By("Killing any existing processes")
	stopMCM(ctx)
	//setting up MCM either locally or by deploying after checking conditions

	checkMcmRepoAvailable()

	if isControlSeed == "true" && isVirtualProvider != "true" {
		ginkgo.By("Scaledown existing machine controllers")
		gomega.Expect(c.scaleMcmDeployment(0)).To(gomega.BeNil())
	} else if isControlSeed != "true" {
		//TODO : Scaledown the MCM deployment of the actual seed of the target cluster

		//create the custom resources in the control cluster using yaml files
		//available in kubernetes/crds directory of machine-controller-manager repo
		//resources to be applied are machineclass, machines, machinesets and machinedeployment
		ginkgo.By("Applying kubernetes/crds into control cluster")
		gomega.Expect(c.ControlCluster.
			ApplyFiles(
				filepath.Join(mcmRepoPath, "kubernetes/crds"), controlClusterNamespace)).To(gomega.BeNil())
	}

	c.runControllersLocally()

	ginkgo.By("Cleaning any old resources")
	if c.ControlCluster.McmClient != nil {
		timeout := int64(900)
		c.cleanTestResources(ctx, timeout)
	}

	ginkgo.By("Setup MachineClass")
	gomega.Expect(c.setupMachineClass()).To(gomega.BeNil())

	// initialize orphan resource tracker
	ginkgo.By("Looking for machineclass resource in the control cluster")
	machineClass, err := c.ControlCluster.McmClient.
		MachineV1alpha1().
		MachineClasses(controlClusterNamespace).
		Get(ctx, testMachineClassResources[0], metav1.GetOptions{})
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	clusterName := targetClusterName

	ginkgo.By("Looking for secrets refered in machineclass in the control cluster")
	secretData, err := c.ControlCluster.
		GetSecretData(
			machineClass.Name,
			machineClass.SecretRef,
			machineClass.CredentialsSecretRef,
		)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	ginkgo.By("Initializing orphan resource tracker")
	err = c.resourcesTracker.InitializeResourcesTracker(machineClass, secretData, clusterName)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	log.Println("orphan resource tracker initialized")
}

// BeforeEachCheck checks if all the nodes are ready and the controllers are runnings
func (c *IntegrationTestFramework) BeforeEachCheck() {
	ginkgo.BeforeEach(func() {
		ginkgo.By("Checking machineController process is running")
		gomega.Expect(mcsession.ExitCode()).Should(gomega.Equal(-1))
		ginkgo.By("Checking machineControllerManager process is running")
		gomega.Expect(mcmsession.ExitCode()).Should(gomega.Equal(-1))
		ginkgo.By("Checking nodes in target cluster are healthy")
		gomega.Eventually(
			c.TargetCluster.GetNumberOfReadyNodes,
			c.timeout,
			c.pollingInterval).
			Should(gomega.BeNumerically("==", c.TargetCluster.GetNumberOfNodes()))
	})
}

// ControllerTests runs common tests like ...
// machine resource creation and deletion,
// machine deployment resource creation, scale-up, scale-down, update and deletion. And
// orphan resource check by invoking IsOrphanedResourcesAvailable from rti
func (c *IntegrationTestFramework) ControllerTests() {
	ctx := context.Background()
	// Testcase #01 | Machine
	ginkgo.Describe("machine resource", func() {
		var initialNodes int16
		ginkgo.Context("creation", func() {
			ginkgo.It("should not lead to any errors and add 1 more node in target cluster", func() {
				// In case of existing deployments creating nodes when starting virtual
				// provider, the change in node count can be >1, this delay prevents
				// checking node count immediately to allow for a correct initial count
				if isVirtualProvider == "true" {
					time.Sleep(2 * time.Second)
				}
				// Probe nodes currently available in target cluster
				initialNodes = c.TargetCluster.GetNumberOfNodes()
				ginkgo.By("Checking for errors")
				gomega.Expect(c.ControlCluster.CreateMachine(controlClusterNamespace, gnaSecretNameLabelValue)).To(gomega.BeNil())

				ginkgo.By("Waiting until number of ready nodes is 1 more than initial nodes")
				gomega.Eventually(
					c.TargetCluster.GetNumberOfNodes,
					c.timeout,
					c.pollingInterval).
					Should(gomega.BeNumerically("==", initialNodes+1))

				gomega.Eventually(
					c.TargetCluster.GetNumberOfReadyNodes,
					c.timeout,
					c.pollingInterval).
					Should(gomega.BeNumerically("==", initialNodes+1))
			})
		})

		ginkgo.Context("deletion", func() {
			ginkgo.Context("when machines available", func() {
				ginkgo.It("should not lead to errors and remove 1 node in target cluster", func() {
					machinesList, _ := c.ControlCluster.McmClient.
						MachineV1alpha1().
						Machines(controlClusterNamespace).
						List(ctx, metav1.ListOptions{})
					if len(machinesList.Items) != 0 {
						ginkgo.By("Checking for errors")
						gomega.Expect(
							c.ControlCluster.McmClient.
								MachineV1alpha1().
								Machines(controlClusterNamespace).
								Delete(ctx, testMachineName, metav1.DeleteOptions{})).
							Should(gomega.BeNil(), "No Errors while deleting machine")

						ginkgo.By("Waiting until test-machine machine object is deleted")
						gomega.Eventually(
							c.ControlCluster.IsTestMachineDeleted,
							c.timeout,
							c.pollingInterval).
							Should(gomega.BeTrue())

						ginkgo.By("Waiting until number of ready nodes is equal to number of initial nodes")
						gomega.Eventually(
							c.TargetCluster.GetNumberOfNodes,
							c.timeout,
							c.pollingInterval).
							Should(gomega.BeNumerically("==", initialNodes))
						gomega.Eventually(
							c.TargetCluster.GetNumberOfReadyNodes,
							c.timeout,
							c.pollingInterval).
							Should(gomega.BeNumerically("==", initialNodes))
					}

				})
			})
			ginkgo.Context("when machines are not available", func() {
				// delete one machine (non-existent) by random text as name of resource
				// check there are no changes to nodes
				ginkgo.It("should keep nodes intact", func() {
					machinesList, _ := c.ControlCluster.McmClient.
						MachineV1alpha1().
						Machines(controlClusterNamespace).
						List(ctx, metav1.ListOptions{})

					if len(machinesList.Items) == 0 {
						err := c.ControlCluster.McmClient.
							MachineV1alpha1().
							Machines(controlClusterNamespace).
							Delete(ctx, "test-machine-dummy", metav1.DeleteOptions{})
						ginkgo.By("Checking for errors")
						gomega.Expect(err).To(gomega.HaveOccurred())
						ginkgo.By("Checking number of nodes is eual to number of initial nodes")
						gomega.Expect(c.TargetCluster.GetNumberOfNodes()).To(gomega.BeEquivalentTo(initialNodes))
					} else {
						ginkgo.By("Skipping as there are machines available and this check can't be performed")
					}
				})
			})
		})
	})

	// Testcase #02 | machine deployment
	ginkgo.Describe("machine deployment resource", func() {
		var initialNodes int16 // initialization should be part of creation test logic
		initialEventCount := 0
		ginkgo.Context("creation with replicas=0, scale up with replicas=1", func() {
			ginkgo.It("should not lead to errors and add 1 more node to target cluster", func() {
				var machineDeployment *v1alpha1.MachineDeployment
				//probe initialnodes before continuing
				initialNodes = c.TargetCluster.GetNumberOfNodes()

				ginkgo.By("Checking for errors")
				gomega.Expect(c.ControlCluster.CreateMachineDeployment(controlClusterNamespace, gnaSecretNameLabelValue, 0)).To(gomega.BeNil())

				ginkgo.By("Waiting for Machine Set to be created")
				gomega.Eventually(func() int { return len(c.getTestMachineSets(ctx, controlClusterNamespace)) }, c.timeout, c.pollingInterval).Should(gomega.BeNumerically("==", 1))

				ginkgo.By("Updating machineDeployment replicas to 1")
				retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
					machineDeployment, _ = c.ControlCluster.McmClient.
						MachineV1alpha1().
						MachineDeployments(controlClusterNamespace).
						Get(ctx, testMachineDeploymentName, metav1.GetOptions{})
					machineDeployment.Spec.Replicas = 1
					_, updateErr := c.ControlCluster.McmClient.
						MachineV1alpha1().
						MachineDeployments(controlClusterNamespace).
						Update(ctx, machineDeployment, metav1.UpdateOptions{})
					return updateErr
				})
				gomega.Expect(retryErr).NotTo(gomega.HaveOccurred())

				//check machineDeploymentStatus to make sure correct condition is reflected
				ginkgo.By("Checking if machineDeployment's status has been updated with correct conditions")
				gomega.Eventually(func() bool {
					var err error
					machineDeployment, err = c.ControlCluster.McmClient.
						MachineV1alpha1().
						MachineDeployments(controlClusterNamespace).
						Get(ctx, testMachineDeploymentName, metav1.GetOptions{})
					gomega.Expect(err).NotTo(gomega.HaveOccurred())
					if len(machineDeployment.Status.Conditions) != 2 {
						return false
					}
					isMCDAvailable := false
					hasMCDProgressed := false
					for _, condition := range machineDeployment.Status.Conditions {
						if condition.Type == v1alpha1.MachineDeploymentAvailable && condition.Status == v1alpha1.ConditionTrue && condition.Reason == controller.MinimumReplicasAvailable {
							isMCDAvailable = true
						}
						if condition.Type == v1alpha1.MachineDeploymentProgressing && condition.Status == v1alpha1.ConditionTrue && condition.Reason == controller.NewISAvailableReason {
							hasMCDProgressed = true
						}
					}
					return hasMCDProgressed && isMCDAvailable && machineDeployment.Status.AvailableReplicas == 1
				}, c.timeout, c.pollingInterval).Should(gomega.BeTrue())

				//check number of ready nodes
				ginkgo.By("Checking number of ready nodes==1")
				gomega.Eventually(
					c.TargetCluster.GetNumberOfNodes,
					c.timeout,
					c.pollingInterval).
					Should(gomega.BeNumerically("==", initialNodes+1))
				gomega.Eventually(
					c.TargetCluster.GetNumberOfReadyNodes,
					c.timeout,
					c.pollingInterval).
					Should(gomega.BeNumerically("==", initialNodes+1))
				ginkgo.By("Fetching initial number of machineset freeze events")
				initialEventCount = c.machineSetFreezeEventCount(ctx, controlClusterNamespace)
			})
		})
		ginkgo.Context("scale-up with replicas=6", func() {
			ginkgo.It("should not lead to errors and add further 5 nodes to target cluster", func() {
				retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
					machineDployment, _ := c.ControlCluster.McmClient.
						MachineV1alpha1().
						MachineDeployments(controlClusterNamespace).
						Get(ctx, testMachineDeploymentName, metav1.GetOptions{})
					machineDployment.Spec.Replicas = 6
					_, updateErr := c.ControlCluster.McmClient.
						MachineV1alpha1().
						MachineDeployments(controlClusterNamespace).
						Update(ctx, machineDployment, metav1.UpdateOptions{})
					return updateErr
				})
				ginkgo.By("Checking for errors")
				gomega.Expect(retryErr).NotTo(gomega.HaveOccurred())
				ginkgo.By("Checking number of ready nodes are 6 more than initial")
				gomega.Eventually(
					c.TargetCluster.GetNumberOfNodes,
					c.timeout,
					c.pollingInterval).
					Should(gomega.BeNumerically("==", initialNodes+6))
				gomega.Eventually(
					c.TargetCluster.GetNumberOfReadyNodes,
					c.timeout,
					c.pollingInterval).
					Should(gomega.BeNumerically("==", initialNodes+6))
			})

		})
		ginkgo.Context("scale-down with replicas=2", func() {
			ginkgo.It("should not lead to errors and remove 4 nodes from target cluster", func() {
				retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
					machineDployment, _ := c.ControlCluster.McmClient.
						MachineV1alpha1().
						MachineDeployments(controlClusterNamespace).
						Get(ctx, testMachineDeploymentName, metav1.GetOptions{})
					machineDployment.Spec.Replicas = 2
					_, updateErr := c.ControlCluster.McmClient.
						MachineV1alpha1().
						MachineDeployments(controlClusterNamespace).
						Update(ctx, machineDployment, metav1.UpdateOptions{})
					return updateErr
				})
				ginkgo.By("Checking for errors")
				gomega.Expect(retryErr).NotTo(gomega.HaveOccurred())

				ginkgo.By("Checking number of ready nodes are 2 more than initial")
				gomega.Eventually(
					c.TargetCluster.GetNumberOfNodes,
					c.timeout,
					c.pollingInterval).
					Should(gomega.BeNumerically("==", initialNodes+2))
				gomega.Eventually(
					c.TargetCluster.GetNumberOfReadyNodes,
					c.timeout,
					c.pollingInterval).
					Should(gomega.BeNumerically("==", initialNodes+2))
			})
			// rapid scaling back to 2, should lead to freezing and unfreezing
			ginkgo.It("should freeze and unfreeze machineset temporarily", func() {
				gomega.Eventually(func() int {
					return c.machineSetFreezeEventCount(ctx, controlClusterNamespace)
				}, c.timeout, c.pollingInterval).Should(gomega.Equal(initialEventCount + 2))
			})
		})
		ginkgo.Context("updation to v2 machine-class and replicas=4", func() {
			// Update machine type -> machineDeployment.spec.template.spec.class.name = "test-mc-v2"
			// Also scale up replicas by 4
			ginkgo.It("should upgrade machines and add more nodes to target", func() {
				retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
					machineDployment, _ := c.ControlCluster.McmClient.
						MachineV1alpha1().
						MachineDeployments(controlClusterNamespace).
						Get(ctx, testMachineDeploymentName, metav1.GetOptions{})
					machineDployment.Spec.Template.Spec.Class.Name = testMachineClassResources[1]
					machineDployment.Spec.Replicas = 4
					_, updateErr := c.ControlCluster.McmClient.
						MachineV1alpha1().
						MachineDeployments(controlClusterNamespace).
						Update(ctx, machineDployment, metav1.UpdateOptions{})
					return updateErr
				})
				ginkgo.By("Checking for errors")
				gomega.Expect(retryErr).NotTo(gomega.HaveOccurred())
				ginkgo.By("UpdatedReplicas to be 4")
				gomega.Eventually(func() int {
					machineDeployment, err := c.ControlCluster.McmClient.
						MachineV1alpha1().
						MachineDeployments(controlClusterNamespace).
						Get(ctx, testMachineDeploymentName, metav1.GetOptions{})
					if err != nil {
						log.Println("Failed to get machinedeployment object")
					}
					return int(machineDeployment.Status.UpdatedReplicas)
				}, c.timeout, c.pollingInterval).Should(gomega.BeNumerically("==", 4))
				ginkgo.By("AvailableReplicas to be 4")
				gomega.Eventually(func() int {
					machineDeployment, err := c.ControlCluster.McmClient.
						MachineV1alpha1().
						MachineDeployments(controlClusterNamespace).
						Get(ctx, testMachineDeploymentName, metav1.GetOptions{})
					if err != nil {
						log.Println("Failed to get machinedeployment object")
					}
					return int(machineDeployment.Status.AvailableReplicas)
				}, c.timeout, c.pollingInterval).Should(gomega.BeNumerically("==", 4))
				ginkgo.By("Number of ready nodes be 4 more")
				gomega.Eventually(
					c.TargetCluster.GetNumberOfNodes,
					c.timeout,
					c.pollingInterval).
					Should(gomega.BeNumerically("==", initialNodes+4))
				gomega.Eventually(
					c.TargetCluster.GetNumberOfReadyNodes,
					c.timeout,
					c.pollingInterval).
					Should(gomega.BeNumerically("==", initialNodes+4))

			})
		})
		ginkgo.Context("deletion", func() {
			ginkgo.Context("When there are machine deployment(s) available in control cluster", func() {
				ginkgo.It("should not lead to errors and list only initial nodes", func() {
					_, err := c.ControlCluster.McmClient.
						MachineV1alpha1().
						MachineDeployments(controlClusterNamespace).
						Get(ctx, testMachineDeploymentName, metav1.GetOptions{})
					if err == nil {
						ginkgo.By("Checking for errors")
						gomega.Expect(
							c.ControlCluster.McmClient.
								MachineV1alpha1().
								MachineDeployments(controlClusterNamespace).
								Delete(
									ctx,
									testMachineDeploymentName,
									metav1.DeleteOptions{},
								)).
							Should(gomega.BeNil())
						ginkgo.By("Waiting until number of ready nodes is equal to number of initial  nodes")
						gomega.Eventually(
							c.TargetCluster.GetNumberOfNodes,
							c.timeout,
							c.pollingInterval).
							Should(gomega.BeNumerically("==", initialNodes))
						gomega.Eventually(
							c.TargetCluster.GetNumberOfReadyNodes,
							c.timeout,
							c.pollingInterval).
							Should(gomega.BeNumerically("==", initialNodes))
					}
				})
			})
		})
	})

	// Testcase #03 | Orphaned Resources
	ginkgo.Describe("orphaned resources", func() {
		ginkgo.Context("when the hyperscaler resources are queried", func() {
			ginkgo.It("should have been deleted", func() {
				// if available, should delete orphaned resources in the cloud provider
				ginkgo.By("Querying and comparing")
				gomega.Eventually(
					c.resourcesTracker.IsOrphanedResourcesAvailable,
					c.timeout,
					c.pollingInterval).
					Should(gomega.BeEquivalentTo(false))

			})
		})
	})
}

// Cleanup performs rollback of original resources and removes any machines created by the test
func (c *IntegrationTestFramework) Cleanup() {

	ctx := context.Background()

	ginkgo.By("Running Cleanup")
	//running locally, means none of the image is specified
	for range 5 {
		if mcsession.ExitCode() != -1 {
			ginkgo.By("Restarting Machine Controller ")
			outputFile, err := rotateOrAppendLogFile(mcLogFile, false)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			_, err = outputFile.WriteString("\n------------RESTARTED MC------------\n")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			args := strings.Fields(
				fmt.Sprintf(
					"make --directory=%s start CONTROL_KUBECONFIG=%s TARGET_KUBECONFIG=%s CONTROL_NAMESPACE=%s LEADER_ELECT=false ",
					"../../..",
					c.ControlCluster.KubeConfigFilePath,
					c.TargetCluster.KubeConfigFilePath,
					controlClusterNamespace),
			)
			mcsession, err = gexec.Start(exec.Command(args[0], args[1:]...), outputFile, outputFile) // #nosec G204 -- Test only
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			break
		}
		time.Sleep(2 * time.Second)
	}
	for range 5 {
		if mcmsession.ExitCode() != -1 {
			ginkgo.By("Restarting Machine Controller Manager")
			outputFile, err := rotateOrAppendLogFile(mcmLogFile, false)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			_, err = outputFile.WriteString("\n------------RESTARTED MCM------------\n")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			args := strings.Fields(
				fmt.Sprintf(
					"make --directory=%s start CONTROL_KUBECONFIG=%s TARGET_KUBECONFIG=%s CONTROL_NAMESPACE=%s LEADER_ELECT=false MACHINE_SAFETY_OVERSHOOTING_PERIOD=300ms",
					mcmRepoPath,
					c.ControlCluster.KubeConfigFilePath,
					c.TargetCluster.KubeConfigFilePath,
					controlClusterNamespace),
			)
			mcmsession, err = gexec.Start(exec.Command(args[0], args[1:]...), outputFile, outputFile) // #nosec G204 -- Test only
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			break
		}
		time.Sleep(2 * time.Second)
	}

	if preserveCRDuringCleanup == "true" {
		log.Printf("Preserve CRD: %s\n", preserveCRDuringCleanup)
	} else if c.ControlCluster.McmClient != nil {
		timeout := int64(900)
		c.cleanTestResources(ctx, timeout)
	}

	ginkgo.By("Killing any existing processes")
	stopMCM(ctx)

	if isControlSeed == "true" {
		// This is needed when IT suite runs locally against a Control & Target cluster
		ginkgo.By("Scale back the existing machine controllers")
		if err := c.scaleMcmDeployment(1); err != nil {
			log.Println(err.Error())
		}

	} else {
		log.Println("Deleting crds")
		if err := c.ControlCluster.DeleteResources(
			filepath.Join(mcmRepoPath, "kubernetes/crds"),
			controlClusterNamespace,
		); err != nil {
			log.Printf("Error occured while deleting crds. %s", err.Error())
		}
	}

}

func (c *IntegrationTestFramework) cleanTestResources(ctx context.Context, timeout int64) {
	// Check and delete machinedeployment resource
	if err := c.cleanMachineDeployment(ctx, testMachineDeploymentName, timeout); err != nil {
		log.Println(err.Error())
	}
	// Check and delete machine resource
	if err := c.cleanMachine(ctx, testMachineName, timeout); err != nil {
		log.Println(err.Error())
	}

	for _, machineClassName := range testMachineClassResources {
		// Check and delete machine class resource
		if err := c.cleanMachineClass(ctx, machineClassName, timeout); err != nil {
			log.Println(err.Error())
		}
	}
}

func (c *IntegrationTestFramework) cleanMachineDeployment(ctx context.Context, name string, timeout int64) error {
	_, err := c.ControlCluster.McmClient.
		MachineV1alpha1().
		MachineDeployments(controlClusterNamespace).
		Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	log.Printf("deleting %s\n", name)
	selector := fmt.Sprintf("metadata.name=%s", name)
	watchMachinesDepl, _ := c.ControlCluster.McmClient.
		MachineV1alpha1().
		MachineDeployments(controlClusterNamespace).
		Watch(ctx, metav1.ListOptions{
			TimeoutSeconds: &timeout,
			FieldSelector:  selector,
		})
	for event := range watchMachinesDepl.ResultChan() {
		gomega.Expect(c.ControlCluster.McmClient.
			MachineV1alpha1().
			MachineDeployments(controlClusterNamespace).
			Delete(ctx, name, metav1.DeleteOptions{})).To(gomega.Or(gomega.Succeed(), matchers.BeNotFoundError()))
		if event.Type == watch.Deleted {
			watchMachinesDepl.Stop()
			log.Println("machinedeployment deleted")
		}
	}
	return nil
}

func (c *IntegrationTestFramework) cleanMachine(ctx context.Context, name string, timeout int64) error {
	_, err := c.ControlCluster.McmClient.
		MachineV1alpha1().
		Machines(controlClusterNamespace).
		Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	log.Printf("deleting %s\n", name)
	selector := fmt.Sprintf("metadata.name=%s", name)
	watchMachines, _ := c.ControlCluster.McmClient.
		MachineV1alpha1().
		Machines(controlClusterNamespace).
		Watch(ctx, metav1.ListOptions{
			TimeoutSeconds: &timeout,
			FieldSelector:  selector,
		}) //ResourceVersion: machineObj.ResourceVersion
	for event := range watchMachines.ResultChan() {
		gomega.Expect(c.ControlCluster.McmClient.
			MachineV1alpha1().
			Machines(controlClusterNamespace).
			Delete(ctx, name, metav1.DeleteOptions{})).To(gomega.Or(gomega.Succeed(), matchers.BeNotFoundError()))
		if event.Type == watch.Deleted {
			watchMachines.Stop()
			log.Println("machine deleted")
		}
	}
	return nil
}

func (c *IntegrationTestFramework) cleanMachineClass(ctx context.Context, machineClassName string, timeout int64) error {
	_, err := c.ControlCluster.McmClient.
		MachineV1alpha1().
		MachineClasses(controlClusterNamespace).
		Get(ctx, machineClassName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	log.Printf("deleting %s machineclass", machineClassName)
	selector := fmt.Sprintf("metadata.name=%s", machineClassName)
	watchMachineClass, _ := c.ControlCluster.McmClient.
		MachineV1alpha1().
		MachineClasses(controlClusterNamespace).
		Watch(ctx, metav1.ListOptions{
			TimeoutSeconds: &timeout,
			FieldSelector:  selector,
		})
	for event := range watchMachineClass.ResultChan() {
		gomega.Expect(c.ControlCluster.McmClient.
			MachineV1alpha1().
			MachineClasses(controlClusterNamespace).
			Delete(ctx, machineClassName, metav1.DeleteOptions{})).To(gomega.Or(gomega.Succeed(), matchers.BeNotFoundError()))
		if event.Type == watch.Deleted {
			watchMachineClass.Stop()
			log.Println("machineclass deleted")
		}
	}
	return nil
}

func stopMCM(ctx context.Context) {
	processesToKill := []string{
		"machine-controller-manager --control-kubeconfig", // virtual
		"machine-controller --control-kubeconfig",         // virtual
		"controller_manager --control-kubeconfig",
		"main --control-kubeconfig",
		"go run cmd/machine-controller/main.go",
		"go run cmd/machine-controller-manager/controller_manager.go",
		"make --directory=../../..",
	}
	for _, proc := range processesToKill {
		pids, err := findAndKillProcess(ctx, proc)
		if err != nil {
			log.Println(err.Error())
		}
		if len(pids) > 0 {
			log.Printf("stopMCM killed MCM process(es) with pid(s): %v\n", pids)
		}
	}
}

func findAndKillProcess(ctx context.Context, prefix string) (pids []int, err error) {
	pids, err = findPidsByPrefix(ctx, prefix)
	if len(pids) != 0 {
		var proc *os.Process
		for _, pid := range pids {
			proc, err = os.FindProcess(pid)
			if err != nil {
				err = fmt.Errorf("failed to find process with PID %d: %v", pid, err)
				return
			}
			err = proc.Kill()
			if err != nil {
				err = fmt.Errorf("failed to kill process with PID %d: %v", pid, err)
				return
			}
		}
	}
	return
}

func findPidsByPrefix(ctx context.Context, prefix string) (pids []int, err error) {
	cmd := exec.CommandContext(ctx, "ps", "-e", "-o", "pid,command")
	psOutput, err := cmd.Output()
	if err != nil {
		log.Printf("FindProcess could not run ps command: %v", err)
		return
	}
	scanner := bufio.NewScanner(bytes.NewReader(psOutput))
	var pid int
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		// Process the PID and command columns
		pidStr := fields[0]
		commandPath := fields[1]
		commandName := path.Base(commandPath)
		if len(fields) > 2 {
			commandName = commandName + " " + strings.Join(fields[2:], " ")
		}

		if strings.HasPrefix(commandName, prefix) {
			log.Println(commandName)
			pid, err = strconv.Atoi(pidStr)
			if err != nil {
				err = fmt.Errorf("invalid pid: %s", pidStr)
				return
			}
			pids = append(pids, pid)
		}
	}
	return
}

func (c *IntegrationTestFramework) getTestMachineSets(ctx context.Context, namespace string) []string {
	machineSets, err := c.ControlCluster.McmClient.MachineV1alpha1().MachineSets(namespace).List(ctx, metav1.ListOptions{})
	testMachineSets := []string{}
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	for _, machineSet := range machineSets.Items {
		if machineSet.OwnerReferences[0].Name == testMachineDeploymentName {
			testMachineSets = append(testMachineSets, machineSet.Name)
		}
	}
	return testMachineSets
}

func (c *IntegrationTestFramework) machineSetFreezeEventCount(ctx context.Context, namespace string) int {
	eventCount := 0
	testMachineSets := c.getTestMachineSets(ctx, namespace)
	for _, machineSet := range testMachineSets {
		for _, reason := range []string{"reason=FrozeMachineSet", "reason=UnfrozeMachineSet"} {
			event := fmt.Sprintf("%s,involvedObject.name=%s", reason, machineSet)
			frozenEvents, err := c.ControlCluster.Clientset.
				CoreV1().
				Events(namespace).
				List(ctx, metav1.ListOptions{
					FieldSelector: event,
				})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			eventCount += len(frozenEvents.Items)
		}
	}
	return eventCount
}

func checkMcmRepoAvailable() {
	ginkgo.By("Checking Machine-Controller-Manager repo is available at: " + mcmRepoPath)
	_, err := os.Stat(mcmRepoPath)
	gomega.Expect(err).To(gomega.BeNil(), "No MCM dir at: "+mcmRepoPath)

	_, err = os.Stat(mcmRepoPath + "/.git")
	gomega.Expect(err).To(gomega.BeNil(), "Not a git repo at: "+mcmRepoPath)

}
