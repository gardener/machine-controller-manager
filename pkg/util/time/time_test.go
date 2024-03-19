// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package time

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("time", func() {
	Describe("#hasTimeOutOccurred", func() {
		type setup struct {
			timeStamp metav1.Time
			period    time.Duration
		}
		type expect struct {
			timeOutOccurred bool
		}
		type data struct {
			setup  setup
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
