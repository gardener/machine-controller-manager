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
	"flag"
	"os"
	"path/filepath"

	"github.com/gardener/gardener/pkg/logger"
	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
)

var commonCfg *CommonConfig

// CommonConfig is the configuration for a common framework
type CommonConfig struct {
	LogLevel         string
	DisableStateDump bool
	ResourceDir      string
	ChartDir         string
}

// CommonFramework represents the common gardener test framework that consolidates all
// shared features of the specific test frameworks (system, garderner, shoot)
type CommonFramework struct {
	Config           *CommonConfig
	Logger           *logrus.Logger
	DisableStateDump bool

	// ResourcesDir is the absolute path to the resources directory
	ResourcesDir string

	// TemplatesDir is the absolute path to the templates directory
	TemplatesDir string

	// Chart is the absolute path to the helm chart directory
	ChartDir string
}

// NewCommonFramework creates a new common framework and registers its ginkgo BeforeEach setup
func NewCommonFramework(cfg *CommonConfig) *CommonFramework {
	f := NewCommonFrameworkFromConfig(cfg)
	ginkgo.BeforeEach(f.BeforeEach)
	return f
}

// NewCommonFrameworkFromConfig creates a new common framework and registers its ginkgo BeforeEach setup
func NewCommonFrameworkFromConfig(cfg *CommonConfig) *CommonFramework {
	f := &CommonFramework{
		Config: cfg,
	}
	return f
}

// BeforeEach should be called in ginkgo's BeforeEach.
// It sets up the common framework.
func (f *CommonFramework) BeforeEach() {
	var err error

	f.Config = mergeCommonConfigs(f.Config, commonCfg)

	f.Logger = logger.AddWriter(logger.NewLogger(f.Config.LogLevel), ginkgo.GinkgoWriter)
	f.DisableStateDump = f.Config.DisableStateDump

	if f.ResourcesDir == "" {
		if f.Config.ResourceDir != "" {
			f.ResourcesDir, err = filepath.Abs(f.Config.ResourceDir)
		} else {
			// This is the default location if the framework is running in one of the gardener/shoot suites.
			// Otherwise the resource dir has to be adjusted
			f.ResourcesDir, err = filepath.Abs(filepath.Join("..", "..", "framework", "resources"))
		}
		ExpectNoError(err)
	}
	FileExists(f.ResourcesDir)

	f.TemplatesDir = filepath.Join(f.ResourcesDir, "templates")

	f.ChartDir = filepath.Join(f.ResourcesDir, "charts")
	if f.Config.ChartDir != "" {
		f.ChartDir = f.Config.ChartDir
	}
}

// CommonAfterSuite performs necessary common steps after all tests of a suite a run
func CommonAfterSuite() {

	// run all registered cleanup functions
	RunCleanupActions()

	resourcesDir, err := filepath.Abs(filepath.Join("..", "..", "framework", "resources"))
	ExpectNoError(err)
	err = os.RemoveAll(filepath.Join(resourcesDir, "charts"))
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	err = os.RemoveAll(filepath.Join(resourcesDir, "repository", "cache"))
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
}

func mergeCommonConfigs(base, overwrite *CommonConfig) *CommonConfig {
	if base == nil {
		return overwrite
	}
	if overwrite == nil {
		return base
	}

	if StringSet(overwrite.LogLevel) {
		base.LogLevel = overwrite.LogLevel
	}
	if StringSet(overwrite.ResourceDir) {
		base.ResourceDir = overwrite.ResourceDir
	}
	if StringSet(overwrite.ChartDir) {
		base.ChartDir = overwrite.ChartDir
	}
	if overwrite.DisableStateDump {
		base.DisableStateDump = overwrite.DisableStateDump
	}
	return base
}

// RegisterCommonFrameworkFlags adds all flags that are needed to configure a common framework to the provided flagset.
func RegisterCommonFrameworkFlags() *CommonConfig {
	newCfg := &CommonConfig{}

	flag.StringVar(&newCfg.LogLevel, "verbose", "", "verbosity level, when set, logging level will be DEBUG")
	flag.BoolVar(&newCfg.DisableStateDump, "disable-dump", false, "Disable the state dump if a test fails")

	commonCfg = newCfg
	return commonCfg
}
