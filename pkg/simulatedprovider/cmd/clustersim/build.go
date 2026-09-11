package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"

	"github.com/spf13/cobra"
)

type BuildFlags struct {
	SourceDir  string
	Components []string
}

var buildFlags BuildFlags

const (
	mcmCmdFile      = "cmd/machine-controller-manager/controller_manager.go"
	providerCmdFile = "cmd/machine-controller/main.go"
)

var buildCmd = &cobra.Command{
	Use:                   "build {--source-dir ..} [--components mcm]...",
	Short:                 "Builds the specified components (machine-controller-manager, simulated provider).",
	Example:               `clustersim build --source-dir ../../../../ --components "mcm,mc"`,
	DisableFlagsInUseLine: true,
	PreRunE: func(cmd *cobra.Command, _ []string) error {
		if buildFlags.SourceDir == "" {
			return fmt.Errorf("required flag --source-dir not passed")
		}

		if err := validateComponents(buildFlags.Components); err != nil {
			return err
		}

		binDir := filepath.Join(getClustersDir(cmd), "bin")
		return os.MkdirAll(binDir, 0750)
	},
	RunE: func(cmd *cobra.Command, _ []string) error {
		cmd.SilenceUsage = true

		ctx := cmd.Context()
		binDir := filepath.Join(getClustersDir(cmd), "bin")

		if slices.Contains(buildFlags.Components, componentMCM) {
			outputPath := filepath.Join(binDir, binaryNameMCM)
			err := goBuild(ctx, buildFlags.SourceDir, mcmCmdFile, outputPath)
			if err != nil {
				return err
			}
		}

		if slices.Contains(buildFlags.Components, componentMC) {
			providerPkgPath, err := getProviderRoot(buildFlags.SourceDir)
			if err != nil {
				return err
			}

			outputPath := filepath.Join(binDir, binaryNameMC)
			err = goBuild(ctx, providerPkgPath, providerCmdFile, outputPath)
			if err != nil {
				return err
			}
		}
		return nil
	},
}

func init() {
	buildCmd.PersistentFlags().StringSliceVarP(
		&buildFlags.Components, "components", "c", allComponents,
		"comma separated list of components to build",
	)
	buildCmd.PersistentFlags().StringVar(
		&buildFlags.SourceDir, "source-dir", "",
		"machine-controller-manager source root directory path (required)",
	)
}

// goBuild invokes `go build -C dir -o binPath -v -buildvcs=true file`
func goBuild(ctx context.Context, dir, file, binPath string) error {
	binPathAbs, err := filepath.Abs(binPath)
	if err != nil {
		return fmt.Errorf("cannot get absolute path for path %q: %w", binPath, err)
	}

	fmt.Printf("Building %q\n", binPath)
	cmd := exec.CommandContext(
		ctx, "go", "build", "-C", dir, "-o", binPathAbs, "-v", "-buildvcs=true", file,
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	if err != nil {
		return err
	}

	if finfo, err := os.Stat(binPath); err != nil && finfo.IsDir() {
		return fmt.Errorf("did not find binary installed at expected path %q", binPath)
	}

	return nil
}

func getProviderRoot(sourceDir string) (string, error) {
	return filepath.Abs(filepath.Join(sourceDir, "pkg", "simulatedprovider"))
}
