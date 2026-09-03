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
	MachineClassCRD []byte
	//go:embed crds/machine.sapcloud.io_machinedeployments.yaml
	MachineDeploymentCRD []byte
	//go:embed crds/machine.sapcloud.io_machinesets.yaml
	MachineSetCRD []byte
	//go:embed crds/machine.sapcloud.io_machines.yaml
	MachineCRD []byte

	CRDs = []*apiextensionsv1.CustomResourceDefinition{
		mustUnmarshal(MachineClassCRD),
		mustUnmarshal(MachineDeploymentCRD),
		mustUnmarshal(MachineSetCRD),
		mustUnmarshal(MachineCRD),
	}
)
