/*
Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved.

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

package permits

import (
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("permit", func() {
	var (
		pg *permitGiver
	)
	const (
		key1 = "key1"
		key2 = "key2"
	)
	Describe("#RegisterPermits", func() {
		BeforeEach(func() {
			pg = NewPermitGiver(5*time.Second, 1*time.Second).(*permitGiver)
		})

		AfterEach(func() {
			// to stop the janitor goroutine
			pg.Close()
		})

		It("should register permit for given key", func() {
			Expect(pg.isPermitAllocated(key1)).To(BeFalse())
			pg.RegisterPermits(key1, 1)
			Expect(pg.isPermitAllocated(key1)).To(BeTrue())
		})
	})
})
