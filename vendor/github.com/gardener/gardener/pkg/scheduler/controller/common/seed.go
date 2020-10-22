// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package common

import (
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
)

// VerifySeedReadiness verifies whether the seed is ready.
func VerifySeedReadiness(seed *gardencorev1beta1.Seed) bool {
	if cond := gardencorev1beta1helper.GetCondition(seed.Status.Conditions, gardencorev1beta1.SeedBootstrapped); cond == nil || cond.Status != gardencorev1beta1.ConditionTrue {
		return false
	}
	if cond := gardencorev1beta1helper.GetCondition(seed.Status.Conditions, gardencorev1beta1.SeedGardenletReady); cond == nil || cond.Status != gardencorev1beta1.ConditionTrue {
		return false
	}
	return true
}
