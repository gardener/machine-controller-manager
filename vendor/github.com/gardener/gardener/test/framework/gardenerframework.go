// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"flag"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var gardenerCfg *GardenerConfig

// GardenerConfig is the configuration for a gardener framework
type GardenerConfig struct {
	CommonConfig       *CommonConfig
	GardenerKubeconfig string
	ProjectNamespace   string
}

// GardenerFramework is the gardener test framework that includes functions for working with a gardener instance
type GardenerFramework struct {
	*CommonFramework
	TestDescription
	GardenClient kubernetes.Interface

	ProjectNamespace        string
	GardenerFrameworkConfig *GardenerConfig
}

// NewGardenerFramework creates a new gardener test framework.
// All needed  flags are parsed during before each suite.
func NewGardenerFramework(cfg *GardenerConfig) *GardenerFramework {
	var commonConfig *CommonConfig
	if cfg != nil {
		commonConfig = cfg.CommonConfig
	}
	f := &GardenerFramework{
		CommonFramework:         NewCommonFramework(commonConfig),
		TestDescription:         NewTestDescription("GARDENER"),
		GardenerFrameworkConfig: cfg,
	}
	ginkgo.BeforeEach(f.BeforeEach)
	CAfterEach(func(ctx context.Context) {
		if !ginkgo.CurrentGinkgoTestDescription().Failed {
			return
		}
		f.DumpState(ctx)
	}, 10*time.Minute)
	return f
}

// NewGardenerFrameworkFromConfig creates a new gardener test framework without registering ginkgo specific functions
func NewGardenerFrameworkFromConfig(cfg *GardenerConfig) *GardenerFramework {
	var commonConfig *CommonConfig
	if cfg != nil {
		commonConfig = cfg.CommonConfig
	}
	f := &GardenerFramework{
		CommonFramework:         NewCommonFrameworkFromConfig(commonConfig),
		TestDescription:         NewTestDescription("GARDENER"),
		GardenerFrameworkConfig: cfg,
	}
	return f
}

// BeforeEach should be called in ginkgo's BeforeEach.
// It sets up the gardener framework.
func (f *GardenerFramework) BeforeEach() {
	f.GardenerFrameworkConfig = mergeGardenerConfig(f.GardenerFrameworkConfig, gardenerCfg)
	validateGardenerConfig(f.GardenerFrameworkConfig)
	gardenClient, err := kubernetes.NewClientFromFile("", f.GardenerFrameworkConfig.GardenerKubeconfig, kubernetes.WithClientOptions(
		client.Options{
			Scheme: kubernetes.GardenScheme,
		}),
	)
	gomega.Expect(err).ToNot(gomega.HaveOccurred())
	f.GardenClient = gardenClient

	f.ProjectNamespace = f.GardenerFrameworkConfig.ProjectNamespace
}

func validateGardenerConfig(cfg *GardenerConfig) {
	if cfg == nil {
		ginkgo.Fail("no gardener framework configuration provided")
	}
	if !StringSet(cfg.GardenerKubeconfig) {
		ginkgo.Fail("you need to specify the correct path for the kubeconfig")
	}

	if !FileExists(cfg.GardenerKubeconfig) {
		ginkgo.Fail("kubeconfig path does not exist")
	}
}

func mergeGardenerConfig(base, overwrite *GardenerConfig) *GardenerConfig {
	if base == nil {
		return overwrite
	}
	if overwrite == nil {
		return base
	}

	if overwrite.CommonConfig != nil {
		base.CommonConfig = overwrite.CommonConfig
	}
	if StringSet(overwrite.ProjectNamespace) {
		base.ProjectNamespace = overwrite.ProjectNamespace
	}
	if StringSet(overwrite.GardenerKubeconfig) {
		base.GardenerKubeconfig = overwrite.GardenerKubeconfig
	}

	return base
}

// RegisterGardenerFrameworkFlags adds all flags that are needed to configure a gardener framework to the provided flagset.
func RegisterGardenerFrameworkFlags() *GardenerConfig {
	_ = RegisterCommonFrameworkFlags()

	newCfg := &GardenerConfig{}

	flag.StringVar(&newCfg.GardenerKubeconfig, "kubecfg", "", "the path to the kubeconfig  of the garden cluster that will be used for integration tests")
	flag.StringVar(&newCfg.ProjectNamespace, "project-namespace", "", "specify the gardener project namespace to run tests")

	gardenerCfg = newCfg
	return gardenerCfg
}

// NewShootFramework creates a new shoot framework with the current gardener framework
// and a shoot
func (f *GardenerFramework) NewShootFramework(shoot *gardencorev1beta1.Shoot) (*ShootFramework, error) {
	shootFramework := &ShootFramework{
		GardenerFramework: f,
		Config: &ShootConfig{
			GardenerConfig: f.GardenerFrameworkConfig,
		},
	}
	if err := shootFramework.AddShoot(context.TODO(), shoot.GetName(), shoot.GetNamespace()); err != nil {
		return nil, err
	}
	return shootFramework, nil
}
