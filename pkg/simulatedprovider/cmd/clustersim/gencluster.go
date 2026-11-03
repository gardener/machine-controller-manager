package main

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"path/filepath"
	"slices"

	"github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/gardener/machine-controller-manager/pkg/client/clientset/versioned/scheme"
	"github.com/spf13/cobra"
	sigyaml "sigs.k8s.io/yaml"
)

//go:embed templates/*.tmpl
//go:embed templates/*.json
var content embed.FS

const (
	mccTemplatePath         = "templates/mcc.tmpl"
	mcdTemplatePath         = "templates/mcd.tmpl"
	instanceDetailsFilePath = "templates/instances.json"
)

type GenClusterFlags struct {
	Zones     []string
	Instances []string
}

type TemplateParams struct {
	ClusterName string
	Zone        string
	InstanceDetails
}

type InstanceDetails struct {
	Arch     string `json:"architecture"`
	Cpu      string `json:"cpu"`
	Gpu      string `json:"gpu"`
	Mem      string `json:"memory"`
	Instance string `json:"name"`
}

var genClusterFlags GenClusterFlags

var genClusterCmd = &cobra.Command{
	Use:                   "gencluster {--clusters name} {--zones zone}... {--instances inst}...",
	Short:                 "Generates MachineClasses and MachineDeployments with the specified parameters.",
	DisableFlagsInUseLine: true,
	PreRunE: func(cmd *cobra.Command, _ []string) error {
		if genClusterFlags.Zones == nil || genClusterFlags.Instances == nil {
			return fmt.Errorf("required flags not passed")
		}

		return prepareClusterDirs(getClustersDir(cmd), getClusterNames(cmd))
	},
	RunE: func(cmd *cobra.Command, _ []string) error {
		cmd.SilenceUsage = true

		clustersDir := getClustersDir(cmd)
		scheme := scheme.Scheme
		instanceList, err := getInstanceDetails()
		if err != nil {
			return fmt.Errorf("could not parse instances list: %v", err)
		}

		for _, clusterName := range getClusterNames(cmd) {
			clusterPath := filepath.Join(clustersDir, clusterName)
			var (
				mccList     v1alpha1.MachineClassList
				mcdList     v1alpha1.MachineDeploymentList
				mccFilePath = filepath.Join(clusterPath, fileNameMachineClass)
				mcdFilePath = filepath.Join(clusterPath, fileNameMachineDeployment)
			)
			for _, instance := range genClusterFlags.Instances {
				for _, zone := range genClusterFlags.Zones {
					idx := slices.IndexFunc(instanceList, func(details InstanceDetails) bool {
						return details.Instance == instance
					})

					templateParams := TemplateParams{
						ClusterName:     clusterName,
						Zone:            zone,
						InstanceDetails: instanceList[idx],
					}

					mcc, err := generateObject[v1alpha1.MachineClass](templateParams, mccTemplatePath)
					if err != nil {
						return err
					}
					mccList.Items = append(mccList.Items, mcc)

					mcd, err := generateObject[v1alpha1.MachineDeployment](templateParams, mcdTemplatePath)
					if err != nil {
						return err
					}
					mcdList.Items = append(mcdList.Items, mcd)
				}
			}

			err = saveObject(&mccList, mccFilePath, scheme)
			if err != nil {
				return err
			}

			err = saveObject(&mcdList, mcdFilePath, scheme)
			if err != nil {
				return err
			}

			err = generateStartConfigs(clusterPath, defaultMCMFlags, defaultMCFlags, true)
			if err != nil {
				return err
			}
		}
		return nil
	},
}

func init() {
	genClusterCmd.PersistentFlags().StringSliceVarP(
		&genClusterFlags.Zones, "zones", "z", nil,
		"comma separated list of zones to use (required)",
	)
	_ = genClusterCmd.MarkFlagRequired("zones")

	genClusterCmd.PersistentFlags().StringSliceVarP(
		&genClusterFlags.Instances, "instances", "i", nil,
		"comma separated list of instances to use (required)",
	)
	_ = genClusterCmd.MarkFlagRequired("instances")
}

func getInstanceDetails() (details []InstanceDetails, err error) {
	var data []byte
	data, err = content.ReadFile(instanceDetailsFilePath)
	if err != nil {
		err = fmt.Errorf("cannot read %s from content FS: %w", instanceDetailsFilePath, err)
		return
	}
	err = json.Unmarshal(data, &details)
	return
}

func generateObject[T any](params TemplateParams, templatePath string) (obj T, err error) {
	data, err := content.ReadFile(templatePath)
	if err != nil {
		return
	}
	templateConfig, err := template.New(templatePath).Parse(string(data))
	if err != nil {
		return
	}

	var buf bytes.Buffer
	err = templateConfig.Execute(&buf, params)
	if err != nil {
		return
	}

	err = sigyaml.Unmarshal(buf.Bytes(), &obj)
	return
}
