package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/gardener/machine-controller-manager/pkg/simulatedprovider/cluster"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/e2e-framework/klient"
)

type StartFlags struct {
	Components []string
}

var startFlags StartFlags

var startCmd = &cobra.Command{
	Use:                   "start {--clusters name}... [--components mcm]...",
	Short:                 "Creates kwok cluster and starts specified components using their launch config.",
	Example:               `clustersim --clusters "test" start`,
	DisableFlagsInUseLine: true,
	PreRunE: func(cmd *cobra.Command, _ []string) (err error) {
		err = validateComponents(startFlags.Components)
		if err != nil {
			return
		}
		// Check if the binaries, yamls and start-configs are present
		clustersDir := getClustersDir(cmd)
		clusterNames := getClusterNames(cmd)
		return requiredDataPresent(clustersDir, clusterNames, startFlags.Components)
	},
	RunE: func(cmd *cobra.Command, cmdArgs []string) (err error) {
		cmd.SilenceUsage = true

		var (
			startConfig StartConfig
			namespace   string
		)
		for cid, name := range getClusterNames(cmd) {
			clusterPath := filepath.Join(getClustersDir(cmd), name)
			namespace, err = getClusterNamespace(clusterPath)
			if err != nil {
				return
			}

			clusterEnv := cluster.New(name, namespace)
			err = clusterEnv.SetupCluster()
			if err != nil {
				return
			}

			linkedConfigPath := filepath.Join(clusterPath, fileNameKubeconfig)
			kubeConfig := clusterEnv.Cfg.KubeconfigFile()
			// Clean up any existing, stale symlink
			_ = os.Remove(linkedConfigPath)
			err = os.Symlink(kubeConfig, linkedConfigPath)
			if err != nil {
				return fmt.Errorf("could not link kubeconfig to %q: %v", linkedConfigPath, err)
			}

			fmt.Printf("Created cluster %q with kubeconfig at %q\n", name, linkedConfigPath)

			// Launch the specified components, record the pid of launched command
			for _, component := range startFlags.Components {
				startConfigFile := filepath.Join(
					clusterPath, getComponentConfigFileName(component),
				)
				startConfig, err = readFileIntoObject[StartConfig](startConfigFile)
				if err != nil {
					return
				}
				// When launching multiple clusters, to avoid re-using the same ports
				portInc := len(startFlags.Components) * cid
				err = startComponent(
					clusterEnv.Ctx, startConfig, component, clusterPath, portInc,
				)
				if err != nil {
					return
				}
			}

			err = deployResources(clusterEnv.Ctx, clusterEnv.Cfg.Client(), clusterPath)
			if err != nil {
				return
			}
		}
		return nil
	},
}

func init() {
	startCmd.PersistentFlags().StringSliceVarP(
		&startFlags.Components, "components", "c", allComponents,
		"comma separated list of components to run",
	)
}

func requiredDataPresent(clustersDir string, clusterNames, components []string) (err error) {
	err = validateClusterDataPresent(clustersDir, clusterNames)
	if err != nil {
		return
	}
	binDir := filepath.Join(clustersDir, "bin")
	err = validateBinariesPresent(binDir, components)
	if err != nil {
		return
	}
	return validateConfigsPresent(clustersDir, clusterNames, components)
}

func validateBinariesPresent(binDir string, components []string) error {
	if slices.Contains(components, componentMCM) {
		componentBinPath := filepath.Join(binDir, binaryNameMCM)
		if _, err := os.Stat(componentBinPath); err != nil && errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("required file %q not found", componentBinPath)
		}
	}
	if slices.Contains(components, componentMC) {
		componentBinPath := filepath.Join(binDir, binaryNameMC)
		if _, err := os.Stat(componentBinPath); err != nil && errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("required file %q not found", componentBinPath)
		}
	}
	return nil
}

func validateConfigsPresent(clustersDir string, clusterNames, components []string) error {
	for _, clusterName := range clusterNames {
		clusterPath := filepath.Join(clustersDir, clusterName)
		for _, component := range components {
			configPath := filepath.Join(clusterPath, getComponentConfigFileName(component))
			if _, err := os.Stat(configPath); err != nil && errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("required file %q not found", configPath)
			}
		}
	}
	return nil
}

func startComponent(
	ctx context.Context,
	startConfig StartConfig,
	component string,
	clusterPath string,
	inc int,
) (err error) {
	flags := strings.Split(startConfig.ComponentFlags, " ")
	modifyPortFlag(flags, inc)

	cmd := exec.CommandContext(ctx, startConfig.ComponentPath, flags...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true, // Start a new process group
	}

	// Create the file if it doesn't exist, and clear the existing content (no append)
	logFilePath := filepath.Join(clusterPath, component+".log")
	logFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0666)
	if err != nil {
		return
	}
	defer logFile.Close()

	cmd.Stdout = logFile
	cmd.Stderr = logFile

	err = cmd.Start()
	if err != nil {
		return
	}

	pid := cmd.Process.Pid
	pidFilePath := filepath.Join(clusterPath, component+".pid")
	err = os.WriteFile(pidFilePath, []byte(strconv.Itoa(pid)), 0666)
	if err != nil {
		return fmt.Errorf("cannot write pid to pidPath %q: %w", pidFilePath, err)
	}
	fmt.Printf("Started %q with PID %d\n", component, pid)
	return nil
}

// changes the default port to (port + inc)
func modifyPortFlag(flags []string, inc int) {
	for i, flag := range flags {
		if strings.HasPrefix(flag, "--port") {
			portValue, err := strconv.Atoi(strings.Split(flag, "=")[1])
			if err != nil {
				continue
			}
			flags[i] = "--port=" + strconv.Itoa(portValue+inc)
		}
	}
}

func deployResources(ctx context.Context, client klient.Client, clusterPath string) (err error) {
	var (
		mccList     v1alpha1.MachineClassList
		mcdList     v1alpha1.MachineDeploymentList
		mccFilePath = filepath.Join(clusterPath, fileNameMachineClass)
		mcdFilePath = filepath.Join(clusterPath, fileNameMachineDeployment)
	)
	mccList, err = readFileIntoObject[v1alpha1.MachineClassList](mccFilePath)
	if err != nil {
		return
	}
	mcdList, err = readFileIntoObject[v1alpha1.MachineDeploymentList](mcdFilePath)
	if err != nil {
		return
	}
	fmt.Println("Deploying MachineClasses")
	for _, mcc := range mccList.Items {
		mcc.ResourceVersion = ""
		err = client.Resources().Create(ctx, &mcc)
		if err != nil && !apierrors.IsAlreadyExists(err) {
			return
		}
		err = ensureSecretsCreation(ctx, client, mcc)
		if err != nil {
			return fmt.Errorf("secret not present for mcc %q: %v", mcc.Name, err)
		}
	}
	fmt.Println("Deploying MachineDeployments")
	for _, mcd := range mcdList.Items {
		mcd.ResourceVersion = ""
		err = client.Resources().Create(ctx, &mcd)
		if err != nil && !apierrors.IsAlreadyExists(err) {
			return
		}
	}
	return
}

func ensureSecretsCreation(
	ctx context.Context,
	client klient.Client,
	mcc v1alpha1.MachineClass,
) error {
	return wait.PollUntilContextTimeout(ctx, 50*time.Millisecond, 1*time.Second, false,
		func(ctx context.Context) (bool, error) {
			var secret corev1.Secret
			if mcc.SecretRef != nil {
				err := client.Resources().Get(
					ctx, mcc.SecretRef.Name, mcc.SecretRef.Namespace, &secret,
				)
				if err != nil {
					return false, err
				}
			}
			if mcc.CredentialsSecretRef != nil {
				err := client.Resources().Get(
					ctx, mcc.CredentialsSecretRef.Name, mcc.CredentialsSecretRef.Namespace, &secret,
				)
				if err != nil {
					return false, err
				}
			}
			return true, nil
		})
}
