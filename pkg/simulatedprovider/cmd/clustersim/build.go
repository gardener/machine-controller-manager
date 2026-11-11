package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

type BuildFlags struct {
	SourceMCM string
	SourceCA  string
}

var buildFlags BuildFlags

const (
	mcmCmdFile      = "cmd/machine-controller-manager/controller_manager.go"
	providerCmdFile = "cmd/machine-controller/main.go"
	caCmdFile       = "main.go"
)

var buildCmd = &cobra.Command{
	Use:                   "build {--source-mcm ...} [--source-ca ...]",
	Short:                 "Builds the specified components (machine-controller-manager, simulated provider, cluster-autoscaler).",
	Example:               `clustersim build --source-mcm ../../ --source-ca $HOME/go/src/github.com/gardener/autoscaler/cluster-autoscaler`,
	DisableFlagsInUseLine: true,
	PreRunE: func(cmd *cobra.Command, _ []string) error {
		if buildFlags.SourceMCM == "" && buildFlags.SourceCA == "" {
			return fmt.Errorf("source directories not passed")
		}

		if !strings.HasSuffix(buildFlags.SourceCA, "cluster-autoscaler") {
			buildFlags.SourceCA = filepath.Join(buildFlags.SourceCA, "cluster-autoscaler")
		}

		binDir := filepath.Join(getClustersDir(cmd), "bin")
		return os.MkdirAll(binDir, 0750)
	},
	RunE: func(cmd *cobra.Command, _ []string) error {
		cmd.SilenceUsage = true

		ctx := cmd.Context()
		binDir := filepath.Join(getClustersDir(cmd), "bin")
		var outputPath string

		if buildFlags.SourceMCM != "" {
			if _, err := os.Stat(buildFlags.SourceMCM); err != nil && errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("specified dir %q not found", buildFlags.SourceMCM)
			}

			outputPath = filepath.Join(binDir, binaryNameMCM)
			err := goBuild(ctx, buildFlags.SourceMCM, mcmCmdFile, outputPath)
			if err != nil {
				return err
			}

			providerPkgPath, err := getProviderRoot(buildFlags.SourceMCM)
			if err != nil {
				return err
			}

			outputPath = filepath.Join(binDir, binaryNameMC)
			err = goBuild(ctx, providerPkgPath, providerCmdFile, outputPath)
			if err != nil {
				return err
			}
		}

		if buildFlags.SourceCA != "" {
			if _, err := os.Stat(buildFlags.SourceCA); err != nil && errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("specified dir %q not found", buildFlags.SourceCA)
			}

			outputPath = filepath.Join(binDir, binaryNameCA)
			err := goBuild(ctx, buildFlags.SourceCA, caCmdFile, outputPath)
			if err != nil {
				return err
			}
		}

		return nil
	},
}

func init() {
	buildCmd.PersistentFlags().StringVar(
		&buildFlags.SourceMCM, "source-mcm", "",
		"machine-controller-manager source root directory path",
	)
	buildCmd.PersistentFlags().StringVar(
		&buildFlags.SourceCA, "source-ca", "",
		"autoscaler source root directory path",
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
