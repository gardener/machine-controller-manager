package machineutils

import (
	"flag"
	machinev1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"testing"
	"time"
)

func TestMachineUtilsSuite(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Machine Utils Suite")
}

var _ = BeforeSuite(func() {
	klog.SetOutput(GinkgoWriter)
	//for filtering out warning logs. Reflector short watch warning logs won't print now
	klog.LogToStderr(false)
	flags := &flag.FlagSet{}
	klog.InitFlags(flags)
	Expect(flags.Set("v", "10")).To(Succeed())

	DeferCleanup(klog.Flush)
})

var _ = Describe("utils.go", func() {
	Describe("#isFailedMachineCandidateForPreservation", func() {

		type setup struct {
			preserveExpiryTime *metav1.Time
			annotationValue    string
		}
		type expect struct {
			result bool
		}
		type testCase struct {
			setup  setup
			expect expect
		}
		DescribeTable("isFailedMachineCandidateForPreservation test cases", func(tc testCase) {
			machine := machinev1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-machine",
					Namespace: "default",
					Annotations: map[string]string{
						PreserveMachineAnnotationKey: tc.setup.annotationValue,
					},
				},
				Status: machinev1.MachineStatus{
					CurrentStatus: machinev1.CurrentStatus{
						Phase:              machinev1.MachineFailed,
						PreserveExpiryTime: tc.setup.preserveExpiryTime,
					},
				},
			}
			result := IsFailedMachineCandidateForPreservation(&machine)
			Expect(result).To(Equal(tc.expect.result))
		},
			Entry("should return true if preserve expiry time is in the future", testCase{
				setup: setup{
					preserveExpiryTime: &metav1.Time{Time: metav1.Now().Add(1 * time.Hour)},
					annotationValue:    PreserveMachineAnnotationValueNow,
				},
				expect: expect{
					result: true,
				},
			}),
			Entry("should return false if machine is annotated with preserve=false", testCase{
				setup: setup{
					annotationValue: PreserveMachineAnnotationValueFalse,
				},
				expect: expect{
					result: false,
				},
			}),
			Entry("should return true if machine is annotated with preserve=now", testCase{
				setup: setup{
					annotationValue: PreserveMachineAnnotationValueNow,
				},
				expect: expect{
					result: true,
				},
			}),
			Entry("should return true if machine is annotated with preserve=when-failed", testCase{
				setup: setup{
					annotationValue: PreserveMachineAnnotationValueWhenFailed,
				},
				expect: expect{
					result: true,
				},
			}),
			Entry("should return false if preservation has timed out", testCase{
				setup: setup{
					preserveExpiryTime: &metav1.Time{Time: metav1.Now().Add(-1 * time.Second)},
					annotationValue:    PreserveMachineAnnotationValueNow,
				},
				expect: expect{
					result: false,
				},
			}),
		)
	})
	Describe("#PreserveAnnotationsChanged", func() {
		type setup struct {
			oldAnnotations map[string]string
			newAnnotations map[string]string
		}
		type expect struct {
			result bool
		}
		type testCase struct {
			setup  setup
			expect expect
		}
		DescribeTable("PreserveAnnotationsChanged test cases", func(tc testCase) {

			result := PreserveAnnotationsChanged(tc.setup.oldAnnotations, tc.setup.newAnnotations)
			Expect(result).To(Equal(tc.expect.result))
		},
			Entry("should return true if preserve annotation added for the first time", testCase{
				setup: setup{
					oldAnnotations: map[string]string{},
					newAnnotations: map[string]string{
						PreserveMachineAnnotationKey: PreserveMachineAnnotationValueNow,
					},
				},
				expect: expect{
					result: true,
				},
			}),
			Entry("should return true if preserve annotation is removed", testCase{
				setup: setup{
					oldAnnotations: map[string]string{
						PreserveMachineAnnotationKey: PreserveMachineAnnotationValueNow,
					},
					newAnnotations: map[string]string{},
				},
				expect: expect{
					result: true,
				},
			}),
			Entry("should return true if preserve annotation value is changed", testCase{
				setup: setup{
					oldAnnotations: map[string]string{
						PreserveMachineAnnotationKey: PreserveMachineAnnotationValueNow,
					},
					newAnnotations: map[string]string{
						PreserveMachineAnnotationKey: PreserveMachineAnnotationValueWhenFailed,
					},
				},
				expect: expect{
					result: true,
				},
			}),
			Entry("should return false if preserve annotation is unchanged", testCase{
				setup: setup{
					oldAnnotations: map[string]string{
						PreserveMachineAnnotationKey: PreserveMachineAnnotationValueNow,
					},
					newAnnotations: map[string]string{
						PreserveMachineAnnotationKey: PreserveMachineAnnotationValueNow,
					},
				},
				expect: expect{
					result: false,
				},
			}),
		)
	})
})
