// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package options

import (
	"testing"

	"github.com/spf13/pflag"
)

func TestAddFlagsDisableMachineDrainTimeout(t *testing.T) {
	tests := []struct {
		name            string
		args            []string
		expectedDisable bool
	}{
		{
			name:            "defaults to false",
			args:            []string{},
			expectedDisable: false,
		},
		{
			name: "disable flag set to true",
			args: []string{
				"--disable-machine-drain-timeout=true",
			},
			expectedDisable: true,
		},
		{
			name: "disable flag set to false",
			args: []string{
				"--disable-machine-drain-timeout=false",
			},
			expectedDisable: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := NewMCServer()
			fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
			server.AddFlags(fs)

			if err := fs.Parse(tt.args); err != nil {
				t.Fatalf("failed to parse flags: %v", err)
			}

			if server.SafetyOptions.DisableMachineDrainTimeout != tt.expectedDisable {
				t.Errorf("expected DisableMachineDrainTimeout %v, got %v", tt.expectedDisable, server.SafetyOptions.DisableMachineDrainTimeout)
			}
		})
	}
}
