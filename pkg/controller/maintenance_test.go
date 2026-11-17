// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"testing"

	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func maintenanceMachine(name string, phase machinev1alpha1.MachinePhase, lastOpState machinev1alpha1.MachineState) *machinev1alpha1.Machine {
	return &machinev1alpha1.Machine{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Status: machinev1alpha1.MachineStatus{
			CurrentStatus: machinev1alpha1.CurrentStatus{Phase: phase},
			LastOperation: machinev1alpha1.LastOperation{State: lastOpState},
		},
	}
}

func TestCalculateMachineSetStatusCountsMaintenanceSeparately(t *testing.T) {
	is := &machinev1alpha1.MachineSet{}
	machines := []*machinev1alpha1.Machine{
		maintenanceMachine("running", machinev1alpha1.MachineRunning, machinev1alpha1.MachineStateSuccessful),
		// A failed last operation must not classify a maintenance machine as failed.
		maintenanceMachine("under-maintenance", machinev1alpha1.MachineMaintenance, machinev1alpha1.MachineStateFailed),
		maintenanceMachine("failed", machinev1alpha1.MachineFailed, machinev1alpha1.MachineStateFailed),
	}

	status := calculateMachineSetStatus(is, machines, nil)

	if status.AvailableReplicas != 1 {
		t.Fatalf("expected 1 available replica, got %d", status.AvailableReplicas)
	}
	if status.MaintenanceReplicas != 1 {
		t.Fatalf("expected 1 maintenance replica, got %d", status.MaintenanceReplicas)
	}
	if status.FailedMachines == nil || len(*status.FailedMachines) != 1 || (*status.FailedMachines)[0].Name != "failed" {
		t.Fatalf("expected FailedMachines to contain only the genuinely failed machine, got %+v", status.FailedMachines)
	}
	if status.Replicas != 3 {
		t.Fatalf("expected 3 total replicas, got %d", status.Replicas)
	}
}

func TestIsMachineInMaintenance(t *testing.T) {
	cases := []struct {
		name     string
		phase    machinev1alpha1.MachinePhase
		expected bool
	}{
		{"maintenance", machinev1alpha1.MachineMaintenance, true},
		{"running", machinev1alpha1.MachineRunning, false},
		{"failed", machinev1alpha1.MachineFailed, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			machine := &machinev1alpha1.Machine{
				Status: machinev1alpha1.MachineStatus{
					CurrentStatus: machinev1alpha1.CurrentStatus{Phase: tc.phase},
				},
			}
			got := isMachineInMaintenance(machine)
			if got != tc.expected {
				t.Fatalf("isMachineInMaintenance(%s) = %v, want %v", tc.phase, got, tc.expected)
			}
		})
	}
}

func TestMachineDeploymentCompleteWithMaintenance(t *testing.T) {
	deployment := &machinev1alpha1.MachineDeployment{
		Spec:       machinev1alpha1.MachineDeploymentSpec{Replicas: 2},
		ObjectMeta: metav1.ObjectMeta{Generation: 1},
	}

	cases := []struct {
		name               string
		replicas           int32
		updated            int32
		available          int32
		maintenance        int32
		observedGeneration int64
		expected           bool
	}{
		{name: "complete with maintenance", replicas: 2, updated: 1, available: 1, maintenance: 1, observedGeneration: 1, expected: true},
		{name: "missing updated replica with maintenance", replicas: 2, updated: 0, available: 1, maintenance: 1, observedGeneration: 1, expected: false},
		{name: "complete without maintenance", replicas: 2, updated: 2, available: 2, maintenance: 0, observedGeneration: 1, expected: true},
		{name: "missing updated replica without maintenance", replicas: 2, updated: 1, available: 2, maintenance: 0, observedGeneration: 1, expected: false},
		{name: "missing available replica with maintenance", replicas: 2, updated: 1, available: 0, maintenance: 1, observedGeneration: 1, expected: false},
		{name: "missing available replica without maintenance", replicas: 2, updated: 2, available: 1, maintenance: 0, observedGeneration: 1, expected: false},
		{name: "too few total replicas", replicas: 1, updated: 1, available: 1, maintenance: 1, observedGeneration: 1, expected: false},
		{name: "too many total replicas", replicas: 3, updated: 1, available: 1, maintenance: 1, observedGeneration: 1, expected: false},
		{name: "outdated observed generation", replicas: 2, updated: 1, available: 1, maintenance: 1, observedGeneration: 0, expected: false},
		{name: "newer observed generation", replicas: 2, updated: 1, available: 1, maintenance: 1, observedGeneration: 2, expected: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status := machinev1alpha1.MachineDeploymentStatus{
				Replicas:            tc.replicas,
				UpdatedReplicas:     tc.updated,
				AvailableReplicas:   tc.available,
				MaintenanceReplicas: tc.maintenance,
				ObservedGeneration:  tc.observedGeneration,
			}
			if got := MachineDeploymentComplete(deployment, &status); got != tc.expected {
				t.Fatalf("MachineDeploymentComplete(%+v) = %v, want %v", status, got, tc.expected)
			}
		})
	}
}
