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
			if !pg.isClosed() {
				pg.Close()
			}
		})

		It("should register permit for given key", func() {
			Expect(pg.isPermitAllocated(key1)).To(BeFalse())
			pg.RegisterPermits(key1, 1)
			Expect(pg.isPermitAllocated(key1)).To(BeTrue())
		})
		It("Should not register if the pg is closed", func() {
			Expect(pg.isPermitAllocated(key1)).To(BeFalse())
			pg.Close()
			pg.RegisterPermits(key1, 1)
			Expect(pg.isPermitAllocated(key1)).To(BeFalse())
		})
		It("should not register if already registered", func() {
			Expect(pg.isPermitAllocated(key1)).To(BeFalse())
			pg.RegisterPermits(key1, 1)
			Expect(pg.isPermitAllocated(key1)).To(BeTrue())
			pg.RegisterPermits(key1, 2)
			if obj, ok := pg.keyPermitsMap.Load(key1); ok {
				p := obj.(permit)
				Expect(cap(p.c)).To(Equal(1))
			}
			Expect(pg.isPermitAllocated(key1)).To(BeTrue())
		})
	})

	Describe("#DeletePermits", func() {
		BeforeEach(func() {
			pg = NewPermitGiver(5*time.Second, 1*time.Second).(*permitGiver)
			pg.RegisterPermits(key1, 1)
		})
		AfterEach(func() {
			if !pg.isClosed() {
				pg.Close()
			}
		})
		It("should delete permit if there is a permit related to that key", func() {
			Expect(pg.isPermitAllocated(key1)).To(BeTrue())
			pg.DeletePermits(key1)
			Expect(pg.isPermitAllocated(key1)).To(BeFalse())
		})
		It("nothing changes if permit is not there already", func() {
			Expect(pg.isPermitAllocated(key2)).To(BeFalse())
			pg.DeletePermits(key2)
			Expect(pg.isPermitAllocated(key2)).To(BeFalse())
		})
		It("do nothing if already closed", func() {
			Expect(pg.isPermitAllocated(key1)).To(BeTrue())
			pg.Close()
			pg.DeletePermits(key1)
			Expect(pg.isPermitAllocated(key1)).To(BeFalse())
		})
	})

	Describe("#TryPermit", func() {
		BeforeEach(func() {
			pg = NewPermitGiver(5*time.Second, 1*time.Second).(*permitGiver)
			pg.RegisterPermits(key1, 1)
		})
		It("should return false if permitGiver is closed", func() {
			pg.Close()
			Expect(pg.TryPermit(key1, 1*time.Second)).To(BeFalse())
		})
		It("should return false if there is no permit for that key", func() {
			Expect(pg.TryPermit(key2, 1*time.Second)).To(BeFalse())
		})
		It("should return true if permit is available", func() {
			Expect(pg.TryPermit(key1, 1*time.Second)).To(BeTrue())
		})
		//TODO: few more things to test is, if while trying to give permit the pg got closed, or the timeout occured,
		//also if the time got written there correctly
	})

	Describe("#Release Permit", func() {
		BeforeEach(func() {
			pg = NewPermitGiver(5*time.Second, 1*time.Second).(*permitGiver)
			pg.RegisterPermits(key1, 1)
			pg.TryPermit(key1, 5*time.Second)
		})
		AfterEach(func() {
			pg.Close()
		})
		It("should release the key1 permit", func() {
			Expect(pg.isPermitAcquired(key1)).To(BeTrue())
			pg.ReleasePermit(key1)
			Expect(pg.isPermitAcquired(key1)).To(BeFalse())
		})
		It("should not release if its not acquired already", func() {
			Expect(pg.isPermitAcquired(key2)).To(BeFalse())
			pg.ReleasePermit(key2)
			//TODO: also need to check if it looged that there is no permit for this key
			Expect(pg.isPermitAcquired(key2)).To(BeFalse())
		})
	})

	Describe("#isClose", func() {
		BeforeEach(func() {
			pg = NewPermitGiver(5*time.Second, 1*time.Second).(*permitGiver)
		})
		It("return true if closed", func() {
			pg.Close()
			Expect(pg.isClosed()).To(BeTrue())
		})
		It("return false if not closed", func() {
			Expect(pg.isClosed()).To(BeFalse())
			pg.Close()
		})
	})

	Describe("#Close", func() {
		BeforeEach(func() {
			pg = NewPermitGiver(5*time.Second, 1*time.Second).(*permitGiver)
			pg.RegisterPermits(key1, 1)
		})
		It("closed the PermitGiver", func() {
			Expect(pg.isClosed()).To(BeFalse())
			pg.Close()
			Expect(pg.isClosed()).To(BeTrue())
		})
	})

	Describe("#NewPermitGiver", func() {
		BeforeEach(func() {
			pg = NewPermitGiver(5*time.Second, 1*time.Second).(*permitGiver)
			pg.RegisterPermits(key1, 1)
		})
		AfterEach(func() {
			pg.Close()
		})
		It("should cleanup if permit stale for more than 5 sec", func() {
			pg.TryPermit(key1, 1*time.Second) //updates the lastAcquiredTime
			pg.ReleasePermit(key1)
			Expect(pg.isPermitAllocated(key1)).To(BeTrue())
			time.Sleep(7 * time.Second)
			Expect(pg.isPermitAllocated(key1)).To(BeFalse())
		})
		It("should not cleanup if permit stale for less than 5 sec", func() {
			pg.TryPermit(key1, 1*time.Second) //updates the lastAcquiredTime
			pg.ReleasePermit(key1)
			Expect(pg.isPermitAllocated(key1)).To(BeTrue())
			time.Sleep(4 * time.Second)
			Expect(pg.isPermitAllocated(key1)).To(BeTrue())
		})
		It("Should return an open permit giver", func() {
			Expect(pg.isClosed()).To(BeFalse())
		})
	})

})
