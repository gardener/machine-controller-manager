/*
Copyright 2016 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

This file was copied and modified from the kubernetes/kubernetes project
https://github.com/kubernetes/kubernetes/release-1.8/pkg/controller/deployment/util/pod_util.go

Modifications Copyright 2017 The Gardener Authors.
*/

package controller

import (
	"github.com/golang/glog"

	errorsutil "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/util/retry"
	"github.com/gardener/node-controller-manager/pkg/apis/node/v1alpha1"
	v1alpha1client "github.com/gardener/node-controller-manager/pkg/client/clientset/typed/node/v1alpha1"
	v1alpha1listers "github.com/gardener/node-controller-manager/pkg/client/listers/node/v1alpha1"
)

// TODO: use client library instead when it starts to support update retries
//       see https://github.com/kubernetes/kubernetes/issues/21479
type updateInstanceFunc func(instance *v1alpha1.Instance) error

// UpdateInstanceWithRetries updates a instance with given applyUpdate function. Note that instance not found error is ignored.
// The returned bool value can be used to tell if the instance is actually updated.
func UpdateInstanceWithRetries(instanceClient v1alpha1client.InstanceInterface, instanceLister v1alpha1listers.InstanceLister, namespace, name string, applyUpdate updateInstanceFunc) (*v1alpha1.Instance, error) {
	var instance *v1alpha1.Instance

	retryErr := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		var err error
		instance, err = instanceLister.Get(name)
		if err != nil {
			return err
		}
		instance = instance.DeepCopy()
		// Apply the update, then attempt to push it to the apiserver.
		if applyErr := applyUpdate(instance); applyErr != nil {
			return applyErr
		}
		instance, err = instanceClient.Update(instance)
		return err
	})

	// Ignore the precondition violated error, this instance is already updated
	// with the desired label.
	if retryErr == errorsutil.ErrPreconditionViolated {
		glog.V(4).Infof("Instance %s precondition doesn't hold, skip updating it.", name)
		retryErr = nil
	}

	return instance, retryErr
}
