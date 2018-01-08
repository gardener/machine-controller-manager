/*
Copyright 2017 The Gardener Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package validation

import (
	"github.com/gardener/node-controller-manager/pkg/apis/node"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// ValidateInstanceDeployment and returns a list of errors.
func ValidateInstanceDeployment(instanceDeployment *node.InstanceDeployment) field.ErrorList {
	return internalValidateInstanceDeployment(instanceDeployment)
}

func internalValidateInstanceDeployment(instanceDeployment *node.InstanceDeployment) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, validateInstanceDeploymentSpec(&instanceDeployment.Spec, field.NewPath("spec"))...)
	return allErrs
}

func validateInstanceDeploymentSpec(spec *node.InstanceDeploymentSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if spec.Replicas < 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("replicas"), "Replicas has to be a whole number"))
	}

	if spec.Strategy.Type != "RollingUpdate" && spec.Strategy.Type != "Recreate" {
		allErrs = append(allErrs, field.Required(fldPath.Child("strategy.type"), "Type can either be RollingUpdate or Recreate"))
	}

	for k, v := range spec.Selector.MatchLabels {
		if spec.Template.Labels[k] != v {
			allErrs = append(allErrs, field.Required(fldPath.Child("selector.matchLabels"), "is not matching with spec.template.metadata.labels"))
			break
		}
	}
	
	allErrs = append(allErrs, validateClassReference(&spec.Template.Spec.Class, field.NewPath("spec.template.spec.class"))...)
	return allErrs
}