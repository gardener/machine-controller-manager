// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

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
