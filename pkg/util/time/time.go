/*
Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved.

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
