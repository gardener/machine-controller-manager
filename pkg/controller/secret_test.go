/*
Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved.

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
package controller

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var _ = Describe("secret", func() {

	//TODO: This method has dependency on generic-machineclass. Implement later.
	Describe("#reconcileClusterSecret", func() {})

	Describe("#addSecretFinalizers", func() {
		var (
			testSecret *corev1.Secret
		)

		BeforeEach(func() {
			testSecret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "Secret-test",
					Namespace: testNamespace,
				},
			}
		})

		// Testcase: It should add finalizer on Secret.
		It("should add finalizer on Secret.", func() {
			stop := make(chan struct{})
			defer close(stop)

			objects := []runtime.Object{}
			objects = append(objects, testSecret)
			c, trackers := createController(stop, testNamespace, nil, objects, nil)
			defer trackers.Stop()
			waitForCacheSync(stop, c)

			c.addSecretFinalizers(testSecret)

			waitForCacheSync(stop, c)
			expectedSecret, _ := c.controlCoreClient.Core().Secrets(testSecret.Namespace).Get(testSecret.Name, metav1.GetOptions{})

			Expect(expectedSecret.Finalizers).To(HaveLen(1))
			Expect(expectedSecret.Finalizers).To(ContainElement(DeleteFinalizerName))
		})
	})

	Describe("#deleteSecretFinalizers", func() {
		var (
			testSecret *corev1.Secret
			finalizers []string
		)

		BeforeEach(func() {
			testSecret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "Secret-test",
					Namespace: testNamespace,
				},
			}
			finalizers = []string{DeleteFinalizerName}

		})

		// Testcase: It should delete the finalizer from Secret.
		It("should delete the finalizer from Secret.", func() {
			stop := make(chan struct{})
			defer close(stop)

			objects := []runtime.Object{}
			objects = append(objects, testSecret)
			c, trackers := createController(stop, testNamespace, nil, objects, nil)
			defer trackers.Stop()
			waitForCacheSync(stop, c)

			testSecret, _ := c.controlCoreClient.Core().Secrets(testSecret.Namespace).Get(testSecret.Name, metav1.GetOptions{})

			testSecret.Finalizers = finalizers
			Expect(testSecret.Finalizers).Should(Not(BeEmpty()))

			c.deleteSecretFinalizers(testSecret)

			waitForCacheSync(stop, c)

			expectedSecret, _ := c.controlCoreClient.Core().Secrets(testSecret.Namespace).Get(testSecret.Name, metav1.GetOptions{})

			Expect(expectedSecret.Finalizers).Should(HaveLen(0))
		})
	})

	Describe("#updateSecretFinalizers", func() {
		var (
			testSecret *corev1.Secret
			finalizers []string
		)

		BeforeEach(func() {
			testSecret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "Secret-test",
					Namespace: testNamespace,
				},
			}

			finalizers = []string{"finalizer1", "finalizer2"}

		})

		// Testcase: It should update the finalizer on Secret.
		It("should update the finalizer on Secret.", func() {
			stop := make(chan struct{})
			defer close(stop)

			objects := []runtime.Object{}
			objects = append(objects, testSecret)
			c, trackers := createController(stop, testNamespace, nil, objects, nil)
			defer trackers.Stop()
			waitForCacheSync(stop, c)

			c.updateSecretFinalizers(testSecret, finalizers)

			waitForCacheSync(stop, c)

			testSecret, _ := c.controlCoreClient.Core().Secrets(testSecret.Namespace).Get(testSecret.Name, metav1.GetOptions{})

			Expect(testSecret.Finalizers).To(Equal(finalizers))
		})
	})

})
