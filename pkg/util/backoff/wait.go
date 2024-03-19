// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package backoff

import (
	"context"
	"time"

	backoff "github.com/cenkalti/backoff/v4"
)

// WaitUntil is a helper method that waits until the operation func()
// returns nil successfully (or) times out after maxElapsedTime.
// initialInterval and maxIntervals let's you specify the intervals
// for exponential backoff
func WaitUntil(
	ctx context.Context,
	initialInterval time.Duration,
	maxInterval time.Duration,
	maxElapsedTime time.Duration,
	operation func() error,
) error {

	exponentialBackoff := backoff.NewExponentialBackOff()
	exponentialBackoff.InitialInterval = initialInterval
	exponentialBackoff.MaxInterval = maxInterval
	exponentialBackoff.MaxElapsedTime = maxElapsedTime
	exponentialBackoff.RandomizationFactor = 0
	exponentialBackoff.Multiplier = 2

	b := backoff.WithContext(exponentialBackoff, ctx)

	err := backoff.Retry(operation, b)
	if err != nil {
		return err
	}

	return nil
}
