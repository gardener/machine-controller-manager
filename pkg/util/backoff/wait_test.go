// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package backoff

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var (
	invokationCount = 0
)

var _ = Describe("#wait", func() {

	type setup struct {
		initialInterval time.Duration
		maxInterval     time.Duration
		maxElapsedTime  time.Duration
	}
	type action struct {
		operation func() error
	}
	type expect struct {
		err error
	}
	type data struct {
		setup  setup
		action action
		expect expect
	}

	DescribeTable("##WaitUntil",
		func(data *data) {
			invokationCount = 0

			ctx := context.Background()
			err := WaitUntil(
				ctx,
				data.setup.initialInterval,
				data.setup.maxInterval,
				data.setup.maxElapsedTime,
				data.action.operation,
			)

			if err == nil {
				Expect(data.expect.err).To(BeNil())
			} else {
				Expect(err).Should(Equal(data.expect.err))
			}
		},

		Entry("Postive test case", &data{
			setup: setup{
				initialInterval: 10 * time.Microsecond,
				maxInterval:     100 * time.Microsecond,
				maxElapsedTime:  1000 * time.Microsecond,
			},
			action: action{
				operation: func() error {
					return nil
				},
			},
			expect: expect{
				err: nil,
			},
		}),

		Entry("Negative error test case", &data{
			setup: setup{
				initialInterval: 10 * time.Microsecond,
				maxInterval:     100 * time.Microsecond,
				maxElapsedTime:  1000 * time.Microsecond,
			},
			action: action{
				operation: func() error {
					return fmt.Errorf("test")
				},
			},
			expect: expect{
				err: fmt.Errorf("test"),
			},
		}),

		Entry("Test case for 30ms timeout", &data{
			setup: setup{
				initialInterval: 1 * time.Millisecond,
				maxInterval:     10 * time.Millisecond,
				maxElapsedTime:  30 * time.Millisecond,
			},
			action: action{
				operation: func() error {
					invokationCount += 1

					if invokationCount > 10 {
						return nil
					}

					return fmt.Errorf("timeout occurred")
				},
			},
			expect: expect{
				err: fmt.Errorf("timeout occurred"),
			},
		}),

		Entry("Test case for successful function call return after 4 retries", &data{
			setup: setup{
				initialInterval: 1 * time.Millisecond,
				maxInterval:     10 * time.Millisecond,
				maxElapsedTime:  30 * time.Millisecond,
			},
			action: action{
				operation: func() error {
					invokationCount += 1
					if invokationCount > 4 {
						return nil
					}

					return fmt.Errorf("timeout occurred")
				},
			},
			expect: expect{
				err: nil,
			},
		}),
	)
})
