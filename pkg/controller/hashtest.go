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
https://github.com/kubernetes/kubernetes/release-1.8/pkg/controller/deployment/util/hash_test.go

Modifications Copyright (c) 2017 SAP SE or an SAP affiliate company. All rights reserved.
*/

// Package controller is used to provide the core functionalities of machine-controller-manager
package controller

import (
	"encoding/json"
	"hash/adler32"
	"strconv"
	"strings"
	"testing"

	"github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	hashutil "github.com/gardener/machine-controller-manager/pkg/util/hash"
)

var machineSpec = `
{
   "metadata": {
      "name": "vm2",
      "labels": {
         "machineSet": "is2"
      }
   },
   "spec": {
      "class": {
         "apiGroup": "machine.sapcloud.io/v1alpha1",
         "kind": "AWSMachineClass",
         "name": "test-aws"
      }
   }
}
`

// TestMachineTemplateSpecHash tests the templateSpecHash for any collisions
func TestMachineTemplateSpecHash(t *testing.T) {
	seenHashes := make(map[uint32]int)

	for i := 0; i < 1000; i++ {
		specJSON := strings.Replace(machineSpec, "@@VERSION@@", strconv.Itoa(i), 1)
		spec := v1alpha1.MachineTemplateSpec{}
		json.Unmarshal([]byte(specJSON), &spec)
		hash := ComputeHash(&spec, nil)
		if v, ok := seenHashes[hash]; ok {
			t.Errorf("Hash collision, old: %d new: %d", v, i)
			break
		}
		seenHashes[hash] = i
	}
}

// BenchmarkAdler is used to fetch machineTemplateSpecOldHash
func BenchmarkAdler(b *testing.B) {
	spec := v1alpha1.MachineTemplateSpec{}
	json.Unmarshal([]byte(machineSpec), &spec)

	for i := 0; i < b.N; i++ {
		getMachineTemplateSpecOldHash(spec)
	}
}

func getMachineTemplateSpecOldHash(template v1alpha1.MachineTemplateSpec) uint32 {
	machineTemplateSpecHasher := adler32.New()
	hashutil.DeepHashObject(machineTemplateSpecHasher, template)
	return machineTemplateSpecHasher.Sum32()
}

// BenchmarkFnv is used the computeHash
func BenchmarkFnv(b *testing.B) {
	spec := v1alpha1.MachineTemplateSpec{}
	json.Unmarshal([]byte(machineSpec), &spec)

	for i := 0; i < b.N; i++ {
		ComputeHash(&spec, nil)
	}
}
