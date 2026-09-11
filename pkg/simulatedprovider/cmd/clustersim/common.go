package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/spf13/cobra"
	sigyaml "sigs.k8s.io/yaml"
)

const (
	componentMCM = "mcm"
	componentMC  = "mc"

	binaryNameMCM = "machine-controller-manager"
	binaryNameMC  = "machine-controller"

	fileNameKubeconfig        = "kubeconfig.yaml"
	fileNameMachineDeployment = "mcd.yaml"
	fileNameMachineClass      = "mcc.yaml"

	defaultMCMFlags = " --port=10258" +
		" --concurrent-syncs=30" +
		" --kube-api-qps=150" +
		" --kube-api-burst=200" +
		" --safety-up=2" +
		" --safety-down=1" +
		" --machine-safety-overshooting-period=300ms"

	// The timeouts are deliberately low values to allow for faster testing
	// of failures
	defaultMCFlags = " --port=10259" +
		" --kube-api-qps=100" +
		" --kube-api-burst=200" +
		" --machine-creation-timeout=2m" +
		" --machine-drain-timeout=5m" +
		" --machine-health-timeout=1m" +
		" --machine-pv-detach-timeout=2m" +
		" --machine-pv-reattach-timeout=150s" +
		" --machine-safety-apiserver-statuscheck-timeout=30s" +
		" --machine-safety-apiserver-statuscheck-period=1m" +
		" --machine-safety-orphan-vms-period=3m"
)

func validateClusterDataPresent(clustersDir string, clusterNames []string) error {
	for _, clusterName := range clusterNames {
		clusterPath := filepath.Join(clustersDir, clusterName)
		mcdFilePath := filepath.Join(clusterPath, fileNameMachineDeployment)
		mccFilePath := filepath.Join(clusterPath, fileNameMachineClass)
		if _, err := os.Stat(mcdFilePath); err != nil && errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("required file %q not found", mcdFilePath)
		}
		if _, err := os.Stat(mccFilePath); err != nil && errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("required file %q not found", mccFilePath)
		}
	}
	return nil
}

var allComponents = []string{componentMCM, componentMC}

func validateComponents(components []string) error {
	for _, component := range components {
		if found := slices.Contains(allComponents, component); !found {
			return fmt.Errorf(
				"passed invalid component %q, supported ones are %v",
				component, allComponents,
			)
		}
	}
	return nil
}

type StartConfig struct {
	ComponentPath  string
	ComponentFlags string
}

func getClusterNames(cmd *cobra.Command) (clusters []string) {
	clusters, _ = cmd.Flags().GetStringSlice("clusters")
	return
}

func getClustersDir(cmd *cobra.Command) (dir string) {
	dir, _ = cmd.Flags().GetString("dir")
	return
}

func prepareClusterDirs(clustersDir string, clusterNames []string) error {
	err := os.MkdirAll(clustersDir, 0750)
	if err != nil {
		return err
	}
	for _, clusterName := range clusterNames {
		clusterPath := filepath.Join(clustersDir, clusterName)
		err := os.MkdirAll(clusterPath, 0750)
		if err != nil {
			return err
		}
	}

	return nil
}

func generateStartConfigs(clusterPath, mcmFlags, mcFlags string, addNamespace bool) (err error) {
	binDir, err := filepath.Abs(filepath.Join(clusterPath, "..", "bin"))
	if err != nil {
		return
	}
	// The specified kubeconfig file will be created when the actual
	// cluster is being created.
	var kubeConfigPath string
	kubeConfigPath, err = filepath.Abs(filepath.Join(clusterPath, fileNameKubeconfig))
	if err != nil {
		return
	}
	additionalFlags := fmt.Sprintf(
		" --control-kubeconfig=%[1]v --target-kubeconfig=%[1]v",
		kubeConfigPath,
	)

	// This namespace will also be created when the cluster is being created.
	if addNamespace {
		var ns string
		ns, err = getClusterNamespace(clusterPath)
		if err != nil {
			return
		}
		additionalFlags += " --namespace=" + ns
	}

	additionalFlags += " --leader-elect=false --v=3"

	mcmStartConfig := StartConfig{
		ComponentPath:  filepath.Join(binDir, binaryNameMCM),
		ComponentFlags: mcmFlags + additionalFlags,
	}

	mcStartConfig := StartConfig{
		ComponentPath:  filepath.Join(binDir, binaryNameMC),
		ComponentFlags: mcFlags + additionalFlags,
	}

	mcmConfigPath := filepath.Join(clusterPath, getComponentConfigFileName(componentMCM))
	if err := saveConfigToFile(mcmConfigPath, mcmStartConfig); err != nil {
		return err
	}

	mcConfigPath := filepath.Join(clusterPath, getComponentConfigFileName(componentMC))
	if err := saveConfigToFile(mcConfigPath, mcStartConfig); err != nil {
		return err
	}
	return nil
}

func getComponentConfigFileName(component string) string {
	return component + "-start-config.json"
}

func getClusterNamespace(clusterPath string) (string, error) {
	mccFile := filepath.Join(clusterPath, fileNameMachineClass)
	mccList, err := readFileIntoObject[v1alpha1.MachineClassList](mccFile)
	if err != nil {
		return "", err
	}
	if len(mccList.Items) < 1 {
		return "default", nil
	}

	return mccList.Items[0].Namespace, nil
}

func saveConfigToFile(path string, config StartConfig) error {
	file, err := os.Create(filepath.Clean(path))
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	fmt.Printf("Saving start-config to %q\n", path)
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(config)
}

func readFileIntoObject[T any](path string) (obj T, err error) {
	var data []byte
	data, err = os.ReadFile(path)
	if err != nil {
		return
	}
	err = sigyaml.Unmarshal(data, &obj)
	return
}
