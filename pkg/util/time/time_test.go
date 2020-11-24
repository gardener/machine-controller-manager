/*
Copyright (c) 2017 SAP SE or an SAP affiliate company. All rights reserved.

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
package time

import (
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("time", func() {
	Describe("#hasTimeOutOccurred", func() {
		type setup struct {
			timeStamp metav1.Time
			period    time.Duration
		}
		type action struct {
		}
		type expect struct {
			timeOutOccurred bool
		}
		type data struct {
			setup  setup
			action action
			expect expect
		}
		DescribeTable("##TimeOut scenarios",
			func(data *data) {
				timeOutOccurred := HasTimeOutOccurred(data.setup.timeStamp, data.setup.period)
				Expect(timeOutOccurred).To(Equal(data.expect.timeOutOccurred))
			},
			Entry("Time stamp is one hour ago and period is 30mins", &data{
				setup: setup{
					timeStamp: metav1.Time{Time: time.Now().Add(-time.Hour)},
					period:    30 * time.Minute,
				},
				expect: expect{
					timeOutOccurred: true,
				},
			}),
			Entry("Time stamp is one hour ago and period is 90mins", &data{
				setup: setup{
					timeStamp: metav1.Time{Time: time.Now().Add(-time.Hour)},
					period:    90 * time.Minute,
				},
				expect: expect{
					timeOutOccurred: false,
				},
			}),
			Entry("Time stamp is now and period is 5mins", &data{
				setup: setup{
					timeStamp: metav1.Time{Time: time.Now()},
					period:    5 * time.Minute,
				},
				expect: expect{
					timeOutOccurred: false,
				},
			}),
		)
	})
})
