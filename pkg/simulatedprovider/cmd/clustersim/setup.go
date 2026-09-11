package main

import (
	"path/filepath"

	"github.com/spf13/cobra"
)

var setupCmd = &cobra.Command{
	Use:                   "setup {--clusters name}...",
	Short:                 "Generates the launch configuration for the cluster components.",
	Example:               `clustersim --clusters "test" setup`,
	DisableFlagsInUseLine: true,
	PreRunE: func(cmd *cobra.Command, _ []string) error {
		clustersDir := getClustersDir(cmd)
		clusterNames := getClusterNames(cmd)
		err := prepareClusterDirs(clustersDir, clusterNames)
		if err != nil {
			return err
		}
		return validateClusterDataPresent(clustersDir, clusterNames)
	},
	RunE: func(cmd *cobra.Command, args []string) (err error) {
		cmd.SilenceUsage = true

		clustersDir := getClustersDir(cmd)
		for _, clusterName := range getClusterNames(cmd) {
			clusterPath := filepath.Join(clustersDir, clusterName)
			err = generateStartConfigs(clusterPath, defaultMCMFlags, defaultMCFlags, true)
			if err != nil {
				return
			}
		}
		return
	},
}
