/*
Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved.

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

// Package controller is used to provide the core functionalities of machine-controller-manager
package controller

import (
	"strconv"

	v1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/gardener/machine-controller-manager/pkg/util/provider/metrics"
	"github.com/prometheus/client_golang/prometheus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// Describe is method required to implement the prometheus.Collect interface.
func (c *controller) Describe(ch chan<- *prometheus.Desc) {
	ch <- metrics.MachineCountDesc
}

// Collect is method required to implement the prometheus.Collect interface.
func (c *controller) Collect(ch chan<- prometheus.Metric) {
	c.CollectMachineMetrics(ch)
	c.CollectMachineControllerFrozenStatusMetrics(ch)
}

// CollectMachineMetrics is method to collect Machine related metrics.
func (c *controller) CollectMachineMetrics(ch chan<- prometheus.Metric) {
	// Collect the count of machines managed by the mcm.
	machineList, err := c.machineLister.Machines(c.namespace).List(labels.Everything())
	if err != nil {
		metrics.ScrapeFailedCounter.With(prometheus.Labels{"kind": "Machine-count"}).Inc()
		return
	}

	for _, machine := range machineList {
		mMeta := machine.ObjectMeta
		mSpec := machine.Spec
		updateMachineInfoMetric(mMeta, mSpec)
		updateMachineStatusConditionMetric(machine, mMeta)
		updateMachineCSPhaseMetric(machine, mMeta)
	}

	updateMachineCountMetric(ch, machineList)
}

// CollectMachineControllerFrozenStatusMetrics is method to collect Machine controller state related metrics.
func (c *controller) CollectMachineControllerFrozenStatusMetrics(ch chan<- prometheus.Metric) {
	var frozenStatus float64
	if c.safetyOptions.MachineControllerFrozen {
		frozenStatus = 1
	}
	metric, err := prometheus.NewConstMetric(metrics.MachineControllerFrozenDesc, prometheus.GaugeValue, frozenStatus)
	if err != nil {
		metrics.ScrapeFailedCounter.With(prometheus.Labels{"kind": "Machine-count"}).Inc()
		return
	}
	ch <- metric
}

func updateMachineCountMetric(ch chan<- prometheus.Metric, machineList []*v1alpha1.Machine) {
	metric, err := prometheus.NewConstMetric(metrics.MachineCountDesc, prometheus.GaugeValue, float64(len(machineList)))
	if err != nil {
		metrics.ScrapeFailedCounter.With(prometheus.Labels{"kind": "Machine-count"}).Inc()
		return
	}
	ch <- metric
}

func updateMachineCSPhaseMetric(machine *v1alpha1.Machine, mMeta metav1.ObjectMeta) {
	var phase float64
	switch machine.Status.CurrentStatus.Phase {
	case v1alpha1.MachineTerminating:
		phase = -4
	case v1alpha1.MachineFailed:
		phase = -3
	case v1alpha1.MachineCrashLoopBackOff:
		phase = -2
	case v1alpha1.MachineUnknown:
		phase = -1
	case v1alpha1.MachinePending:
		phase = 0
	case v1alpha1.MachineRunning:
		phase = 1
	}
	metrics.MachineCSPhase.With(prometheus.Labels{
		"name":      mMeta.Name,
		"namespace": mMeta.Namespace,
	}).Set(phase)
}

func updateMachineStatusConditionMetric(machine *v1alpha1.Machine, mMeta metav1.ObjectMeta) {
	for _, condition := range machine.Status.Conditions {
		var status float64
		switch condition.Status {
		case v1.ConditionTrue:
			status = 1
		case v1.ConditionFalse:
			status = 0
		case v1.ConditionUnknown:
			status = 2
		}

		metrics.MachineStatusCondition.With(prometheus.Labels{
			"name":      mMeta.Name,
			"namespace": mMeta.Namespace,
			"condition": string(condition.Type),
		}).Set(status)
	}
}

func updateMachineInfoMetric(mMeta metav1.ObjectMeta, mSpec v1alpha1.MachineSpec) {
	metrics.MachineInfo.With(prometheus.Labels{
		"name":                 mMeta.Name,
		"namespace":            mMeta.Namespace,
		"createdAt":            strconv.FormatInt(mMeta.GetCreationTimestamp().Time.Unix(), 10),
		"spec_provider_id":     mSpec.ProviderID,
		"spec_class_api_group": mSpec.Class.APIGroup,
		"spec_class_kind":      mSpec.Class.Kind,
		"spec_class_name":      mSpec.Class.Name}).Set(float64(1))
}
