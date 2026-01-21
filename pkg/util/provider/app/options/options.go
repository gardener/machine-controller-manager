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

Modifications Copyright SAP SE or an SAP affiliate company and Gardener contributors
*/

package options

import (
	"fmt"
	"mime"
	"net"
	"time"

	drain "github.com/gardener/machine-controller-manager/pkg/util/provider/drain"
	machineconfig "github.com/gardener/machine-controller-manager/pkg/util/provider/options"
	"github.com/spf13/pflag"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/component-base/logs"

	"github.com/gardener/machine-controller-manager/pkg/util/client/leaderelectionconfig"

	// add the machine feature gates
	"github.com/gardener/machine-controller-manager/pkg/apis/constants"
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
				MachineInPlaceUpdateTimeout:              metav1.Duration{Duration: 20 * time.Minute},
				MaxEvictRetries:                          drain.DefaultMaxEvictRetries,
				PvDetachTimeout:                          metav1.Duration{Duration: 2 * time.Minute},
				PvReattachTimeout:                        metav1.Duration{Duration: 90 * time.Second},
				MachineSafetyOrphanVMsPeriod:             metav1.Duration{Duration: 15 * time.Minute},
				MachineSafetyAPIServerStatusCheckPeriod:  metav1.Duration{Duration: 1 * time.Minute},
				MachineSafetyAPIServerStatusCheckTimeout: metav1.Duration{Duration: 30 * time.Second},
				MachinePreserveTimeout:                   metav1.Duration{Duration: 3 * time.Hour},
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
	fs.BoolVar(&s.EnableProfiling, "profiling", false, "Enable profiling via web interface host:port/debug/pprof/")
	fs.BoolVar(&s.EnableContentionProfiling, "contention-profiling", false, "Enable lock contention profiling, if profiling is enabled")
	fs.StringVar(&s.TargetKubeconfig, "target-kubeconfig", s.TargetKubeconfig, fmt.Sprintf("Filepath to the target cluster's kubeconfig where node objects are expected to join or %q if there is no target cluster", constants.TargetKubeconfigDisabledValue))
	fs.StringVar(&s.ControlKubeconfig, "control-kubeconfig", s.ControlKubeconfig, "Filepath to the control cluster's kubeconfig where machine objects would be created. Optionally you could also use 'inClusterConfig' when pod is running inside control kubeconfig. (Default value is same as target-kubeconfig)")
	fs.StringVar(&s.Namespace, "namespace", s.Namespace, "Name of the namespace in control cluster where controller would look for CRDs and Kubernetes objects")
	fs.StringVar(&s.ContentType, "kube-api-content-type", s.ContentType, "Content type of requests sent to apiserver.")
	fs.Float32Var(&s.KubeAPIQPS, "kube-api-qps", s.KubeAPIQPS, "QPS to use while talking with kubernetes apiserver")
	fs.Int32Var(&s.KubeAPIBurst, "kube-api-burst", s.KubeAPIBurst, "Burst to use while talking with kubernetes apiserver")
	fs.DurationVar(&s.ControllerStartInterval.Duration, "controller-start-interval", s.ControllerStartInterval.Duration, "Interval between starting controller managers.")

	fs.DurationVar(&s.SafetyOptions.MachineCreationTimeout.Duration, "machine-creation-timeout", s.SafetyOptions.MachineCreationTimeout.Duration, "Timeout (in duration) used while joining (during creation) of machine before it is declared as failed.")
	fs.DurationVar(&s.SafetyOptions.MachineHealthTimeout.Duration, "machine-health-timeout", s.SafetyOptions.MachineHealthTimeout.Duration, "Timeout (in duration) used while re-joining (in case of temporary health issues) of machine before it is declared as failed.")
	fs.DurationVar(&s.SafetyOptions.MachineDrainTimeout.Duration, "machine-drain-timeout", drain.DefaultMachineDrainTimeout, "Timeout (in duration) used while draining of machine before deletion, beyond which MCM forcefully deletes machine.")
	fs.DurationVar(&s.SafetyOptions.MachineInPlaceUpdateTimeout.Duration, "machine-inplace-update-timeout", s.SafetyOptions.MachineInPlaceUpdateTimeout.Duration, "Timeout (in duration) used while updating a machine in-place, beyond which it is declared as failed.")
	fs.Int32Var(&s.SafetyOptions.MaxEvictRetries, "machine-max-evict-retries", drain.DefaultMaxEvictRetries, "Maximum number of times evicts would be attempted on a pod before it is forcibly deleted during draining of a machine.")
	fs.DurationVar(&s.SafetyOptions.PvDetachTimeout.Duration, "machine-pv-detach-timeout", s.SafetyOptions.PvDetachTimeout.Duration, "Timeout (in duration) used while waiting for detach of PV while evicting/deleting pods")
	fs.DurationVar(&s.SafetyOptions.PvReattachTimeout.Duration, "machine-pv-reattach-timeout", s.SafetyOptions.PvReattachTimeout.Duration, "Timeout (in duration) used while waiting for reattach of PV onto a different node")
	fs.DurationVar(&s.SafetyOptions.MachineSafetyAPIServerStatusCheckTimeout.Duration, "machine-safety-apiserver-statuscheck-timeout", s.SafetyOptions.MachineSafetyAPIServerStatusCheckTimeout.Duration, "Timeout (in duration) for which the APIServer can be down before declare the machine controller frozen by safety controller")
	fs.DurationVar(&s.SafetyOptions.MachinePreserveTimeout.Duration, "machine-preserve-timeout", s.SafetyOptions.MachinePreserveTimeout.Duration, "Duration for which a failed machine should be preserved if it has the appropriate preserve annotation set.")

	fs.DurationVar(&s.SafetyOptions.MachineSafetyOrphanVMsPeriod.Duration, "machine-safety-orphan-vms-period", s.SafetyOptions.MachineSafetyOrphanVMsPeriod.Duration, "Time period (in duration) used to poll for orphan VMs by safety controller.")
	fs.DurationVar(&s.SafetyOptions.MachineSafetyAPIServerStatusCheckPeriod.Duration, "machine-safety-apiserver-statuscheck-period", s.SafetyOptions.MachineSafetyAPIServerStatusCheckPeriod.Duration, "Time period (in duration) used to poll for APIServer's health by safety controller")
	fs.StringVar(&s.NodeConditions, "node-conditions", s.NodeConditions, "List of comma-separated/case-sensitive node-conditions which when set to True will change machine to a failed state after MachineHealthTimeout duration. It may further be replaced with a new machine if the machine is backed by a machine-set object.")
	fs.StringVar(&s.BootstrapTokenAuthExtraGroups, "bootstrap-token-auth-extra-groups", s.BootstrapTokenAuthExtraGroups, "Comma-separated list of groups to set bootstrap token's \"auth-extra-groups\" field to")

	logs.AddFlags(fs) // adds --v flag for log level.

	leaderelectionconfig.BindFlags(&s.LeaderElection, fs)
	// TODO: DefaultFeatureGate is global and it adds all k8s flags
	// utilfeature.DefaultFeatureGate.AddFlag(fs)
}

// Validate is used to validate the options and config before launching the controller manager
func (s *MCServer) Validate() error {
	var errs []error

	if s.Port < 1 || s.Port > 65535 {
		errs = append(errs, fmt.Errorf("invalid port number provided: got %d", s.Port))
	}
	if ip := net.ParseIP(s.Address); ip == nil {
		errs = append(errs, fmt.Errorf("invalid IP address provided: got %v", ip))
	}
	if s.ConcurrentNodeSyncs <= 0 {
		errs = append(errs, fmt.Errorf("concurrent syncs should be greater than zero: got %d", s.ConcurrentNodeSyncs))
	}
	if s.MinResyncPeriod.Duration < 0 {
		errs = append(errs, fmt.Errorf("min resync period should be a non-negative value: got %v", s.MinResyncPeriod.Duration))
	}
	if !s.EnableProfiling && s.EnableContentionProfiling {
		errs = append(errs, fmt.Errorf("contention-profiling cannot be enabled without enabling profiling"))
	}
	if _, _, err := mime.ParseMediaType(s.ContentType); err != nil {
		errs = append(errs, fmt.Errorf("kube api content type cannot be parsed: %w", err))
	}
	if s.KubeAPIQPS <= 0 {
		errs = append(errs, fmt.Errorf("kube api qps should be greater than zero: got %f", s.KubeAPIQPS))
	}
	if s.KubeAPIBurst < 0 {
		errs = append(errs, fmt.Errorf("kube api burst should not be a negative value: got %d", s.KubeAPIBurst))
	}
	if s.ControllerStartInterval.Duration < 0 {
		errs = append(errs, fmt.Errorf("controller start interval should be a non-negative value: got %v", s.ControllerStartInterval.Duration))
	}
	if s.SafetyOptions.MachineCreationTimeout.Duration < 0 {
		errs = append(errs, fmt.Errorf("machine creation timeout should be a non-negative number: got %v", s.SafetyOptions.MachineCreationTimeout.Duration))
	}
	if s.SafetyOptions.MachineHealthTimeout.Duration < 0 {
		errs = append(errs, fmt.Errorf("machine health timeout should be a non-negative number: got %v", s.SafetyOptions.MachineHealthTimeout.Duration))
	}
	if s.SafetyOptions.MachineDrainTimeout.Duration < 0 {
		errs = append(errs, fmt.Errorf("machine drain timeout should be a non-negative number: got %v", s.SafetyOptions.MachineDrainTimeout.Duration))
	}
	if s.SafetyOptions.MachineInPlaceUpdateTimeout.Duration < 0 {
		errs = append(errs, fmt.Errorf("machine in-place update timeout should be a non-negative number: got %v", s.SafetyOptions.MachineInPlaceUpdateTimeout.Duration))
	}
	if s.SafetyOptions.MaxEvictRetries < 0 {
		errs = append(errs, fmt.Errorf("max evict retries should not be a negative value: got %d", s.SafetyOptions.MaxEvictRetries))
	}
	if s.SafetyOptions.PvDetachTimeout.Duration < 0 {
		errs = append(errs, fmt.Errorf("machine PV detach timeout should be a non-negative number: got %v", s.SafetyOptions.PvDetachTimeout.Duration))
	}
	if s.SafetyOptions.PvReattachTimeout.Duration < 0 {
		errs = append(errs, fmt.Errorf("machine PV reattach timeout should be a non-negative number: got %v", s.SafetyOptions.PvReattachTimeout.Duration))
	}
	if s.SafetyOptions.MachineSafetyAPIServerStatusCheckTimeout.Duration < 0 {
		errs = append(errs, fmt.Errorf("machine safety APIServer status check timeout should be a non-negative number: got %v", s.SafetyOptions.MachineSafetyAPIServerStatusCheckTimeout.Duration))
	}
	if s.SafetyOptions.MachineSafetyOrphanVMsPeriod.Duration < 0 {
		errs = append(errs, fmt.Errorf("machine safety oprhan VMs period should be a non-negative number: got %v", s.SafetyOptions.MachineSafetyOrphanVMsPeriod.Duration))
	}
	if s.SafetyOptions.MachineSafetyAPIServerStatusCheckPeriod.Duration < 0 {
		errs = append(errs, fmt.Errorf("machine safety APIServer status check period should be a non-negative number: got %v", s.SafetyOptions.MachineSafetyAPIServerStatusCheckPeriod.Duration))
	}
	if s.SafetyOptions.MachineSafetyAPIServerStatusCheckPeriod.Duration < s.SafetyOptions.MachineSafetyAPIServerStatusCheckTimeout.Duration {
		errs = append(errs, fmt.Errorf("machine safety APIServer status check period should not be less than APIServer status check timeout"))
	}
	if s.SafetyOptions.MachinePreserveTimeout.Duration < 0 {
		errs = append(errs, fmt.Errorf("machine preserve timeout should be a non-negative number: got %v", s.SafetyOptions.MachinePreserveTimeout.Duration))
	}
	if s.ControlKubeconfig == "" && s.TargetKubeconfig == constants.TargetKubeconfigDisabledValue {
		errs = append(errs, fmt.Errorf("--control-kubeconfig cannot be empty if --target-kubeconfig=%s is specified", constants.TargetKubeconfigDisabledValue))
	}
	return utilerrors.NewAggregate(errs)
}
