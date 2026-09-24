package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

type RootArgs struct {
	// ClusterNames is the comma separated list of cluster names which the
	// command is targeting, all subcommands re-use this list.
	// This is a required argument.
	ClusterNames []string

	// ClustersDir is an optional flag which denotes the directory where the
	// data for the specified clusters is present or should be stored if not
	// already present.
	// This defaults to "./gen" directory if nothing is specified.
	ClustersDir string
}

var rootFlags RootArgs

func newRootCommand() (*cobra.Command, error) {
	rootCmd := &cobra.Command{
		// Cluster name is the argument that needs to be passed to all subcommands
		// except 'build'. Hence it's added to the root command args.
		Use:                   "clustersim {--clusters name}... command [--dir /tmp]",
		Short:                 "Manages and sets up virtual cluster running simulated Machine Controller Manager.",
		Example:               `clustersim --clusters "test" setup`,
		Args:                  cobra.ExactArgs(1),
		DisableFlagsInUseLine: true,
		// The same validation happens to the subcommands since the argument is reused
		// for all of them.
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			if len(rootFlags.ClusterNames) == 0 && cmd.Name() != "build" {
				return fmt.Errorf("no cluster names passed")
			}
			return nil
		},
		// This `Run` block, even though it does nothing but silence the command usage,
		// is required so that we can have a `PreRun` validation.
		Run: func(cmd *cobra.Command, _ []string) {
			// any further errors are not related to this command's usage and can be silenced
			cmd.SilenceUsage = true
		},
	}

	rootCmd.PersistentFlags().StringSliceVar(
		&rootFlags.ClusterNames, "clusters", nil,
		"comma separated list of cluster(s) to target (required)",
	)

	rootCmd.PersistentFlags().StringVar(
		&rootFlags.ClustersDir, "dir", "./gen",
		"optional flag to specify the directory for clusters data",
	)

	return rootCmd, nil
}
