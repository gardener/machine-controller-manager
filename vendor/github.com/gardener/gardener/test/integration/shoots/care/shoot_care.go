// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

/**
	Overview
		- Tests the Care Controller in the Gardenlet

	Prerequisite
		- Shoot Cluster with  Condition "APIServerAvailable" equals true

	Test: Scale down API Server deployment of the Shoot in the Seed
	Expected Output
		- Shoot Condition "APIServerAvailable" becomes unhealthy
 **/

package care

import (
	"context"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardencorev1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/test/framework"
	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
)

const (
	timeout = 10 * time.Minute
)

var _ = ginkgo.Describe("Shoot Care testing", func() {
	var (
		f            = framework.NewShootFramework(nil)
		origReplicas *int32
		err          error
	)

	f.Beta().Serial().CIt("Should observe failed health condition in the Shoot when scaling down the API Server of the Shoot", func(ctx context.Context) {
		cond := helper.GetCondition(f.Shoot.Status.Conditions, gardencorev1beta1.ShootAPIServerAvailable)
		gomega.Expect(cond).ToNot(gomega.BeNil())
		gomega.Expect(cond.Status).To(gomega.Equal(gardencorev1beta1.ConditionTrue))

		zero := int32(0)
		origReplicas, err = framework.ScaleDeployment(timeout, f.SeedClient.DirectClient(), &zero, gardencorev1beta1constants.DeploymentNameKubeAPIServer, f.ShootSeedNamespace())
		gomega.Expect(err).ToNot(gomega.HaveOccurred())

		// wait for unhealthy condition
		err = f.WaitForShootCondition(ctx, 20*time.Second, 5*time.Minute, gardencorev1beta1.ShootAPIServerAvailable, gardencorev1beta1.ConditionFalse)
		gomega.Expect(err).ToNot(gomega.HaveOccurred())
	}, timeout, framework.WithCAfterTest(func(ctx context.Context) {
		if origReplicas != nil {
			f.Logger.Infof("Test cleanup: Scale API Server to '%d' replicas again", *origReplicas)
			origReplicas, err = framework.ScaleDeployment(timeout, f.SeedClient.DirectClient(), origReplicas, gardencorev1beta1constants.DeploymentNameKubeAPIServer, f.ShootSeedNamespace())
			gomega.Expect(err).ToNot(gomega.HaveOccurred())

			// wait for healthy condition
			f.Logger.Infof("Test cleanup: wait for Shoot Health condition '%s' to become healthy again", gardencorev1beta1.ShootAPIServerAvailable)
			err = f.WaitForShootCondition(ctx, 20*time.Second, 5*time.Minute, gardencorev1beta1.ShootAPIServerAvailable, gardencorev1beta1.ConditionTrue)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())

			f.Logger.Info("Test cleanup successful")
		}
	}, timeout))
})
