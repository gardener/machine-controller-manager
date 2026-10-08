package main

import (
	"github.com/spf13/cobra"
)

func main() {
	rootCmd, err := newRootCommand()
	cobra.CheckErr(err)

	rootCmd.AddCommand(buildCmd)
	rootCmd.AddCommand(copyShootCmd)
	rootCmd.AddCommand(genClusterCmd)
	rootCmd.AddCommand(setupCmd)
	rootCmd.AddCommand(startCmd)
	rootCmd.AddCommand(stopCmd)

	err = rootCmd.Execute()
	cobra.CheckErr(err)
}
