// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

// Package time is used to provide the core functionalities of machine-controller-manager
package time

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// HasTimeOutOccurred returns true, when time.Now() is more than time + period
func HasTimeOutOccurred(timeStamp metav1.Time, period time.Duration) bool {
	// Timeout value obtained by subtracting last operation with expected time out period
	timeOut := metav1.Now().Add(-period).Sub(timeStamp.Time)
	return timeOut > 0
}
