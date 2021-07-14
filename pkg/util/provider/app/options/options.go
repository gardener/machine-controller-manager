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
https://github.com/kubernetes/kubernetes/release-1.8/cmd/kube-controller-manager/app/options/options.go

Modifications Copyright (c) 2017 SAP SE or an SAP affiliate company. All rights reserved.
*/

package options

import (
	"time"

	drain "github.com/gardener/machine-controller-manager/pkg/util/provider/drain"
	machineconfig "github.com/gardener/machine-controller-manager/pkg/util/provider/options"
	"github.com/spf13/pflag"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"

	"github.com/gardener/machine-controller-manager/pkg/util/client/leaderelectionconfig"

	// add the machine feature gates
	_ "github.com/gardener/machine-controller-manager/pkg/features"
)

// MCServer is the main context object for the machine controller.
type MCServer struct {
	machineconfig.MachineControllerConfiguration

	ControlKubeconfig string
	TargetKubeconfig  string
}

// NewMCServer creates a new MCServer with a default config.
func NewMCServer() *MCServer {

	s := MCServer{
		// Part of these default values also present in 'cmd/cloud-controller-manager/app/options/options.go'.
		// Please keep them in sync when doing update.
		MachineControllerConfiguration: machineconfig.MachineControllerConfiguration{
			Port:                    10259,
			Namespace:               "default",
			Address:                 "0.0.0.0",
			ConcurrentNodeSyncs:     50,
			ContentType:             "application/vnd.kubernetes.protobuf",
			NodeConditions:          "KernelDeadlock,ReadonlyFilesystem,DiskPressure,NetworkUnavailable",
			MinResyncPeriod:         metav1.Duration{Duration: 12 * time.Hour},
			KubeAPIQPS:              20.0,
			KubeAPIBurst:            30,
			LeaderElection:          leaderelectionconfig.DefaultLeaderElectionConfiguration(),
			ControllerStartInterval: metav1.Duration{Duration: 0 * time.Second},
			SafetyOptions: machineconfig.SafetyOptions{
				MachineCreationTimeout:                   metav1.Duration{Duration: 20 * time.Minute},
				MachineHealthTimeout:                     metav1.Duration{Duration: 10 * time.Minute},
				MachineDrainTimeout:                      metav1.Duration{Duration: drain.DefaultMachineDrainTimeout},
				MaxEvictRetries:                          drain.DefaultMaxEvictRetries,
				PvDetachTimeout:                          metav1.Duration{Duration: 2 * time.Minute},
				PvReattachTimeout:                        metav1.Duration{Duration: 90 * time.Second},
				MachineSafetyOrphanVMsPeriod:             metav1.Duration{Duration: 30 * time.Minute},
				MachineSafetyAPIServerStatusCheckPeriod:  metav1.Duration{Duration: 1 * time.Minute},
				MachineSafetyAPIServerStatusCheckTimeout: metav1.Duration{Duration: 30 * time.Second},
			},
		},
	}
	s.LeaderElection.LeaderElect = true
	return &s
}

// AddFlags adds flags for a specific CMServer to the specified FlagSet
func (s *MCServer) AddFlags(fs *pflag.FlagSet) {
	fs.Int32Var(&s.Port, "port", s.Port, "The port that the controller-manager's http service runs on")
	fs.Var(machineconfig.IPVar{Val: &s.Address}, "address", "The IP address to serve on (set to 0.0.0.0 for all interfaces)")
	fs.StringVar(&s.CloudProvider, "cloud-provider", s.CloudProvider, "The provider for cloud services.  Empty string for no provider.")
	fs.Int32Var(&s.ConcurrentNodeSyncs, "concurrent-syncs", s.ConcurrentNodeSyncs, "The number of nodes that are allowed to sync concurrently. Larger number = more responsive service management, but more CPU (and network) load")
	fs.DurationVar(&s.MinResyncPeriod.Duration, "min-resync-period", s.MinResyncPeriod.Duration, "The resync period in reflectors will be random between MinResyncPeriod and 2*MinResyncPeriod")
	fs.BoolVar(&s.EnableProfiling, "profiling", true, "Enable profiling via web interface host:port/debug/pprof/")
	fs.BoolVar(&s.EnableContentionProfiling, "contention-profiling", false, "Enable lock contention profiling, if profiling is enabled")
	fs.StringVar(&s.TargetKubeconfig, "target-kubeconfig", s.TargetKubeconfig, "Filepath to the target cluster's kubeconfig where node objects are expected to join")
	fs.StringVar(&s.ControlKubeconfig, "control-kubeconfig", s.ControlKubeconfig, "Filepath to the control cluster's kubeconfig where machine objects would be created. Optionally you could also use 'inClusterConfig' when pod is running inside control kubeconfig. (Default value is same as target-kubeconfig)")
	fs.StringVar(&s.Namespace, "namespace", s.Namespace, "Name of the namespace in control cluster where controller would look for CRDs and Kubernetes objects")
	fs.StringVar(&s.ContentType, "kube-api-content-type", s.ContentType, "Content type of requests sent to apiserver.")
	fs.Float32Var(&s.KubeAPIQPS, "kube-api-qps", s.KubeAPIQPS, "QPS to use while talking with kubernetes apiserver")
	fs.Int32Var(&s.KubeAPIBurst, "kube-api-burst", s.KubeAPIBurst, "Burst to use while talking with kubernetes apiserver")
	fs.DurationVar(&s.ControllerStartInterval.Duration, "controller-start-interval", s.ControllerStartInterval.Duration, "Interval between starting controller managers.")

	fs.DurationVar(&s.SafetyOptions.MachineCreationTimeout.Duration, "machine-creation-timeout", s.SafetyOptions.MachineCreationTimeout.Duration, "Timeout (in durartion) used while joining (during creation) of machine before it is declared as failed.")
	fs.DurationVar(&s.SafetyOptions.MachineHealthTimeout.Duration, "machine-health-timeout", s.SafetyOptions.MachineHealthTimeout.Duration, "Timeout (in durartion) used while re-joining (in case of temporary health issues) of machine before it is declared as failed.")
	fs.DurationVar(&s.SafetyOptions.MachineDrainTimeout.Duration, "machine-drain-timeout", drain.DefaultMachineDrainTimeout, "Timeout (in durartion) used while draining of machine before deletion, beyond which MCM forcefully deletes machine.")
	fs.Int32Var(&s.SafetyOptions.MaxEvictRetries, "machine-max-evict-retries", drain.DefaultMaxEvictRetries, "Maximum number of times evicts would be attempted on a pod before it is forcibly deleted during draining of a machine.")
	fs.DurationVar(&s.SafetyOptions.PvDetachTimeout.Duration, "machine-pv-detach-timeout", s.SafetyOptions.PvDetachTimeout.Duration, "Timeout (in duration) used while waiting for detach of PV while evicting/deleting pods")
	fs.DurationVar(&s.SafetyOptions.PvReattachTimeout.Duration, "machine-pv-reattach-timeout", s.SafetyOptions.PvReattachTimeout.Duration, "Timeout (in duration) used while waiting for reattach of PV onto a different node")
	fs.DurationVar(&s.SafetyOptions.MachineSafetyAPIServerStatusCheckTimeout.Duration, "machine-safety-apiserver-statuscheck-timeout", s.SafetyOptions.MachineSafetyAPIServerStatusCheckTimeout.Duration, "Timeout (in duration) for which the APIServer can be down before declare the machine controller frozen by safety controller")

	fs.DurationVar(&s.SafetyOptions.MachineSafetyOrphanVMsPeriod.Duration, "machine-safety-orphan-vms-period", s.SafetyOptions.MachineSafetyOrphanVMsPeriod.Duration, "Time period (in durartion) used to poll for orphan VMs by safety controller.")
	fs.DurationVar(&s.SafetyOptions.MachineSafetyAPIServerStatusCheckPeriod.Duration, "machine-safety-apiserver-statuscheck-period", s.SafetyOptions.MachineSafetyAPIServerStatusCheckPeriod.Duration, "Time period (in duration) used to poll for APIServer's health by safety controller")
	fs.StringVar(&s.NodeConditions, "node-conditions", s.NodeConditions, "List of comma-separated/case-sensitive node-conditions which when set to True will change machine to a failed state after MachineHealthTimeout duration. It may further be replaced with a new machine if the machine is backed by a machine-set object.")
	fs.StringVar(&s.BootstrapTokenAuthExtraGroups, "bootstrap-token-auth-extra-groups", s.BootstrapTokenAuthExtraGroups, "Comma-separated list of groups to set bootstrap token's \"auth-extra-groups\" field to")

	leaderelectionconfig.BindFlags(&s.LeaderElection, fs)
	// TODO: DefaultFeatureGate is global and it adds all k8s flags
	// utilfeature.DefaultFeatureGate.AddFlag(fs)
}

// Validate is used to validate the options and config before launching the controller manager
func (s *MCServer) Validate() error {
	var errs []error
	// TODO add validation
	return utilerrors.NewAggregate(errs)
}
