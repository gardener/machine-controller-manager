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
	"net/http/httptest"
	"testing"
)

func TestUpdateHealth(t *testing.T) {
	type testCase struct {
		name          string
		initialHealth bool
		updatedHealth bool
	}

	tests := []testCase{
		{"update health from false to true", false, true},
		{"update health from true to false", true, false},
		{"update health with same value", false, false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			healthy = test.initialHealth

			UpdateHealth(test.updatedHealth)

			if healthy != test.updatedHealth {
				t.Errorf("Health not updated correctly, got: %t, want: %t.", healthy, test.updatedHealth)
			}
		})
	}
}

func TestHealthz(t *testing.T) {
	type testCase struct {
		name           string
		health         bool
		expectedStatus int
	}

	tests := []testCase{
		{"respond with 200 when healthy", true, 200},
		{"respond with 500 when not healthy", false, 500},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			healthy = test.health
			fakeResponseWriter := httptest.NewRecorder()

			Healthz(fakeResponseWriter, nil)

			actualStatus := fakeResponseWriter.Result().StatusCode

			if actualStatus != test.expectedStatus {
				t.Errorf("/healthz endpoint incorrect response, got: %d, want: %d.", actualStatus, test.expectedStatus)
			}
		})
	}
}
