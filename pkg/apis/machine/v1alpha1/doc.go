// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

// Package v1alpha1 is the v1alpha1 version of the API.
// +k8s:deepcopy-gen=package,register
// +k8s:conversion-gen=github.com/gardener/machine-controller-manager/pkg/apis/machine
// +k8s:openapi-gen=true
// +k8s:defaulter-gen=TypeMeta
// +groupName=machine.sapcloud.io
// +kubebuilder:object:generate=true

//go:generate crd-ref-docs --source-path=. --config=../../../../hack/api-reference/config.yaml --renderer=markdown --templates-dir=../../../../hack/api-reference/template --log-level=ERROR --output-path=../../../../docs/documents/apis.md

// Package v1alpha1 is a version of the API.
package v1alpha1
