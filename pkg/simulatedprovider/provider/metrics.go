// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package provider

import (
	"errors"
	"strconv"
	"time"

	"github.com/gardener/machine-controller-manager/pkg/util/provider/machinecodes/codes"
	"github.com/gardener/machine-controller-manager/pkg/util/provider/machinecodes/status"
	"github.com/gardener/machine-controller-manager/pkg/util/provider/metrics"
)

const prometheusProviderLabelValue = "simulation"

// recordDriverAPIMetric records a prometheus metric capturing the total duration of a successful execution for
// any driver method (e.g. CreateMachine, DeleteMachine etc.). In case an error is returned then a failed counter
// metric is recorded.
func recordDriverAPIMetric(err error, operation string, invocationTime time.Time) {
	if err != nil {
		var (
			statusErr *status.Status
			labels    = []string{prometheusProviderLabelValue, operation}
		)
		if errors.As(err, &statusErr) {
			labels = append(labels, strconv.Itoa(int(statusErr.Code())))
		} else {
			labels = append(labels, strconv.Itoa(int(codes.Internal)))
		}
		metrics.DriverFailedAPIRequests.
			WithLabelValues(labels...).
			Inc()
		return
	}
	// compute the time taken to complete the call and record it as a metric
	elapsed := time.Since(invocationTime)
	metrics.DriverAPIRequestDuration.WithLabelValues(
		prometheusProviderLabelValue,
		operation,
	).Observe(elapsed.Seconds())
}

// driverAPIMetricRecorderFn returns a function that can be used to record a prometheus metric for driver API calls.
// NOTE: a pointer to an error (which itself is a fat interface pointer) is necessary to enable the callers of this function to enclose this call into a `defer` statement.
func driverAPIMetricRecorderFn(operation string, err *error) func() {
	invocationTime := time.Now()
	return func() {
		recordDriverAPIMetric(*err, operation, invocationTime)
	}
}
