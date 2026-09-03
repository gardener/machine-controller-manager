// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package kubernetes

import (
	_ "embed"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	sigyaml "sigs.k8s.io/yaml"
)

func mustUnmarshal(raw []byte) *apiextensionsv1.CustomResourceDefinition {
	t := new(apiextensionsv1.CustomResourceDefinition)
	err := sigyaml.Unmarshal(raw, &t)
	if err != nil {
		panic(err)
	}
	return t
}

var (
	//go:embed crds/machine.sapcloud.io_machineclasses.yaml
	// MachineClassCRD represents the raw bytes for MachineClass resource.
	MachineClassCRD []byte
	//go:embed crds/machine.sapcloud.io_machinedeployments.yaml
	// MachineDeploymentCRD represents the raw bytes for MachineDeployment resource.
	MachineDeploymentCRD []byte
	//go:embed crds/machine.sapcloud.io_machinesets.yaml
	// MachineSetCRD represents the raw bytes for MachineSet resource.
	MachineSetCRD []byte
	//go:embed crds/machine.sapcloud.io_machines.yaml
	// MachineCRD represents the raw bytes for Machine resource.
	MachineCRD []byte

	// CRDs is a list of all machine-controller-manager CRDs.
	CRDs = []*apiextensionsv1.CustomResourceDefinition{
		mustUnmarshal(MachineClassCRD),
		mustUnmarshal(MachineDeploymentCRD),
		mustUnmarshal(MachineSetCRD),
		mustUnmarshal(MachineCRD),
	}
)
