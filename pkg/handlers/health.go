// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package handlers

import (
	"k8s.io/apimachinery/pkg/util/runtime"
	"net/http"
	"sync"
)

var (
	mutex   sync.Mutex
	healthy = false
)

// UpdateHealth expects a boolean value <isHealthy> and assigns it to the package-internal 'healthy' variable.
func UpdateHealth(isHealthy bool) {
	mutex.Lock()
	healthy = isHealthy
	mutex.Unlock()
}

// Healthz is an HTTP handler for the /healthz endpoint which responds with 200 OK status code
// if the Machine Controller Manager is healthy; and with 500 Internal Server error status code otherwise.
func Healthz(w http.ResponseWriter, r *http.Request) {
	mutex.Lock()
	isHealthy := healthy
	mutex.Unlock()
	if isHealthy {
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte("OK"))
		runtime.Must(err)
		return
	}

	w.WriteHeader(http.StatusInternalServerError)
	_, err := w.Write([]byte("Unhealthy"))
	runtime.Must(err)
}
