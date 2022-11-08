package common

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	v1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/gardener/machine-controller-manager/pkg/test/integration/common/helpers"
	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	appsV1 "k8s.io/api/apps/v1"
	coreV1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/util/retry"
)

const dwdIgnoreScalingAnnotation = "dependency-watchdog.gardener.cloud/ignore-scaling"

var (
	// path for storing log files (mcm & mc processes)
	targetDir = filepath.Join("..", "..", "..", ".ci", "controllers-test", "logs")
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

	// Update namespace to use
	if c.ControlCluster.IsSeed(c.TargetCluster) {
		_, err := c.TargetCluster.ClusterName()
		if err != nil {
			log.Println("Failed to determine shoot cluster namespace")
			return err
		}
		controlClusterNamespace, _ = c.TargetCluster.ClusterName()
	} else if len(controlClusterNamespace) == 0 {
		controlClusterNamespace = "default"
	}

	// setting env variable for later use
	os.Setenv("CONTROL_CLUSTER_NAMESPACE", controlClusterNamespace)
	return nil
}

func (c *IntegrationTestFramework) prepareMcmDeployment(

	mcContainerImage string,
	mcmContainerImage string,
	byCreating bool) error {

	ctx := context.Background()

	if len(machineControllerManagerDeploymentName) == 0 {
		machineControllerManagerDeploymentName = "machine-controller-manager"
	}

	if byCreating {
		// Create clusterroles and clusterrolebindings for control and target cluster
		// Create secret containing target kubeconfig file
		// Create machine-deployment using the yaml file
		controlClusterRegexp, _ := regexp.Compile("control-cluster-role")
		targetClusterRegexp, _ := regexp.Compile("target-cluster-role")
		log.Printf("Creating required roles and rolebinginds")

		err := filepath.Walk(
			filepath.Join(
				mcmRepoPath,
				"kubernetes/deployment/out-of-tree/",
			),
			func(
				path string,
				info os.FileInfo,
				err error,
			) error {

				if err != nil {
					return err
				}

				if !info.IsDir() {
					if controlClusterRegexp.MatchString(info.Name()) {
						err = c.
							ControlCluster.
							ApplyFiles(
								path,
								controlClusterNamespace,
							)
					} else if targetClusterRegexp.MatchString(info.Name()) {
						err = c.
							TargetCluster.
							ApplyFiles(
								path,
								"default",
							)
					}
				}
				return err
			})

		if err != nil {
			return err
		}

		configFile, _ := os.ReadFile(c.TargetCluster.KubeConfigFilePath)
		c.ControlCluster.Clientset.
			CoreV1().
			Secrets(controlClusterNamespace).
			Create(ctx,
				&coreV1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name: "machine-controller-manager",
					},
					Data: map[string][]byte{
						"kubeconfig": configFile,
					},
					Type: coreV1.SecretTypeOpaque,
				},
				metav1.CreateOptions{},
			)

		if err := c.ControlCluster.
			ApplyFiles(
				"../../../kubernetes/deployment.yaml",
				controlClusterNamespace,
			); err != nil {
			return err
		}
	}

	// mcmDeploymentOrigObj holds a copy of original mcm deployment
	result, getErr := c.ControlCluster.Clientset.
		AppsV1().
		Deployments(controlClusterNamespace).
		Get(ctx, machineControllerManagerDeploymentName, metav1.GetOptions{})

	if getErr != nil {
		log.Printf("failed to get latest version of Deployment: %v", getErr)
		return getErr
	}

	mcmDeploymentOrigObj = result

	// update containers spec
	providerSpecificRegexp, _ := regexp.Compile(mcContainerPrefix)

	containers := mcmDeploymentOrigObj.Spec.Template.Spec.Containers

	for i := range containers {
		if providerSpecificRegexp.MatchString(containers[i].Image) {
			// set container image to mcContainerImageTag as the name of the container contains provider
			if len(mcContainerImage) != 0 {
				containers[i].Image = mcContainerImage
				var isOptionAvailable bool
				for option := range containers[i].Command {
					if strings.Contains(containers[i].Command[option], "machine-drain-timeout=") {
						isOptionAvailable = true
						containers[i].Command[option] = "--machine-drain-timeout=5m"
					}
				}
				if !isOptionAvailable {
					containers[i].Command = append(containers[i].Command, "--machine-drain-timeout=5m")
				}
			}
		} else {
			// set container image to mcmContainerImageTag as the name of container contains provider
			if len(mcmContainerImage) != 0 {
				containers[i].Image = mcmContainerImage
			}

			// set machine-safety-overshooting-period to 300ms for freeze check to succeed
			var isOptionAvailable bool
			for option := range containers[i].Command {
				if strings.Contains(containers[i].Command[option], "machine-safety-overshooting-period=") {
					isOptionAvailable = true
					containers[i].Command[option] = "--machine-safety-overshooting-period=300ms"
				}
			}
			if !isOptionAvailable {
				containers[i].Command = append(containers[i].Command, "--machine-safety-overshooting-period=300ms")
			}
		}
	}

	// apply updated containers spec to mcmDeploymentObj in kubernetes cluster
	retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		// Retrieve the latest version of Deployment before attempting to update
		// RetryOnConflict uses exponential backoff to avoid exhausting the apiserver

		mcmDeployment, getErr := c.ControlCluster.Clientset.
			AppsV1().
			Deployments(controlClusterNamespace).
			Get(ctx,
				machineControllerManagerDeploymentName,
				metav1.GetOptions{},
			)

		if getErr != nil {
			log.Printf("failed to get latest version of Deployment: %v", getErr)
			return getErr
		}

		mcmDeployment.Spec.Template.Spec.Containers = containers
		_, updateErr := c.ControlCluster.Clientset.
			AppsV1().
			Deployments(controlClusterNamespace).
			Update(ctx, mcmDeployment, metav1.UpdateOptions{})
		return updateErr
	})

	ginkgo.By("Checking controllers are ready in kubernetes cluster")
	gomega.Eventually(
		func() error {
			deployment, err := c.ControlCluster.Clientset.
				AppsV1().
				Deployments(controlClusterNamespace).
				Get(ctx,
					machineControllerManagerDeploymentName,
					metav1.GetOptions{},
				)
			if err != nil {
				return err
			}
			if deployment.Status.ReadyReplicas == 1 {
				pods, err := c.ControlCluster.Clientset.
					CoreV1().
					Pods(controlClusterNamespace).
					List(
						ctx,
						metav1.ListOptions{
							LabelSelector: "role=machine-controller-manager",
						},
					)
				if err != nil {
					return err
				}
				podsCount := len(pods.Items)
				readyPods := 0
				for _, pod := range pods.Items {
					if len(pod.Spec.Containers) == 2 && pod.Status.ContainerStatuses[0].Ready {
						if pod.Status.ContainerStatuses[1].Ready {
							readyPods++
						} else {
							return fmt.Errorf("container(s) not ready.\n%s", pod.Status.ContainerStatuses[1].State.String())
						}
					} else {
						return fmt.Errorf("containers(s) not ready.\n%s", pod.Status.ContainerStatuses[0].State.String())
					}
				}
				if podsCount == readyPods {
					return nil
				}
			}
			return fmt.Errorf("deployment replicas are not ready.\n%s", deployment.Status.String())
		},
		c.timeout,
		c.pollingInterval).
		Should(gomega.BeNil())

	return retryErr
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
	clusterName, _ := c.TargetCluster.ClusterName()
	clusterTag := "kubernetes-io-cluster-" + clusterName
	testRoleTag := "kubernetes-io-role-integration-test"

	patchMachineClassData := MachineClassPatch{
		ProviderSpec: ProviderSpecPatch{
			Tags: []string{
				clusterName,
				clusterTag,
				testRoleTag,
			},
		},
	}

	data, _ := json.MarshalIndent(patchMachineClassData, "", " ")

	//writing to machine-class-patch.json
	_ = ioutil.WriteFile(filepath.Join("..", "..", "..",
		".ci",
		"controllers-test",
		"machine-class-patch.json"), data, 0644)
}

func (c *IntegrationTestFramework) patchMachineClass(ctx context.Context, resourceName string) error {

	//adding cluster tag and noderole tag to the patch file if tags are strings
	if isTagsStrings == "true" {
		c.updatePatchFile()
	}

	if data, err := os.ReadFile(
		filepath.Join("..", "..", "..",
			".ci",
			"controllers-test",
			"machine-class-patch.json"),
	); err == nil {
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
	} else {
		// Error reading file. So skipping patch
		log.Panicln("error while reading patch file. So skipping it")
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

	if c.ControlCluster.IsSeed(c.TargetCluster) && len(v1MachineClassPath) == 0 {
		if machineClasses, err := c.ControlCluster.McmClient.
			MachineV1alpha1().
			MachineClasses(controlClusterNamespace).
			List(ctx,
				metav1.ListOptions{},
			); err == nil {
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
			return err
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
			if machineClass, err := c.ControlCluster.McmClient.
				MachineV1alpha1().
				MachineClasses(controlClusterNamespace).
				Get(
					ctx,
					testMachineClassResources[0],
					metav1.GetOptions{},
				); err == nil {

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
			} else {
				return err
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
func rotateLogFile(fileName string) (*os.File, error) {

	if _, err := os.Stat(fileName); err == nil { // !strings.Contains(err.Error(), "no such file or directory") {
		for i := 9; i > 0; i-- {
			os.Rename(fmt.Sprintf("%s.%d", fileName, i), fmt.Sprintf("%s.%d", fileName, i+1))
		}
		os.Rename(fileName, fmt.Sprintf("%s.%d", fileName, 1))
	}

	return os.Create(fileName)
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
	outputFile, err := rotateLogFile(mcLogFile)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	mcsession, err = gexec.Start(exec.Command(args[0], args[1:]...), outputFile, outputFile)
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
	outputFile, err = rotateLogFile(mcmLogFile)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	mcmsession, err = gexec.Start(exec.Command(args[0], args[1:]...), outputFile, outputFile)
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
	mcContainerImage := os.Getenv("MC_CONTAINER_IMAGE")
	mcmContainerImage := os.Getenv("MCM_CONTAINER_IMAGE")

	ginkgo.By("Checking for the clusters if provided are available")
	gomega.Expect(c.initalizeClusters()).To(gomega.BeNil())

	//setting up MCM either locally or by deploying after checking conditions

	if c.ControlCluster.IsSeed(c.TargetCluster) {

		if len(mcContainerImage) != 0 || len(mcmContainerImage) != 0 {
			ginkgo.By("Updating MCM Deployemnt")
			gomega.Expect(c.prepareMcmDeployment(mcContainerImage, mcmContainerImage, false)).To(gomega.BeNil())
		} else {
			ginkgo.By("Cloning Machine-Controller-Manager github repo")
			gomega.Expect(helpers.CloneRepo("https://github.com/gardener/machine-controller-manager.git", mcmRepoPath)).
				To(gomega.BeNil())

			ginkgo.By("Scaledown existing machine controllers")
			gomega.Expect(c.scaleMcmDeployment(0)).To(gomega.BeNil())

			c.runControllersLocally()
		}
	} else {
		//TODO : Scaledown the MCM deployment of the actual seed of the target cluster

		ginkgo.By("Cloning Machine-Controller-Manager github repo")
		gomega.Expect(helpers.CloneRepo("https://github.com/gardener/machine-controller-manager.git", mcmRepoPath)).
			To(gomega.BeNil())

		//create the custom resources in the control cluster using yaml files
		//available in kubernetes/crds directory of machine-controller-manager repo
		//resources to be applied are machineclass, machines, machinesets and machinedeployment
		ginkgo.By("Applying kubernetes/crds into control cluster")
		gomega.Expect(c.ControlCluster.
			ApplyFiles(
				filepath.Join(mcmRepoPath, "kubernetes/crds"), controlClusterNamespace)).To(gomega.BeNil())

		if len(mcContainerImage) != 0 || len(mcmContainerImage) != 0 {
			ginkgo.By("Creating MCM Deployemnt")
			gomega.Expect(c.prepareMcmDeployment(mcContainerImage, mcmContainerImage, true)).To(gomega.BeNil())
		} else {
			c.runControllersLocally()
		}
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

	ginkgo.By("Determining target cluster name")
	clusterName, err := c.TargetCluster.ClusterName()
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	ginkgo.By("Looking for secrets refered in machineclass in the control cluster")
	secretData, err := c.ControlCluster.
		GetSecretData(
			machineClass.Name,
			machineClass.SecretRef,
			machineClass.CredentialsSecretRef,
		)
	gomega.Expect(err).
		NotTo(
			gomega.HaveOccurred(),
		)

	ginkgo.By(
		"Initializing orphan resource tracker",
	)
	err = c.resourcesTracker.InitializeResourcesTracker(machineClass, secretData, clusterName)

	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	log.Println("orphan resource tracker initialized")

}

// BeforeEachCheck checks if all the nodes are ready and the controllers are runnings
func (c *IntegrationTestFramework) BeforeEachCheck() {
	ginkgo.BeforeEach(func() {
		if len(os.Getenv("MC_CONTAINER_IMAGE")) == 0 && len(os.Getenv("MCM_CONTAINER_IMAGE")) == 0 {
			ginkgo.By("Checking machineController process is running")
			gomega.Expect(mcsession.ExitCode()).Should(gomega.Equal(-1))
			ginkgo.By("Checking machineControllerManager process is running")
			gomega.Expect(mcmsession.ExitCode()).Should(gomega.Equal(-1))
		}
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
				// Probe nodes currently available in target cluster
				initialNodes = c.TargetCluster.GetNumberOfNodes()
				ginkgo.By("Checking for errors")
				gomega.Expect(c.ControlCluster.CreateMachine(controlClusterNamespace)).To(gomega.BeNil())

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
								Delete(ctx, "test-machine", metav1.DeleteOptions{})).
							Should(gomega.BeNil(), "No Errors while deleting machine")

						ginkgo.By("Waiting until test-machine machine object is deleted")
						gomega.Eventually(
							c.ControlCluster.IsTestMachineDeleted,
							c.timeout,
							c.pollingInterval).
							Should(gomega.BeTrue())

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
		ginkgo.Context("creation with replicas=3", func() {
			ginkgo.It("should not lead to errors and add 3 more nodes to target cluster", func() {
				//probe initialnodes before continuing
				initialNodes = c.TargetCluster.GetNumberOfNodes()

				ginkgo.By("Checking for errors")
				gomega.Expect(c.ControlCluster.CreateMachineDeployment(controlClusterNamespace)).To(gomega.BeNil())

				ginkgo.By("Waiting until number of ready nodes are 3 more than initial")
				gomega.Eventually(
					c.TargetCluster.GetNumberOfNodes,
					c.timeout, c.pollingInterval).
					Should(gomega.BeNumerically("==", initialNodes+3))
				gomega.Eventually(
					c.TargetCluster.GetNumberOfReadyNodes,
					c.timeout, c.pollingInterval).
					Should(gomega.BeNumerically("==", initialNodes+3))
			})
		})
		ginkgo.Context("scale-up with replicas=6", func() {
			ginkgo.It("should not lead to errors and add futher 3 nodes to target cluster", func() {
				retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
					machineDployment, _ := c.ControlCluster.McmClient.
						MachineV1alpha1().
						MachineDeployments(controlClusterNamespace).
						Get(ctx, "test-machine-deployment", metav1.GetOptions{})
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
						Get(ctx, "test-machine-deployment", metav1.GetOptions{})
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
				if mcsession == nil {
					// controllers running in pod
					// Create log file from container log
					mcmOutputFile, err := rotateLogFile(mcmLogFile)
					gomega.Expect(err).NotTo(gomega.HaveOccurred())
					mcOutputFile, err := rotateLogFile(mcLogFile)
					gomega.Expect(err).NotTo(gomega.HaveOccurred())
					ginkgo.By("Reading container log is leading to no errors")
					podList, err := c.ControlCluster.Clientset.
						CoreV1().
						Pods(controlClusterNamespace).
						List(
							ctx,
							metav1.ListOptions{
								LabelSelector: "role=machine-controller-manager",
							},
						)
					gomega.Expect(err).NotTo(gomega.HaveOccurred())

					mcmPod := podList.Items[0]
					providerSpecificRegexp, _ := regexp.Compile(mcContainerPrefix)
					containers := mcmPod.Spec.Containers

					for i := range containers {
						if providerSpecificRegexp.Match([]byte(containers[i].Image)) {
							readCloser, err := c.ControlCluster.Clientset.CoreV1().
								Pods(controlClusterNamespace).
								GetLogs(
									mcmPod.Name,
									&coreV1.PodLogOptions{
										Container: containers[i].Name,
									},
								).Stream(ctx)
							gomega.Expect(err).NotTo(gomega.HaveOccurred())
							io.Copy(mcOutputFile, readCloser)
							gomega.Expect(err).NotTo(gomega.HaveOccurred())
						} else {
							readCloser, err := c.ControlCluster.Clientset.CoreV1().
								Pods(controlClusterNamespace).
								GetLogs(mcmPod.Name, &coreV1.PodLogOptions{
									Container: containers[i].Name,
								}).Stream(ctx)
							gomega.Expect(err).NotTo(gomega.HaveOccurred())
							io.Copy(mcmOutputFile, readCloser)
							gomega.Expect(err).NotTo(gomega.HaveOccurred())
						}
					}
				}

				ginkgo.By("Searching for Froze in mcm log file")
				frozeRegexp, _ := regexp.Compile(` Froze MachineSet`)
				gomega.Eventually(func() bool {
					data, _ := ioutil.ReadFile(mcmLogFile)
					return frozeRegexp.Match(data)
				}, c.timeout, c.pollingInterval).Should(gomega.BeTrue())

				ginkgo.By("Searching Unfroze in mcm log file")
				unfrozeRegexp, _ := regexp.Compile(` Unfroze MachineSet`)
				gomega.Eventually(func() bool {
					data, _ := ioutil.ReadFile(mcmLogFile)
					return unfrozeRegexp.Match(data)
				}, c.timeout, c.pollingInterval).Should(gomega.BeTrue())
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
						Get(ctx, "test-machine-deployment", metav1.GetOptions{})
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
						Get(ctx, "test-machine-deployment", metav1.GetOptions{})
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
						Get(ctx, "test-machine-deployment", metav1.GetOptions{})
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
						Get(ctx, "test-machine-deployment", metav1.GetOptions{})
					if err == nil {
						ginkgo.By("Checking for errors")
						gomega.Expect(
							c.ControlCluster.McmClient.
								MachineV1alpha1().
								MachineDeployments(controlClusterNamespace).
								Delete(
									ctx,
									"test-machine-deployment",
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
	if len(os.Getenv("MC_CONTAINER_IMAGE")) == 0 && len(os.Getenv("MCM_CONTAINER_IMAGE")) == 0 {
		for i := 0; i < 5; i++ {
			if mcsession.ExitCode() != -1 {
				ginkgo.By("Restarting Machine Controller ")
				outputFile, err := rotateLogFile(mcLogFile)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gexec.Start(mcsession.Command, outputFile, outputFile)
				break
			}
			time.Sleep(2 * time.Second)
		}
		for i := 0; i < 5; i++ {
			if mcmsession.ExitCode() != -1 {
				ginkgo.By("Restarting Machine Controller Manager")
				outputFile, err := rotateLogFile(mcmLogFile)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gexec.Start(mcmsession.Command, outputFile, outputFile)
				break
			}
			time.Sleep(2 * time.Second)
		}
	}

	if c.ControlCluster.McmClient != nil {
		timeout := int64(900)
		// Check and delete machinedeployment resource
		_, err := c.ControlCluster.McmClient.
			MachineV1alpha1().
			MachineDeployments(controlClusterNamespace).
			Get(ctx, "test-machine-deployment", metav1.GetOptions{})
		if err == nil {
			log.Println("deleting test-machine-deployment")
			watchMachinesDepl, _ := c.ControlCluster.McmClient.
				MachineV1alpha1().
				MachineDeployments(controlClusterNamespace).
				Watch(ctx, metav1.ListOptions{TimeoutSeconds: &timeout}) //ResourceVersion: machineDeploymentObj.ResourceVersion
			for event := range watchMachinesDepl.ResultChan() {
				c.ControlCluster.McmClient.
					MachineV1alpha1().
					MachineDeployments(controlClusterNamespace).
					Delete(ctx, "test-machine-deployment", metav1.DeleteOptions{})
				if event.Type == watch.Deleted {
					watchMachinesDepl.Stop()
					log.Println("machinedeployment deleted")
				}
			}
		} else {
			log.Println(err.Error())
		}
		// Check and delete machine resource
		_, err = c.ControlCluster.McmClient.
			MachineV1alpha1().
			Machines(controlClusterNamespace).
			Get(ctx, "test-machine", metav1.GetOptions{})
		if err == nil {
			log.Println("deleting test-machine")
			watchMachines, _ := c.ControlCluster.McmClient.
				MachineV1alpha1().
				Machines(controlClusterNamespace).
				Watch(ctx, metav1.ListOptions{TimeoutSeconds: &timeout}) //ResourceVersion: machineObj.ResourceVersion
			for event := range watchMachines.ResultChan() {
				c.ControlCluster.McmClient.
					MachineV1alpha1().
					Machines(controlClusterNamespace).
					Delete(ctx, "test-machine", metav1.DeleteOptions{})
				if event.Type == watch.Deleted {
					watchMachines.Stop()
					log.Println("machine deleted")
				}
			}
		} else {
			log.Println(err.Error())
		}

		for _, machineClassName := range testMachineClassResources {
			// Check and delete machine class resource
			_, err = c.ControlCluster.McmClient.
				MachineV1alpha1().
				MachineClasses(controlClusterNamespace).
				Get(ctx, machineClassName, metav1.GetOptions{})
			if err == nil {
				log.Printf("deleting %s machineclass", machineClassName)
				watchMachineClass, _ := c.ControlCluster.McmClient.
					MachineV1alpha1().
					MachineClasses(controlClusterNamespace).
					Watch(ctx, metav1.ListOptions{TimeoutSeconds: &timeout})
				for event := range watchMachineClass.ResultChan() {
					c.ControlCluster.McmClient.
						MachineV1alpha1().
						MachineClasses(controlClusterNamespace).
						Delete(ctx, machineClassName, metav1.DeleteOptions{})
					if event.Type == watch.Deleted {
						watchMachineClass.Stop()
						log.Println("machineclass deleted")
					}
				}
			} else {
				log.Println(err.Error())
			}
		}

	}
	if c.ControlCluster.IsSeed(c.TargetCluster) {
		// scale back up the MCM deployment to 1 in the Control Cluster
		// This is needed when IT suite runs locally against a Control & Target cluster

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

		if len(os.Getenv("MC_CONTAINER_IMAGE")) != 0 || len(os.Getenv("MCM_CONTAINER_IMAGE")) != 0 {
			log.Println("Deleting Clusterroles")

			if err := c.ControlCluster.RbacClient.ClusterRoles().Delete(
				ctx,
				"machine-controller-manager-control",
				metav1.DeleteOptions{},
			); err != nil {
				log.Printf("Error occured while deleting clusterrole. %s", err.Error())
			}

			if err := c.ControlCluster.RbacClient.ClusterRoleBindings().Delete(
				ctx,
				"machine-controller-manager-control",
				metav1.DeleteOptions{},
			); err != nil {
				log.Printf("Error occured while deleting clusterrolebinding . %s", err.Error())
			}

			if err := c.TargetCluster.RbacClient.ClusterRoles().Delete(
				ctx,
				"machine-controller-manager-target",
				metav1.DeleteOptions{},
			); err != nil {
				log.Printf("Error occured while deleting clusterrole . %s", err.Error())
			}

			if err := c.TargetCluster.RbacClient.ClusterRoleBindings().Delete(
				ctx,
				"machine-controller-manager-target",
				metav1.DeleteOptions{},
			); err != nil {
				log.Printf("Error occured while deleting clusterrolebinding . %s", err.Error())
			}

			log.Println("Deleting MCM deployment")

			c.ControlCluster.Clientset.
				CoreV1().
				Secrets(controlClusterNamespace).
				Delete(ctx, "machine-controller-manager", metav1.DeleteOptions{})
			c.ControlCluster.Clientset.
				AppsV1().
				Deployments(controlClusterNamespace).
				Delete(
					ctx,
					machineControllerManagerDeploymentName,
					metav1.DeleteOptions{},
				)
		}
	}

}
