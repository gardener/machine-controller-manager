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

package manager

import (
	"context"

	resourcesv1alpha1 "github.com/gardener/gardener-resource-manager/pkg/apis/resources/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type ManagedResource struct {
	client   client.Client
	resource *resourcesv1alpha1.ManagedResource
}

func NewManagedResource(client client.Client) *ManagedResource {
	return &ManagedResource{
		client:   client,
		resource: &resourcesv1alpha1.ManagedResource{},
	}
}

func (m *ManagedResource) WithNamespacedName(namespace, name string) *ManagedResource {
	m.resource.Namespace = namespace
	m.resource.Name = name
	return m
}

func (m *ManagedResource) WithLabels(labels map[string]string) *ManagedResource {
	m.resource.Labels = labels
	return m
}

func (m *ManagedResource) WithAnnotations(annotations map[string]string) *ManagedResource {
	m.resource.Annotations = annotations
	return m
}

func (m *ManagedResource) WithClass(name string) *ManagedResource {
	if name == "" {
		m.resource.Spec.Class = nil
	} else {
		m.resource.Spec.Class = &name
	}
	return m
}

func (m *ManagedResource) WithSecretRef(secretRefName string) *ManagedResource {
	m.resource.Spec.SecretRefs = append(m.resource.Spec.SecretRefs, corev1.LocalObjectReference{Name: secretRefName})
	return m
}

func (m *ManagedResource) WithSecretRefs(secretRefs []corev1.LocalObjectReference) *ManagedResource {
	m.resource.Spec.SecretRefs = append(m.resource.Spec.SecretRefs, secretRefs...)
	return m
}

func (m *ManagedResource) WithInjectedLabels(labelsToInject map[string]string) *ManagedResource {
	m.resource.Spec.InjectLabels = labelsToInject
	return m
}

func (m *ManagedResource) ForceOverwriteAnnotations(v bool) *ManagedResource {
	m.resource.Spec.ForceOverwriteAnnotations = &v
	return m
}

func (m *ManagedResource) ForceOverwriteLabels(v bool) *ManagedResource {
	m.resource.Spec.ForceOverwriteLabels = &v
	return m
}

func (m *ManagedResource) KeepObjects(v bool) *ManagedResource {
	m.resource.Spec.KeepObjects = &v
	return m
}

func (m *ManagedResource) DeletePersistentVolumeClaims(v bool) *ManagedResource {
	m.resource.Spec.DeletePersistentVolumeClaims = &v
	return m
}

func (m *ManagedResource) Reconcile(ctx context.Context) error {
	resource := &resourcesv1alpha1.ManagedResource{
		ObjectMeta: metav1.ObjectMeta{Name: m.resource.Name, Namespace: m.resource.Namespace},
	}

	_, err := controllerutil.CreateOrUpdate(ctx, m.client, resource, func() error {
		resource.Labels = m.resource.Labels
		resource.Annotations = m.resource.Annotations
		resource.Spec = m.resource.Spec
		return nil
	})
	return err
}

func (m *ManagedResource) Delete(ctx context.Context) error {
	if err := m.client.Delete(ctx, m.resource); err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	return nil
}
