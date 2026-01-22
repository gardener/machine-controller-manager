package machineutils

import (
	"flag"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/klog/v2"
	"testing"
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
