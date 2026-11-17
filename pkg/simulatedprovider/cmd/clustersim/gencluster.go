package main

import (
	"bytes"
	"embed"
	"fmt"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/gardener/machine-controller-manager/pkg/client/clientset/versioned/scheme"
	"github.com/spf13/cobra"
	sigyaml "sigs.k8s.io/yaml"
)

//go:embed templates/*.tmpl
var content embed.FS

const (
	mccTemplatePath = "templates/mcc.tmpl"
	mcdTemplatePath = "templates/mcd.tmpl"
)

type GenClusterFlags struct {
	Zones     []string
	Instances []string
}

type TemplateParams struct {
	ClusterName string
	Region      string
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

var (
	genClusterFlags GenClusterFlags
	instanceList    []InstanceDetails
)

var genClusterCmd = &cobra.Command{
	Use:                   "gencluster {--clusters name} {--zones zone}... {--instance inst}...",
	Short:                 "Generates MachineClasses and MachineDeployments with the specified parameters.",
	Example:               `clustersim gencluster --clusters "test" --zones "eu-west1,eu-east2" --instance name=m5.xlarge;cpu=2;gpu=0;mem=8Gi;arch=amd64`,
	DisableFlagsInUseLine: true,
	PreRunE: func(cmd *cobra.Command, _ []string) (err error) {
		if genClusterFlags.Zones == nil || genClusterFlags.Instances == nil {
			return fmt.Errorf("required flags not passed")
		}

		for _, zone := range genClusterFlags.Zones {
			if strings.TrimSpace(zone) == "" {
				return fmt.Errorf("zone cannot be empty")
			}

			if strings.LastIndex(zone, "-") == -1 {
				return fmt.Errorf("invalid zone %q passed, expected format is region-zone e.g. euwest-1", zone)
			}
		}

		for _, instance := range genClusterFlags.Instances {
			if strings.TrimSpace(instance) == "" {
				return fmt.Errorf("instance cannot be empty")
			}
		}

		instanceList, err = parseInstanceList(genClusterFlags.Instances)
		if err != nil {
			return fmt.Errorf("could not parse instances list: %v", err)
		}

		return prepareClusterDirs(getClustersDir(cmd), getClusterNames(cmd))
	},
	RunE: func(cmd *cobra.Command, _ []string) (err error) {
		cmd.SilenceUsage = true

		clustersDir := getClustersDir(cmd)
		scheme := scheme.Scheme

		for _, clusterName := range getClusterNames(cmd) {
			clusterPath := filepath.Join(clustersDir, clusterName)
			var (
				mccList     v1alpha1.MachineClassList
				mcdList     v1alpha1.MachineDeploymentList
				mccFilePath = filepath.Join(clusterPath, fileNameMachineClass)
				mcdFilePath = filepath.Join(clusterPath, fileNameMachineDeployment)
			)
			for _, instance := range instanceList {
				for _, zone := range genClusterFlags.Zones {
					templateParams := TemplateParams{
						ClusterName:     clusterName,
						Region:          getRegionFromZone(zone),
						Zone:            zone,
						InstanceDetails: instance,
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
		&genClusterFlags.Instances, "instance", "i", nil,
		"comma separated list of instances to use (required)",
	)
	_ = genClusterCmd.MarkFlagRequired("instance")
}

// parses an instance list of the form:
//
//	name=m5.xlarge;cpu=2;gpu=0;mem=8Gi;arch=amd64
func parseInstanceList(instances []string) (details []InstanceDetails, err error) {
	details = make([]InstanceDetails, len(instances))
	for i, instance := range instances {
		for params := range strings.SplitSeq(instance, ";") {
			detail := strings.Split(params, "=")
			if len(detail) != 2 {
				err = fmt.Errorf("could not parse instance list")
				return
			}
			switch detail[0] {
			case "name":
				details[i].Instance = detail[1]
			case "arch":
				details[i].Arch = detail[1]
			case "cpu":
				details[i].Cpu = detail[1]
			case "gpu":
				details[i].Gpu = detail[1]
			case "mem":
				details[i].Mem = detail[1]
			default:
				return nil, fmt.Errorf("invalid instance key: %q", detail[0])
			}
		}

		if details[i].Instance == "" || details[i].Cpu == "" || details[i].Mem == "" {
			err = fmt.Errorf("could not parse instance: %+v", instance)
			return
		}

		if details[i].Gpu == "" {
			details[i].Gpu = "0"
		}
		if details[i].Arch == "" {
			details[i].Arch = "amd64"
		}

		fmt.Printf("Parsed instance: %+v\n", details[i])
	}
	return
}

func getRegionFromZone(zone string) string {
	// TODO: consider different CSP based zone formats
	// If support for other formats is added, the validation in `PreRunE`
	// has to be adapted accordingly.
	idx := strings.LastIndex(zone, "-")
	return zone[:idx]
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
