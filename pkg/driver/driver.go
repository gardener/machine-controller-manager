/*
Copyright 2017 The Gardener Authors.

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
package driver

import (
	v1alpha1 "code.sapcloud.io/kubernetes/node-controller-manager/pkg/apis/node/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

//Driver is the common interface for creation/deletion of the VMs over different cloud-providers.
type Driver interface {
	Create() (string, string, error)
	Delete() error
	GetExisting() (string, error)
}

func NewDriver(instanceID string, class *v1alpha1.AWSInstanceClass, secretRef *corev1.Secret, classKind string) Driver {

	switch classKind {
	case "AWSInstanceClass":
		return &AWSDriver{
			AWSInstanceClass: 	class,
			CloudConfig: 		secretRef,
			UserData: 			string(secretRef.Data["userData"]),
			InstanceId: 		instanceID,
		}
	}
	return NewFakeDriver(
		func() (string, string, error) {
			return "fake", "fake_ip", nil
		},
		func() error {
			return nil
		},
		func() (string, error) {
			return "fake", nil
		})
}
