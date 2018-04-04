// Copyright 2018 The Gardener Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package handlers

import (
	"net/http"
	"sync"
)

var (
	mutex  sync.Mutex
	lively = false
)

// UpdateLiveness expects a boolean value <isLive> and assigns it to the package-internal 'lively' variable.
func UpdateLiveness(isLive bool) {
	mutex.Lock()
	lively = isLive
	mutex.Unlock()
}

// Livez is a HTTP handler for the /livez endpoint which responses with 200 OK status code
// if the Gardener controller manager is lively; and with 500 Internal Server error status code otherwise.
func Livez(w http.ResponseWriter, r *http.Request) {
	mutex.Lock()
	isLive := lively
	mutex.Unlock()
	if isLive {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusInternalServerError)
	}
}
