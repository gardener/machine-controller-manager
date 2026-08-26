// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package time_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestTime(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Time Suite")
}
