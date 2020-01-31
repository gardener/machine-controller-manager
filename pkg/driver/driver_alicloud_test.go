/*
Copyright (c) 2017 SAP SE or an SAP affiliate company. All rights reserved.

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
	"testing"

	"github.com/aliyun/alibaba-cloud-sdk-go/services/ecs"
)

func TestTagsOrdered(t *testing.T) {
	tags := map[string]string{
		"kubernetes.io/cluster/ali-test": "1",
		"kubernetes.io/role/worker":      "1",
		"taga":                           "tagvala",
		"tagb":                           "tagvalb",
		"tagc":                           "tagvalc",
	}
	c := &AlicloudDriver{}
	res, err := c.toInstanceTags(tags)
	if err != nil {
		t.Errorf("toInstanceTags in TestTagsOrdered should not generate error: %v", err)
	}

	expected := []ecs.RunInstancesTag{
		{
			Key:   "kubernetes.io/cluster/ali-test",
			Value: "1",
		},
		{
			Key:   "kubernetes.io/role/worker",
			Value: "1",
		},
		{
			Key:   "taga",
			Value: "tagvala",
		},
		{
			Key:   "tagb",
			Value: "tagvalb",
		},
		{
			Key:   "tagc",
			Value: "tagvalc",
		},
	}
	checkRunInstanceTags("Function TestTagsOrdered: ", t, res, expected)
}

func TestNoClusterTags(t *testing.T) {
	tags := map[string]string{
		"kubernetes.io/role/worker": "1",
		"taga":                      "tagvala",
		"tagb":                      "tagvalb",
		"tagc":                      "tagvalc",
	}
	c := &AlicloudDriver{}
	_, err := c.toInstanceTags(tags)
	if err == nil {
		t.Errorf("toInstanceTags in TestRandomOrderTags should return an error")
	}
}
func TestRandomOrderTags(t *testing.T) {
	tags := map[string]string{
		"taga":                           "tagvala",
		"tagb":                           "tagvalb",
		"kubernetes.io/cluster/ali-test": "1",
		"kubernetes.io/role/worker":      "1",
		"tagc":                           "tagvalc",
	}
	c := &AlicloudDriver{}
	res, err := c.toInstanceTags(tags)
	if err != nil {
		t.Errorf("toInstanceTags in TestRandomOrderTags should not generate error: %v", err)
	}

	expected := []ecs.RunInstancesTag{
		{
			Key:   "kubernetes.io/cluster/ali-test",
			Value: "1",
		},
		{
			Key:   "kubernetes.io/role/worker",
			Value: "1",
		},
		{
			Key:   "taga",
			Value: "tagvala",
		},
		{
			Key:   "tagb",
			Value: "tagvalb",
		},
		{
			Key:   "tagc",
			Value: "tagvalc",
		},
	}
	checkRunInstanceTags("Function TestRandomOrderTags: ", t, res, expected)
}
func TestIDToName(t *testing.T) {
	id := "i-uf69zddmom11ci7est12"

	c := &AlicloudDriver{}
	if "iZuf69zddmom11ci7est12Z" != c.idToName(id) {
		t.Error("idToName() is not working")
	}
}

// real[2..]'s order is NOT predicted as tags which generated them is a MAP!!!
func checkRunInstanceTags(leadErrMsg string, t *testing.T, real, expected []ecs.RunInstancesTag) {
	if len(real) != len(expected) {
		t.Errorf("%s: %s", leadErrMsg, "count of generated tags is not as expected")
		return
	}

	// index 0 and 1 is static
	if real[0] != expected[0] {
		t.Errorf("%s: tag %s should be at index %d", leadErrMsg, expected[0], 0)
	}
	if real[1] != expected[1] {
		t.Errorf("%s: tag %s should be at index %d", leadErrMsg, expected[1], 1)
	}

found:
	for i := 2; i < len(expected); i++ {
		for j := 2; j < len(real); j++ {
			if expected[i] == real[j] {
				continue found
			}
		}
		t.Errorf("%s: tag %s is not in real tags", leadErrMsg, expected[i])
		return
	}
}
