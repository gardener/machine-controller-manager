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

func TestUpdateLiveness(t *testing.T) {
	type testCase struct {
		name            string
		initialLiveness bool
		updatedLiveness bool
	}

	tests := []testCase{
		{"update liveness from false to true", false, true},
		{"update liveness from true to false", true, false},
		{"update liveness with same value", false, false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			lively = test.initialLiveness

			UpdateLiveness(test.updatedLiveness)

			if lively != test.updatedLiveness {
				t.Errorf("Liveness not updated correctly, got: %t, want: %t.", lively, test.updatedLiveness)
			}
		})
	}
}

func TestLivez(t *testing.T) {
	type testCase struct {
		name           string
		liveness       bool
		expectedStatus int
	}

	tests := []testCase{
		{"respond with 200 when live", true, 200},
		{"respond with 500 when not live", false, 500},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			lively = test.liveness
			fakeResponseWriter := httptest.NewRecorder()

			Livez(fakeResponseWriter, nil)

			actualStatus := fakeResponseWriter.Result().StatusCode

			if actualStatus != test.expectedStatus {
				t.Errorf("Livez endpoint incorrect response, got: %d, want: %d.", actualStatus, test.expectedStatus)
			}
		})
	}
}
