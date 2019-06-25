/*
Copyright (c) 2017 SAP SE or an SAP affiliate company. All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package options is used to specify options to MCM
package options

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ClientConnectionConfiguration contains details for constructing a client.
type ClientConnectionConfiguration struct {
	// kubeConfigFile is the path to a kubeconfig file.
	KubeConfigFile string
	// acceptContentTypes defines the Accept header sent by clients when connecting to a server, overriding the
	// default value of 'application/json'. This field will control all connections to the server used by a particular
	// client.
	AcceptContentTypes string
	// contentType is the content type used when sending data to the server from this client.
	ContentType string
	// qps controls the number of queries per second allowed for this connection.
	QPS float32
	// burst allows extra queries to accumulate when a client is exceeding its rate.
	Burst int
}

// MachineControllerManagerConfiguration contains machine configurations
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type MachineControllerManagerConfiguration struct {
	metav1.TypeMeta

	// namespace in seed cluster in which controller would look for the resources.
	Namespace string

	// port is the port that the controller-manager's http service runs on.
	Port int32
	// address is the IP address to serve on (set to 0.0.0.0 for all interfaces).
	Address string
	// CloudProvider is the provider for cloud services.
	CloudProvider string
	// ConcurrentNodeSyncs is the number of node objects that are
	// allowed to sync concurrently. Larger number = more responsive nodes,
	// but more CPU (and network) load.
	ConcurrentNodeSyncs int32

	// enableProfiling enables profiling via web interface host:port/debug/pprof/
	EnableProfiling bool
	// enableContentionProfiling enables lock contention profiling, if enableProfiling is true.
	EnableContentionProfiling bool
	// contentType is contentType of requests sent to apiserver.
	ContentType string
	// kubeAPIQPS is the QPS to use while talking with kubernetes apiserver.
	KubeAPIQPS float32
	// kubeAPIBurst is the burst to use while talking with kubernetes apiserver.
	KubeAPIBurst int32
	// leaderElection defines the configuration of leader election client.
	LeaderElection LeaderElectionConfiguration
	// How long to wait between starting controller managers
	ControllerStartInterval metav1.Duration
	// minResyncPeriod is the resync period in reflectors; will be random between
	// minResyncPeriod and 2*minResyncPeriod.
	MinResyncPeriod metav1.Duration

	// SafetyOptions is the set of options to set to ensure safety of controller
	SafetyOptions SafetyOptions
}

// SafetyOptions are used to configure the upper-limit and lower-limit
// while configuring freezing of machineSet objects
type SafetyOptions struct {
	// SafetyUp
	SafetyUp int32
	// SafetyDown
	SafetyDown int32

	// Timeout (in durartion) used while creation of
	// a machine before it is declared as failed
	MachineCreationTimeout metav1.Duration
	// Timeout (in durartion) used while health-check of
	// a machine before it is declared as failed
	MachineHealthTimeout metav1.Duration
	// Timeout (in durartion) used while draining of machine before deletion,
	// beyond which it forcefully deletes machine
	MachineDrainTimeout metav1.Duration
	// Timeout (in duration) used while waiting for PV to detach
	PvDetachTimeout metav1.Duration
	// Timeout (in duration) for which the APIServer can be down before
	// declare the machine controller frozen by safety controller
	MachineSafetyAPIServerStatusCheckTimeout metav1.Duration

	// Period (in durartion) used to poll for orphan VMs
	// by safety controller
	MachineSafetyOrphanVMsPeriod metav1.Duration
	// Period (in durartion) used to poll for overshooting
	// of machine objects backing a machineSet by safety controller
	MachineSafetyOvershootingPeriod metav1.Duration
	// Period (in duration) used to poll for APIServer's health
	// by safety controller
	MachineSafetyAPIServerStatusCheckPeriod metav1.Duration

	// APIserverInactiveStartTime to keep track of the
	// start time of when the APIServers were not reachable
	APIserverInactiveStartTime time.Time
	// MachineControllerFrozen indicates if the machine controller
	// is frozen due to Unreachable APIServers
	MachineControllerFrozen bool
}

// LeaderElectionConfiguration defines the configuration of leader election
// clients for components that can run with leader election enabled.
type LeaderElectionConfiguration struct {
	// leaderElect enables a leader election client to gain leadership
	// before executing the main loop. Enable this when running replicated
	// components for high availability.
	LeaderElect bool
	// leaseDuration is the duration that non-leader candidates will wait
	// after observing a leadership renewal until attempting to acquire
	// leadership of a led but unrenewed leader slot. This is effectively the
	// maximum duration that a leader can be stopped before it is replaced
	// by another candidate. This is only applicable if leader election is
	// enabled.
	LeaseDuration metav1.Duration
	// renewDeadline is the interval between attempts by the acting master to
	// renew a leadership slot before it stops leading. This must be less
	// than or equal to the lease duration. This is only applicable if leader
	// election is enabled.
	RenewDeadline metav1.Duration
	// retryPeriod is the duration the clients should wait between attempting
	// acquisition and renewal of a leadership. This is only applicable if
	// leader election is enabled.
	RetryPeriod metav1.Duration
	// resourceLock indicates the resource object type that will be used to lock
	// during leader election cycles.
	ResourceLock string
}
