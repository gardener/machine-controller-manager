package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/gardener/machine-controller-manager/pkg/simulatedprovider/cluster"
	"github.com/spf13/cobra"
)

type StopFlags struct {
	Components []string
	// Stop all components and destroy all clusters
	KillAll bool
}

var stopFlags StopFlags

const dummyNamespace = "dummy-namespace"

var stopCmd = &cobra.Command{
	Use:                   "stop {--clusters name}... [--components mcm]... [--killall true|false]",
	Short:                 "Stops specified components using their pid and destroys the kwok cluster.",
	Example:               `clustersim --clusters "test" stop`,
	DisableFlagsInUseLine: true,
	// Not using PreRunE/RunE, since any error in the 'stop' command shouldn't be fatal.
	// It should just log the error and continue with the removal to atleast perform
	// whatever cleanup can be done.
	PreRun: func(cmd *cobra.Command, cmdArgs []string) {
		err := validateComponents(stopFlags.Components)
		if err != nil {
			logErr(err)
		}
	},
	Run: func(cmd *cobra.Command, cmdArgs []string) {
		cmd.SilenceUsage = true

		clustersDir := getClustersDir(cmd)
		clusterNames := getClusterNames(cmd)
		for _, name := range clusterNames {
			clusterPath := filepath.Join(clustersDir, name)

			for _, component := range stopFlags.Components {
				err := stopComponent(component, clusterPath)
				if err != nil {
					logErr(err)
				}
			}

			clusterEnv := cluster.New(name, dummyNamespace)

			if stopFlags.KillAll {
				err := clusterEnv.DeleteCluster()
				if err != nil {
					logErr(err)
				}
				fmt.Printf("Deleted cluster %q\n", name)
				_ = os.Remove(filepath.Join(clusterPath, fileNameKubeconfig))
			}
		}
	},
}

func init() {
	stopCmd.PersistentFlags().BoolVar(
		&stopFlags.KillAll, "killall", true,
		"stop all components and destroy all clusters",
	)

	stopCmd.PersistentFlags().StringSliceVarP(
		&stopFlags.Components, "components", "c", allComponents,
		"comma separated list of components to stop",
	)

	// If killall specified, override specified components with all
	if stopFlags.KillAll {
		stopFlags.Components = allComponents
	}
}

func stopComponent(component, clusterPath string) error {
	pidFile := filepath.Join(clusterPath, component+".pid")
	data, err := os.ReadFile(pidFile)
	if err != nil {
		return err
	}

	pidInt, err := strconv.Atoi(string(data))
	if err != nil {
		return err
	}

	proc, err := os.FindProcess(pidInt)
	if err != nil {
		return err
	}
	err = proc.Kill()
	if errors.Is(err, os.ErrProcessDone) {
		err = nil
	}
	fmt.Printf("Stopped %q with PID %d\n", component, pidInt)
	return os.Remove(pidFile)
}

func logErr(err error) {
	fmt.Fprintln(os.Stderr, "Error:", err)
}
