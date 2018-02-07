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

Modifications Copyright 2017 The Gardener Authors.
*/

package options

import (
	"time"

	nodeconfig "github.com/gardener/node-controller-manager/pkg/options"
	"github.com/spf13/pflag"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"

	"k8s.io/kubernetes/pkg/apis/componentconfig"
	"k8s.io/kubernetes/pkg/client/leaderelectionconfig"

	// add the node feature gates
	_ "github.com/gardener/node-controller-manager/pkg/features"
)

// NCMServer is the main context object for the controller manager.
type NCMServer struct {
	nodeconfig.NodeControllerManagerConfiguration

	KubeconfigTarget string
	KubeconfigControl string
}

// NewNCMServer creates a new NCMServer with a default config.
func NewNCMServer() *NCMServer {

	s := NCMServer{
		// Part of these default values also present in 'cmd/cloud-controller-manager/app/options/options.go'.
		// Please keep them in sync when doing update.
		NodeControllerManagerConfiguration: nodeconfig.NodeControllerManagerConfiguration{
			Port:                    10258,
			Namespace:               "default",	
			Address:                 "0.0.0.0",
			ConcurrentNodeSyncs:     5,
			ContentType:             "application/vnd.kubernetes.protobuf",
			MinResyncPeriod:         metav1.Duration{Duration: 12 * time.Hour},
			KubeAPIQPS:              20.0,
			KubeAPIBurst:            30,
			LeaderElection:          leaderelectionconfig.DefaultLeaderElectionConfiguration(),
			ControllerStartInterval: metav1.Duration{Duration: 0 * time.Second},
		},
	}
	s.LeaderElection.LeaderElect = true
	return &s
}

// AddFlags adds flags for a specific CMServer to the specified FlagSet
func (s *NCMServer) AddFlags(fs *pflag.FlagSet) {
	fs.Int32Var(&s.Port, "port", s.Port, "The port that the controller-manager's http service runs on")
	fs.Var(componentconfig.IPVar{Val: &s.Address}, "address", "The IP address to serve on (set to 0.0.0.0 for all interfaces)")
	fs.StringVar(&s.CloudProvider, "cloud-provider", s.CloudProvider, "The provider for cloud services.  Empty string for no provider.")
	fs.Int32Var(&s.ConcurrentNodeSyncs, "concurrent-syncs", s.ConcurrentNodeSyncs, "The number of nodes that are allowed to sync concurrently. Larger number = more responsive service management, but more CPU (and network) load")

	fs.DurationVar(&s.MinResyncPeriod.Duration, "min-resync-period", s.MinResyncPeriod.Duration, "The resync period in reflectors will be random between MinResyncPeriod and 2*MinResyncPeriod")

	fs.BoolVar(&s.EnableProfiling, "profiling", true, "Enable profiling via web interface host:port/debug/pprof/")
	fs.BoolVar(&s.EnableContentionProfiling, "contention-profiling", false, "Enable lock contention profiling, if profiling is enabled")
	fs.StringVar(&s.KubeconfigTarget, "target-kubeconfig", s.KubeconfigTarget, "Path to kubeconfig file with authorization and master location information.")
	fs.StringVar(&s.KubeconfigControl, "control-kubeconfig", s.KubeconfigControl, "Path to seed/controlplane cluster's kubeconfig file with authorization and master location information.")	
	fs.StringVar(&s.Namespace, "namespace", s.Namespace, "Name of the namespace in seed cluster where controller would look for CRDs")
	fs.StringVar(&s.ContentType, "kube-api-content-type", s.ContentType, "Content type of requests sent to apiserver.")
	fs.Float32Var(&s.KubeAPIQPS, "kube-api-qps", s.KubeAPIQPS, "QPS to use while talking with kubernetes apiserver")
	fs.Int32Var(&s.KubeAPIBurst, "kube-api-burst", s.KubeAPIBurst, "Burst to use while talking with kubernetes apiserver")
	fs.DurationVar(&s.ControllerStartInterval.Duration, "controller-start-interval", s.ControllerStartInterval.Duration, "Interval between starting controller managers.")

	leaderelectionconfig.BindFlags(&s.LeaderElection, fs)

	// TODO: DefaultFeatureGate is global and it adds all k8s flags
	// utilfeature.DefaultFeatureGate.AddFlag(fs)
}

// Validate is used to validate the options and config before launching the controller manager
func (s *NCMServer) Validate() error {
	var errs []error
	// TODO add validation
	return utilerrors.NewAggregate(errs)
}
