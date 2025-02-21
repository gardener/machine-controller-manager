// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"

	"github.com/gardener/machine-controller-manager/pkg/apis/machine"
	. "github.com/gardener/machine-controller-manager/pkg/apis/machine/validation"
)

var _ = Describe("MachineDeployment API Validation", func() {
	var (
		machineDeployment *machine.MachineDeployment
	)

	BeforeEach(func() {
		machineDeployment = &machine.MachineDeployment{
			Spec: machine.MachineDeploymentSpec{
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"key": "value",
					},
				},
				Template: machine.MachineTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"key": "value",
						},
					},
					Spec: machine.MachineSpec{
						Class: machine.ClassSpec{
							Name: "test",
							Kind: "test",
						},
					},
				},
				Strategy: machine.MachineDeploymentStrategy{
					Type: machine.RollingUpdateMachineDeploymentStrategyType,
					RollingUpdate: &machine.RollingUpdateMachineDeployment{
						UpdateConfiguration: machine.UpdateConfiguration{
							MaxUnavailable: ptr.To(intstr.FromInt32(1)),
							MaxSurge:       ptr.To(intstr.FromInt32(1)),
						},
					},
				},
			},
		}
	})

	Describe("ValidateMachineDeployment", func() {
		It("should not return error if machine deployment is valid", func() {
			Expect(ValidateMachineDeployment(machineDeployment)).To(BeEmpty())
		})

		It("should return error if selector doesn't match MachineTemplateSpec labels", func() {
			machineDeployment.Spec.Selector.MatchLabels = map[string]string{
				"key": "value1",
			}

			Expect(ValidateMachineDeployment(machineDeployment)).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(field.ErrorTypeRequired),
				"Field":  Equal("spec.selector.matchLabels"),
				"Detail": Equal("is not matching with spec.template.metadata.labels"),
			}))))
		})

		It("should return error if machine template spec class kind is not defined", func() {
			machineDeployment.Spec.Template.Spec.Class.Kind = ""

			Expect(ValidateMachineDeployment(machineDeployment)).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(field.ErrorTypeRequired),
				"Field":  Equal("spec.template.spec.class.kind"),
				"Detail": Equal("Kind is required"),
			}))))
		})

		It("should return error if machine template spec class name is not defined", func() {
			machineDeployment.Spec.Template.Spec.Class.Name = ""

			Expect(ValidateMachineDeployment(machineDeployment)).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(field.ErrorTypeRequired),
				"Field":  Equal("spec.template.spec.class.name"),
				"Detail": Equal("Name is required"),
			}))))
		})

		Context("Validate UpdateStrategy of machine deployment", func() {
			It("should return error if strategy type is not valid", func() {
				machineDeployment.Spec.Strategy.Type = "invalid"

				Expect(ValidateMachineDeployment(machineDeployment)).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("spec.strategy.type"),
					"Detail": Equal("strategy type must be one of [InPlaceUpdate Recreate RollingUpdate]"),
				}))))
			})

			Context("Validate RollingUpdate strategy", func() {
				It("should not return error if RollingUpdate strategy is valid", func() {
					Expect(ValidateMachineDeployment(machineDeployment)).To(BeEmpty())
				})

				It("should return error if MaxUnavailable is unspecified", func() {
					machineDeployment.Spec.Strategy.RollingUpdate.UpdateConfiguration.MaxUnavailable = nil

					Expect(ValidateMachineDeployment(machineDeployment)).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeRequired),
						"Field":  Equal("spec.strategy.rollingUpdate.maxUnavailable"),
						"Detail": Equal("unable to convert maxUnavailable to int32"),
					}))))
				})

				It("should return error if MaxSurge is unspecified", func() {
					machineDeployment.Spec.Strategy.RollingUpdate.UpdateConfiguration.MaxSurge = nil

					Expect(ValidateMachineDeployment(machineDeployment)).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeRequired),
						"Field":  Equal("spec.strategy.rollingUpdate.maxSurge"),
						"Detail": Equal("unable to convert maxSurge to int32"),
					}))))
				})
			})

			Context("Validate InPlaceUpdate strategy", func() {
				BeforeEach(func() {
					machineDeployment.Spec.Strategy = machine.MachineDeploymentStrategy{
						Type: machine.InPlaceUpdateMachineDeploymentStrategyType,
						InPlaceUpdate: &machine.InPlaceUpdateMachineDeployment{
							UpdateConfiguration: machine.UpdateConfiguration{
								MaxUnavailable: ptr.To(intstr.FromInt32(1)),
								MaxSurge:       ptr.To(intstr.FromInt32(1)),
							},
							OrchestrationType: machine.OrchestrationTypeAuto,
						},
					}
				})

				It("should not return error if InPlaceUpdate strategy is valid", func() {
					Expect(ValidateMachineDeployment(machineDeployment)).To(BeEmpty())
				})

				It("should return error if MaxUnavailable is unspecified", func() {
					machineDeployment.Spec.Strategy.InPlaceUpdate.UpdateConfiguration.MaxUnavailable = nil

					Expect(ValidateMachineDeployment(machineDeployment)).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeRequired),
						"Field":  Equal("spec.strategy.inPlaceUpdate.maxUnavailable"),
						"Detail": Equal("unable to convert maxUnavailable to int32"),
					}))))
				})

				It("should return error if MaxSurge is unspecified", func() {
					machineDeployment.Spec.Strategy.InPlaceUpdate.UpdateConfiguration.MaxSurge = nil

					Expect(ValidateMachineDeployment(machineDeployment)).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeRequired),
						"Field":  Equal("spec.strategy.inPlaceUpdate.maxSurge"),
						"Detail": Equal("unable to convert maxSurge to int32"),
					}))))
				})

				It("should return error if OrchestrationType is not valid", func() {
					machineDeployment.Spec.Strategy.InPlaceUpdate.OrchestrationType = "invalid"

					Expect(ValidateMachineDeployment(machineDeployment)).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.strategy.inPlaceUpdate.orchestrationType"),
						"Detail": Equal("orchestrationType must be either Auto or Manual"),
					}))))
				})
			})
		})
	})
})
