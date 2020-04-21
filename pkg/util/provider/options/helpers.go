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
https://github.com/kubernetes/kubernetes/blob/release-1.8/pkg/apis/componentconfig/helpers.go
*/

// Package options is used to specify options to MCM
package options

import (
	"encoding/json"
	"fmt"
	"net"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilnet "k8s.io/apimachinery/pkg/util/net"
)

// used for validating command line opts
// TODO(mikedanese): remove these when we remove command line flags

// IPVar is used to store the IP Address as a string
type IPVar struct {
	Val *string
}

// Set is used to set IPVar
func (v IPVar) Set(s string) error {
	if net.ParseIP(s) == nil {
		return fmt.Errorf("%q is not a valid IP address", s)
	}
	if v.Val == nil {
		// it's okay to panic here since this is programmer error
		panic("the string pointer passed into IPVar should not be nil")
	}
	*v.Val = s
	return nil
}

// String is used to get IPVar in string format
func (v IPVar) String() string {
	if v.Val == nil {
		return ""
	}
	return *v.Val
}

// Type is used to determine the type of IPVar
func (v IPVar) Type() string {
	return "ip"
}

// PortRangeVar is used to store the range of ports
type PortRangeVar struct {
	Val *string
}

// Set is used to set the PortRangeVar
func (v PortRangeVar) Set(s string) error {
	if _, err := utilnet.ParsePortRange(s); err != nil {
		return fmt.Errorf("%q is not a valid port range: %v", s, err)
	}
	if v.Val == nil {
		// it's okay to panic here since this is programmer error
		panic("the string pointer passed into PortRangeVar should not be nil")
	}
	*v.Val = s
	return nil
}

// String is used to get PortRangeVar in string format
func (v PortRangeVar) String() string {
	if v.Val == nil {
		return ""
	}
	return *v.Val
}

// Type is used to determine the type of PortRangeVar
func (v PortRangeVar) Type() string {
	return "port-range"
}

// ConvertObjToConfigMap converts an object to a ConfigMap.
// This is specifically meant for ComponentConfigs.
func ConvertObjToConfigMap(name string, obj runtime.Object) (*v1.ConfigMap, error) {
	eJSONBytes, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}
	cm := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Data: map[string]string{
			name: string(eJSONBytes[:]),
		},
	}
	return cm, nil
}
