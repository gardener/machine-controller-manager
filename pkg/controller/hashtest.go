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

Modifications Copyright 2017 The Gardener Authors.
*/

package controller

import (
    "encoding/json"
    "hash/adler32"
    "strconv"
    "strings"
    "testing"

    hashutil "k8s.io/kubernetes/pkg/util/hash"
    "github.com/gardener/node-controller-manager/pkg/apis/machine/v1alpha1"
)

var machineSpec string = `
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

func TestMachineTemplateSpecHash(t *testing.T) {
    seenHashes := make(map[uint32]int)

    for i := 0; i < 1000; i++ {
        specJson := strings.Replace(machineSpec, "@@VERSION@@", strconv.Itoa(i), 1)
        spec := v1alpha1.MachineTemplateSpec{}
        json.Unmarshal([]byte(specJson), &spec)
        hash := ComputeHash(&spec, nil)
        if v, ok := seenHashes[hash]; ok {
            t.Errorf("Hash collision, old: %d new: %d", v, i)
            break
        }
        seenHashes[hash] = i
    }
}

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

func BenchmarkFnv(b *testing.B) {
    spec := v1alpha1.MachineTemplateSpec{}
    json.Unmarshal([]byte(machineSpec), &spec)

    for i := 0; i < b.N; i++ {
        ComputeHash(&spec, nil)
    }
}
