// Copyright 2019 Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package framework

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/retry"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

// ShootSeedNamespace gets the shoot namespace in the seed
func (f *ShootFramework) ShootSeedNamespace() string {
	return computeTechnicalID(f.Project.Name, f.Shoot)
}

// ShootKubeconfigSecretName gets the name of the secret with the kubeconfig of the shoot
func (f *ShootFramework) ShootKubeconfigSecretName() string {
	return fmt.Sprintf("%s.kubeconfig", f.Shoot.GetName())
}

// GetLokiLogs gets logs for <podName> from the loki instance in <lokiNamespace>
func (f *ShootFramework) GetLokiLogs(ctx context.Context, lokiNamespace, podName string, client kubernetes.Interface) (*SearchResponse, error) {
	lokiLabels := labels.SelectorFromSet(labels.Set(map[string]string{
		"app":  lokiLogging,
		"role": "logging",
	}))

	query := fmt.Sprintf("{app=\"%s\"}", podName)

	command := fmt.Sprintf("wget 'http://localhost:%d/loki/api/v1/query_range' -O- --post-data='query=%s'", lokiPort, query)

	var reader io.Reader
	err := retry.Until(ctx, defaultPollInterval, func(ctx context.Context) (bool, error) {
		var err error
		reader, err = PodExecByLabel(ctx, lokiLabels, lokiLogging, command, lokiNamespace, client)

		if err != nil {
			f.Logger.Warn(err)
			return retry.MinorError(err)
		}
		return retry.Ok()
	})
	if err != nil {
		return nil, err
	}

	search := &SearchResponse{}

	if err = json.NewDecoder(reader).Decode(search); err != nil {
		return nil, err
	}

	return search, nil
}

// DumpState dumps the state of a shoot
// The state includes all k8s components running in the shoot itself as well as the controlplane
func (f *ShootFramework) DumpState(ctx context.Context) {
	if f.DisableStateDump {
		return
	}

	if f.Shoot != nil {
		if err := PrettyPrintObject(f.Shoot); err != nil {
			f.Logger.Fatalf("Cannot decode shoot %s: %s", f.Shoot.GetName(), err)
		}

		isRunning, err := f.IsAPIServerRunning(ctx)
		if f.ShootClient != nil && isRunning && err == nil {
			ctxIdentifier := fmt.Sprintf("[SHOOT %s]", f.Shoot.Name)
			f.Logger.Info(ctxIdentifier)
			if err := f.DumpDefaultResourcesInAllNamespaces(ctx, ctxIdentifier, f.ShootClient); err != nil {
				f.Logger.Errorf("unable to dump resources from all namespaces in shoot %s: %s", f.Shoot.Name, err.Error())
			}
			if err := f.dumpNodes(ctx, ctxIdentifier, f.ShootClient); err != nil {
				f.Logger.Errorf("unable to dump information of nodes from shoot %s: %s", f.Shoot.Name, err.Error())
			}
		} else {
			errMsg := ""
			if err != nil {
				errMsg = ": " + err.Error()
			}
			f.Logger.Errorf("unable to dump resources from shoot %s: API server is currently not running%s", f.Shoot.Name, errMsg)
		}
	}

	// dump controlplane in the shoot namespace
	if f.Seed != nil && f.SeedClient != nil {
		if err := f.dumpControlplaneInSeed(ctx, f.Seed, f.ShootSeedNamespace()); err != nil {
			f.Logger.Errorf("unable to dump controlplane of %s in seed %s: %v", f.Shoot.Name, f.Seed.Name, err)
		}
	}

	ctxIdentifier := "[GARDENER]"
	f.Logger.Info(ctxIdentifier)
	if f.Shoot != nil {
		project, err := f.GetShootProject(ctx, f.Shoot.GetNamespace())
		if err != nil {
			f.Logger.Errorf("unable to get project namespace of shoot %s: %s", f.Shoot.GetNamespace(), err.Error())
			return
		}

		// dump seed status if seed is available
		if f.Shoot.Spec.SeedName != nil {
			seed := &gardencorev1beta1.Seed{}
			if err := f.GardenClient.DirectClient().Get(ctx, client.ObjectKey{Name: *f.Shoot.Spec.SeedName}, seed); err != nil {
				f.Logger.Errorf("unable to get seed %s: %s", *f.Shoot.Spec.SeedName, err.Error())
				return
			}
			f.Logger.Infof("%s [SEED]", ctxIdentifier)
			f.dumpSeed(seed)
		}

		err = f.dumpEventsInNamespace(ctx, ctxIdentifier, f.GardenClient, *project.Spec.Namespace, func(event corev1.Event) bool {
			return event.InvolvedObject.Name == f.Shoot.Name
		})
		if err != nil {
			f.Logger.Errorf("unable to dump Events from project namespace %s in gardener: %s", *project.Spec.Namespace, err.Error())
		}
	}
}

// CreateShootTestArtifacts creates a shoot object from the given path and sets common attributes (test-individual settings like workers have to be handled by each test).
func CreateShootTestArtifacts(cfg *ShootCreationConfig, projectNamespace string, clearDNS bool, clearExtensions bool) (string, *gardencorev1beta1.Shoot, error) {
	shoot := &gardencorev1beta1.Shoot{}
	if cfg.shootYamlPath != "" {
		if err := ReadObject(cfg.shootYamlPath, shoot); err != nil {
			return "", nil, err
		}
	}

	if err := setShootMetadata(shoot, cfg, projectNamespace); err != nil {
		return "", nil, err
	}

	setShootGeneralSettings(shoot, cfg, clearExtensions)

	setShootNetworkingSettings(shoot, cfg, clearDNS)

	return shoot.Name, shoot, nil
}

func parseAnnotationCfg(cfg string) (map[string]string, error) {
	if !StringSet(cfg) {
		return nil, nil
	}
	result := make(map[string]string)
	annotations := strings.Split(cfg, ",")
	for _, annotation := range annotations {
		annotation = strings.TrimSpace(annotation)
		if !StringSet(annotation) {
			continue
		}
		keyValue := strings.Split(annotation, "=")
		if len(keyValue) != 2 {
			return nil, fmt.Errorf("annotation %s could not be parsed into key and value", annotation)
		}
		result[keyValue[0]] = keyValue[1]
	}

	return result, nil
}

// setShootMetadata sets the Shoot's metadata from the given config and project namespace
func setShootMetadata(shoot *gardencorev1beta1.Shoot, cfg *ShootCreationConfig, projectNamespace string) error {
	if StringSet(cfg.testShootName) {
		shoot.Name = cfg.testShootName
	} else {
		integrationTestName, err := generateRandomShootName(cfg.testShootPrefix, 8)
		if err != nil {
			return err
		}
		shoot.Name = integrationTestName
	}

	if StringSet(projectNamespace) {
		shoot.Namespace = projectNamespace
	}

	if err := setConfiguredShootAnnotations(shoot, cfg); err != nil {
		return err
	}

	metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, v1beta1constants.AnnotationShootIgnoreAlerts, "true")

	return nil
}

// setConfiguredShootAnnotations sets annotations from the given config on the given shoot
func setConfiguredShootAnnotations(shoot *gardencorev1beta1.Shoot, cfg *ShootCreationConfig) error {
	annotations, err := parseAnnotationCfg(cfg.shootAnnotations)
	if err != nil {
		return err
	}
	for k, v := range annotations {
		metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, k, v)
	}
	return nil
}

// setShootGeneralSettings sets the Shoot's general settings from the given config
func setShootGeneralSettings(shoot *gardencorev1beta1.Shoot, cfg *ShootCreationConfig, clearExtensions bool) {
	if StringSet(cfg.shootRegion) {
		shoot.Spec.Region = cfg.shootRegion
	}

	if StringSet(cfg.cloudProfile) {
		shoot.Spec.CloudProfileName = cfg.cloudProfile
	}

	if StringSet(cfg.secretBinding) {
		shoot.Spec.SecretBindingName = cfg.secretBinding
	}

	if StringSet(cfg.shootProviderType) {
		shoot.Spec.Provider.Type = cfg.shootProviderType
	}

	if StringSet(cfg.shootK8sVersion) {
		shoot.Spec.Kubernetes.Version = cfg.shootK8sVersion
	}

	if StringSet(cfg.seedName) {
		shoot.Spec.SeedName = &cfg.seedName
	}

	if cfg.startHibernated {
		if shoot.Spec.Hibernation == nil {
			shoot.Spec.Hibernation = &gardencorev1beta1.Hibernation{}
		}
		shoot.Spec.Hibernation.Enabled = &cfg.startHibernated
	}

	// allow privileged containers defaults to true
	if cfg.allowPrivilegedContainers != nil {
		shoot.Spec.Kubernetes.AllowPrivilegedContainers = cfg.allowPrivilegedContainers
	}

	if clearExtensions {
		shoot.Spec.Extensions = nil
	}
}

// setShootNetworkingSettings sets the Shoot's networking settings from the given config
func setShootNetworkingSettings(shoot *gardencorev1beta1.Shoot, cfg *ShootCreationConfig, clearDNS bool) {
	if StringSet(cfg.externalDomain) {
		shoot.Spec.DNS = &gardencorev1beta1.DNS{Domain: &cfg.externalDomain}
		clearDNS = false
	}
	if StringSet(cfg.networkingPods) {
		shoot.Spec.Networking.Pods = &cfg.networkingPods
	}

	if StringSet(cfg.networkingServices) {
		shoot.Spec.Networking.Services = &cfg.networkingServices
	}

	if StringSet(cfg.networkingNodes) {
		shoot.Spec.Networking.Nodes = &cfg.networkingNodes
	}

	if clearDNS {
		shoot.Spec.DNS = &gardencorev1beta1.DNS{}
	}
}

// SetProviderConfigsFromFilepath parses the infrastructure, controlPlane and networking provider-configs and sets them on the shoot
func SetProviderConfigsFromFilepath(shoot *gardencorev1beta1.Shoot, infrastructureConfigPath, controlPlaneConfigPath, networkingConfigPath string) error {
	// clear provider configs first
	shoot.Spec.Provider.InfrastructureConfig = nil
	shoot.Spec.Provider.ControlPlaneConfig = nil
	shoot.Spec.Networking.ProviderConfig = nil

	if StringSet(infrastructureConfigPath) {
		infrastructureProviderConfig, err := ParseFileAsProviderConfig(infrastructureConfigPath)
		if err != nil {
			return err
		}
		shoot.Spec.Provider.InfrastructureConfig = infrastructureProviderConfig
	}

	if StringSet(controlPlaneConfigPath) {
		controlPlaneProviderConfig, err := ParseFileAsProviderConfig(controlPlaneConfigPath)
		if err != nil {
			return err
		}
		shoot.Spec.Provider.ControlPlaneConfig = controlPlaneProviderConfig
	}

	if StringSet(networkingConfigPath) {
		networkingProviderConfig, err := ParseFileAsProviderConfig(networkingConfigPath)
		if err != nil {
			return err
		}
		shoot.Spec.Networking.ProviderConfig = networkingProviderConfig
	}

	return nil
}

func generateRandomShootName(prefix string, length int) (string, error) {
	randomString, err := utils.GenerateRandomString(length)
	if err != nil {
		return "", err
	}

	if len(prefix) > 0 {
		return prefix + strings.ToLower(randomString), nil
	}

	return IntegrationTestPrefix + strings.ToLower(randomString), nil
}

// PrettyPrintObject prints a object as pretty printed yaml to stdout
func PrettyPrintObject(obj runtime.Object) error {
	d, err := yaml.Marshal(obj)
	if err != nil {
		return err
	}
	fmt.Print(string(d))
	return nil
}
