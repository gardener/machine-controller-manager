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

package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

const (
	namespace         = "mcm"
	machineSubsystem  = "machine"
	cloudAPISubsystem = "cloud_api"
	miscSubsystem     = "misc"
)

// variables for subsystem: machine
var (
	// MachineControllerFrozenDesc is a metric about MachineController's frozen status
	MachineControllerFrozenDesc = prometheus.NewDesc("mcm_machine_controller_frozen", "Frozen status of the machine controller manager.", nil, nil)

	// MachineCountDesc is a metric about machine count of the mcm manages
	MachineCountDesc = prometheus.NewDesc("mcm_machine_items_total", "Count of machines currently managed by the mcm.", nil, nil)

	// MachineCSPhase Current status phase of the Machines currently managed by the mcm.
	MachineCSPhase = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: machineSubsystem,
		Name:      "current_status_phase",
		Help:      "Current status phase of the Machines currently managed by the mcm.",
	}, []string{"name", "namespace"})

	// MachineInfo Information of the Machines currently managed by the mcm.
	MachineInfo = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: machineSubsystem,
		Name:      "info",
		Help:      "Information of the Machines currently managed by the mcm.",
	}, []string{"name", "namespace", "createdAt",
		"spec_provider_id", "spec_class_api_group", "spec_class_kind", "spec_class_name"})

	// MachineStatusCondition Information of the mcm managed Machines' status conditions
	MachineStatusCondition = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: machineSubsystem,
		Name:      "status_condition",
		Help:      "Information of the mcm managed Machines' status conditions.",
	}, []string{"name", "namespace", "condition"})
)

// variables for subsystem: cloud_api
var (
	// APIRequestCount Number of Cloud Service API requests, partitioned by provider, and service.
	APIRequestCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: cloudAPISubsystem,
		Name:      "requests_total",
		Help:      "Number of Cloud Service API requests, partitioned by provider, and service.",
	}, []string{"provider", "service"},
	)

	// APIFailedRequestCount Number of Failed Cloud Service API requests, partitioned by provider, and service.
	APIFailedRequestCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: cloudAPISubsystem,
		Name:      "requests_failed_total",
		Help:      "Number of Failed Cloud Service API requests, partitioned by provider, and service.",
	}, []string{"provider", "service"},
	)
)

// variables for subsystem: misc
var (
	// ScrapeFailedCounter is a Prometheus metric, which counts errors during metrics collection.
	ScrapeFailedCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace:   namespace,
		Subsystem:   miscSubsystem,
		Name:        "scrape_failure_total",
		Help:        "Total count of scrape failures.",
		ConstLabels: map[string]string{"binary": "machine-controller-manager-provider"},
	}, []string{"kind"})
)

func registerMachineSubsystemMetrics() {
	prometheus.MustRegister(MachineInfo)
	prometheus.MustRegister(MachineStatusCondition)
	prometheus.MustRegister(MachineCSPhase)
}

func registerCloudAPISubsystemMetrics() {
	prometheus.MustRegister(APIRequestCount)
	prometheus.MustRegister(APIFailedRequestCount)
}

func registerMiscellaneousMetrics() {
	prometheus.MustRegister(ScrapeFailedCounter)
}

func init() {
	registerMachineSubsystemMetrics()
	registerCloudAPISubsystemMetrics()
	registerMiscellaneousMetrics()
}
