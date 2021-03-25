/*
Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

This package makes use of https://github.com/cenkalti/backoff project to
implement the backoff mechanism.
*/

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
