/*
Copyright 2016 The Kubernetes Authors.

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
https://github.com/kubernetes/kubernetes/blob/release-1.8/pkg/util/workqueue/prometheus/prometheus.go

Modifications Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved.
*/

package prometheus

import (
	"k8s.io/client-go/util/workqueue"

	"github.com/prometheus/client_golang/prometheus"
)

// Package prometheus sets the workqueue DefaultMetricsFactory to produce
// prometheus metrics. To use this package, you just have to import it.

func init() {
	workqueue.SetProvider(prometheusMetricsProvider{})
}

type prometheusMetricsProvider struct{}

func (prometheusMetricsProvider) NewDepthMetric(name string) workqueue.GaugeMetric {
	depth := prometheus.NewGauge(prometheus.GaugeOpts{
		Subsystem: name,
		Name:      "depth",
		Help:      "Current depth of workqueue: " + name,
	})
	prometheus.Register(depth)
	return depth
}

func (prometheusMetricsProvider) NewAddsMetric(name string) workqueue.CounterMetric {
	adds := prometheus.NewCounter(prometheus.CounterOpts{
		Subsystem: name,
		Name:      "adds",
		Help:      "Total number of adds handled by workqueue: " + name,
	})
	prometheus.Register(adds)
	return adds
}

func (prometheusMetricsProvider) NewLatencyMetric(name string) workqueue.SummaryMetric {
	latency := prometheus.NewSummary(prometheus.SummaryOpts{
		Subsystem: name,
		Name:      "queue_latency",
		Help:      "How long an item stays in workqueue" + name + " before being requested.",
	})
	prometheus.Register(latency)
	return latency
}

func (prometheusMetricsProvider) NewWorkDurationMetric(name string) workqueue.SummaryMetric {
	workDuration := prometheus.NewSummary(prometheus.SummaryOpts{
		Subsystem: name,
		Name:      "work_duration",
		Help:      "How long processing an item from workqueue" + name + " takes.",
	})
	prometheus.Register(workDuration)
	return workDuration
}

func (prometheusMetricsProvider) NewRetriesMetric(name string) workqueue.CounterMetric {
	retries := prometheus.NewCounter(prometheus.CounterOpts{
		Subsystem: name,
		Name:      "retries",
		Help:      "Total number of retries handled by workqueue: " + name,
	})
	prometheus.Register(retries)
	return retries
}
