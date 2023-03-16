/*
Copyright 2014 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

This file was copied and modified from the kubernetes/kubernetes project
https://github.com/kubernetes/kubernetes/release-1.8/cmd/kube-controller-manager/controller_manager.go

Modifications Copyright (c) 2017 SAP SE or an SAP affiliate company. All rights reserved.
*/

package main

import (
	"fmt"
	"os"

	"github.com/gardener/machine-controller-manager/cmd/machine-controller-manager/app"
	"github.com/gardener/machine-controller-manager/cmd/machine-controller-manager/app/options"
	_ "github.com/gardener/machine-controller-manager/pkg/util/client/metrics/prometheus" // for client metric registration
	_ "github.com/gardener/machine-controller-manager/pkg/util/reflector/prometheus"      // for reflector metric registration
	_ "github.com/gardener/machine-controller-manager/pkg/util/workqueue/prometheus"      // for workqueue metric registration
	"github.com/spf13/pflag"
	"k8s.io/component-base/cli/flag"
	"k8s.io/component-base/logs"
	// TODO version should be enabled later on. DON'T import k8s.io/kubernetes 	_ "k8s.io/kubernetes/pkg/version/prometheus"        // for version metric registration
	// TODO version should be enabled later on. DON'T import k8s.io/kubernetes  "k8s.io/kubernetes/pkg/version/verflag"
)

func main() {

	s := options.NewMCMServer()
	s.AddFlags(pflag.CommandLine)

	flag.InitFlags()
	logs.InitLogs()
	defer logs.FlushLogs()

	// verflag.PrintAndExitIfRequested()

	if err := app.Run(s); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

}
