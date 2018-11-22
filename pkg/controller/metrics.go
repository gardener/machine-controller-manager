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

	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
)

var (
	machineCountDesc           = prometheus.NewDesc("mcm_machine_items_total", "Count of machines currently managed by the mcm.", nil, nil)
	machineSetCountDesc        = prometheus.NewDesc("mcm_machineset_items_total", "Count of machinesets currently managed by the mcm.", nil, nil)
	machineDeploymentCountDesc = prometheus.NewDesc("mcm_machinedeployment_items_total", "Count of machinedeployments currently managed by the mcm.", nil, nil)

	MachineCreated = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "mcm_machine_created",
		Help: "Creation time of the machines currently managed by the mcm.",
	}, []string{"Name", "Namespace", "UID"})

	MachineInfo = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "mcm_machine_info",
		Help: "Information of the machines currently managed by the mcm.",
	}, []string{"Name", "Namespace", "UID", "Generation", "Kind", "APIVersion",
		"spec_ProviderID", "spec_class_APIGroup", "spec_class_kind", "spec_class_name"})

	MachineStatusCondition = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "mcm_machine_status_condition",
		Help: "Information of the mcm managed machines' status conditions.",
	}, []string{"Machine", "Namespace", "Condition", "Status", "LastTransitionTime"})

	// ScrapeFailedCounter is a Prometheus metric, which counts errors during metrics collection.
	ScrapeFailedCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "mcm_scrape_failure_total",
		Help: "Total count of scrape failures.",
	}, []string{"kind"})
)

func init() {
	prometheus.MustRegister(ScrapeFailedCounter)
	prometheus.MustRegister(MachineCreated)
	prometheus.MustRegister(MachineInfo)
	prometheus.MustRegister(MachineStatusCondition)
}

// Describe is method required to implement the prometheus.Collect interface.
func (c *controller) Describe(ch chan<- *prometheus.Desc) {
	ch <- machineCountDesc
}

// Collect is method required to implement the prometheus.Collect interface.
func (c *controller) Collect(ch chan<- prometheus.Metric) {
	// Collect the count of machines managed by the mcm.
	machineList, err := c.machineLister.Machines(c.namespace).List(labels.Everything())
	if err != nil {
		ScrapeFailedCounter.With(prometheus.Labels{"kind": "machine-count"}).Inc()
		return
	}

	for _, machine := range machineList {
		mMeta := machine.ObjectMeta
		mType := machine.TypeMeta
		mSpec := machine.Spec

		MachineCreated.With(prometheus.Labels{
			"Name":      mMeta.Name,
			"Namespace": mMeta.Namespace,
			"UID":       string(mMeta.UID)}).Set(
			float64(mMeta.GetCreationTimestamp().Time.Unix()))

		MachineInfo.With(prometheus.Labels{
			"Name":                mMeta.Name,
			"Namespace":           mMeta.Namespace,
			"UID":                 string(mMeta.UID),
			"Generation":          strconv.FormatInt(mMeta.Generation, 10),
			"Kind":                mType.Kind,
			"APIVersion":          mType.APIVersion,
			"spec_ProviderID":     mSpec.ProviderID,
			"spec_class_APIGroup": mSpec.Class.APIGroup,
			"spec_class_kind":     mSpec.Class.Kind,
			"spec_class_name":     mSpec.Class.Name}).Set(float64(1))

		for _, condition := range machine.Status.Conditions {
			status := 0
			switch condition.Status {
			case v1.ConditionTrue:
				status = 1
			case v1.ConditionFalse:
				status = 0
			case v1.ConditionUnknown:
				status = 2
			}

			MachineStatusCondition.With(prometheus.Labels{
				"Machine":            mMeta.Name,
				"Namespace":          mMeta.Namespace,
				"Condition":          string(condition.Type),
				"Status":             string(condition.Status),
				"LastTransitionTime": strconv.FormatInt(condition.LastTransitionTime.Time.Unix(), 10)}).Set(float64(status))

		}
	}

	metric, err := prometheus.NewConstMetric(machineCountDesc, prometheus.GaugeValue, float64(len(machineList)))
	if err != nil {
		ScrapeFailedCounter.With(prometheus.Labels{"kind": "machine-count"}).Inc()
		return
	}
	ch <- metric

	machineSetList, err := c.machineSetLister.MachineSets(c.namespace).List(labels.Everything())
	if err != nil {
		ScrapeFailedCounter.With(prometheus.Labels{"kind": "machineset-count"}).Inc()
		return
	}
	metric, err = prometheus.NewConstMetric(machineSetCountDesc, prometheus.GaugeValue, float64(len(machineSetList)))
	if err != nil {
		ScrapeFailedCounter.With(prometheus.Labels{"kind": "machineset-count"}).Inc()
		return
	}
	ch <- metric

	machineDeploymentList, err := c.machineDeploymentLister.MachineDeployments(c.namespace).List(labels.Everything())
	if err != nil {
		ScrapeFailedCounter.With(prometheus.Labels{"kind": "machinedeployment-count"}).Inc()
		return
	}
	metric, err = prometheus.NewConstMetric(machineDeploymentCountDesc, prometheus.GaugeValue, float64(len(machineDeploymentList)))
	if err != nil {
		ScrapeFailedCounter.With(prometheus.Labels{"kind": "machinedeployment-count"}).Inc()
		return
	}
	ch <- metric
}
