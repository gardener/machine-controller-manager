package annotations_test

import (
	"flag"
	"io"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/klog/v2"
)

func TestAnnotations(t *testing.T) {
	klog.SetOutput(io.Discard)
	flags := &flag.FlagSet{}
	klog.InitFlags(flags)
	flags.Set("logtostderr", "false")
	RegisterFailHandler(Fail)
	RunSpecs(t, "Annotations Suite")
}
