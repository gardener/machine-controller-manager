package driver

import "testing"
import . "github.com/onsi/ginkgo"
import . "github.com/onsi/gomega"

func TestDriverSuite(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Driver Suite")
}
