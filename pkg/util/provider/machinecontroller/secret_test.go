// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
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
			c, trackers := createController(stop, testNamespace, nil, objects, nil, nil)
			defer trackers.Stop()
			waitForCacheSync(stop, c)

			c.addSecretFinalizers(context.TODO(), testSecret)

			waitForCacheSync(stop, c)
			expectedSecret, _ := c.controlCoreClient.CoreV1().Secrets(testSecret.Namespace).Get(context.TODO(), testSecret.Name, metav1.GetOptions{})

			Expect(expectedSecret.Finalizers).To(HaveLen(1))
			Expect(expectedSecret.Finalizers).To(ContainElement(MCFinalizerName))
		})
	})

	Describe("#deleteSecretFinalizers", func() {
		var (
			rightFinalizers = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "Secret-test",
					Namespace:  testNamespace,
					Finalizers: []string{MCFinalizerName},
				},
			}
			wrongFinalizers = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "Secret-test",
					Namespace:  testNamespace,
					Finalizers: []string{MCMFinalizerName},
				},
			}
		)

		// Testcase: It should delete the finalizer from Secret.
		It("should delete the finalizer from Secret.", func() {
			stop := make(chan struct{})
			defer close(stop)

			objects := []runtime.Object{}
			objects = append(objects, rightFinalizers)
			c, trackers := createController(stop, testNamespace, nil, objects, nil, nil)
			defer trackers.Stop()
			waitForCacheSync(stop, c)

			testSecret, _ := c.controlCoreClient.CoreV1().Secrets(rightFinalizers.Namespace).Get(context.TODO(), rightFinalizers.Name, metav1.GetOptions{})
			Expect(testSecret.Finalizers).Should(Not(BeEmpty()))

			c.deleteSecretFinalizers(context.TODO(), testSecret)

			waitForCacheSync(stop, c)

			expectedSecret, _ := c.controlCoreClient.CoreV1().Secrets(rightFinalizers.Namespace).Get(context.TODO(), rightFinalizers.Name, metav1.GetOptions{})
			Expect(expectedSecret.Finalizers).Should(HaveLen(0))
		})
		It("should not be able delete the wrong finalizer from Secret.", func() {
			stop := make(chan struct{})
			defer close(stop)

			objects := []runtime.Object{}
			objects = append(objects, wrongFinalizers)
			c, trackers := createController(stop, testNamespace, nil, objects, nil, nil)
			defer trackers.Stop()
			waitForCacheSync(stop, c)

			testSecret, _ := c.controlCoreClient.CoreV1().Secrets(wrongFinalizers.Namespace).Get(context.TODO(), wrongFinalizers.Name, metav1.GetOptions{})
			Expect(testSecret.Finalizers).Should(Not(BeEmpty()))

			c.deleteSecretFinalizers(context.TODO(), testSecret)

			waitForCacheSync(stop, c)

			expectedSecret, _ := c.controlCoreClient.CoreV1().Secrets(wrongFinalizers.Namespace).Get(context.TODO(), wrongFinalizers.Name, metav1.GetOptions{})
			Expect(expectedSecret.Finalizers).Should(HaveLen(1))
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
			c, trackers := createController(stop, testNamespace, nil, objects, nil, nil)
			defer trackers.Stop()
			waitForCacheSync(stop, c)

			c.updateSecretFinalizers(context.TODO(), testSecret, finalizers)

			waitForCacheSync(stop, c)

			testSecret, _ := c.controlCoreClient.CoreV1().Secrets(testSecret.Namespace).Get(context.TODO(), testSecret.Name, metav1.GetOptions{})

			Expect(testSecret.Finalizers).To(Equal(finalizers))
		})
	})
})
