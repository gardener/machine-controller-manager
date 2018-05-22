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
https://github.com/kubernetes/kubernetes/blob/release-1.8/pkg/util/taints/taints_test.go

Modifications Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved.
*/

package taints

import (
	"reflect"
	"strings"
	"testing"

	"k8s.io/api/core/v1"

	"github.com/spf13/pflag"
)

func TestTaintsVar(t *testing.T) {
	cases := []struct {
		f   string
		err bool
		t   []v1.Taint
	}{
		{
			f: "",
			t: []v1.Taint(nil),
		},
		{
			f: "--t=foo=bar:NoSchedule",
			t: []v1.Taint{{Key: "foo", Value: "bar", Effect: "NoSchedule"}},
		},
		{
			f: "--t=foo=bar:NoSchedule,bing=bang:PreferNoSchedule",
			t: []v1.Taint{
				{Key: "foo", Value: "bar", Effect: v1.TaintEffectNoSchedule},
				{Key: "bing", Value: "bang", Effect: v1.TaintEffectPreferNoSchedule},
			},
		},
		{
			f: "--t=dedicated-for=user1:NoExecute",
			t: []v1.Taint{{Key: "dedicated-for", Value: "user1", Effect: "NoExecute"}},
		},
	}

	for i, c := range cases {
		args := append([]string{"test"}, strings.Fields(c.f)...)
		cli := pflag.NewFlagSet("test", pflag.ContinueOnError)
		var taints []v1.Taint
		cli.Var(NewVar(&taints), "t", "bar")

		err := cli.Parse(args)
		if err == nil && c.err {
			t.Errorf("[%v] expected error", i)
			continue
		}
		if err != nil && !c.err {
			t.Errorf("[%v] unexpected error: %v", i, err)
			continue
		}
		if !reflect.DeepEqual(c.t, taints) {
			t.Errorf("[%v] unexpected taints:\n\texpected:\n\t\t%#v\n\tgot:\n\t\t%#v", i, c.t, taints)
		}
	}

}

func TestAddOrUpdateTaint(t *testing.T) {
	node := &v1.Node{}

	taint := &v1.Taint{
		Key:    "foo",
		Value:  "bar",
		Effect: v1.TaintEffectNoSchedule,
	}

	checkResult := func(testCaseName string, newNode *v1.Node, expectedTaint *v1.Taint, result, expectedResult bool, err error) {
		if err != nil {
			t.Errorf("[%s] should not raise error but got %v", testCaseName, err)
		}
		if result != expectedResult {
			t.Errorf("[%s] should return %t, but got: %t", testCaseName, expectedResult, result)
		}
		if len(newNode.Spec.Taints) != 1 || !reflect.DeepEqual(newNode.Spec.Taints[0], *expectedTaint) {
			t.Errorf("[%s] node should only have one taint: %v, but got: %v", testCaseName, *expectedTaint, newNode.Spec.Taints)
		}
	}

	// Add a new Taint.
	newNode, result, err := AddOrUpdateTaint(node, taint)
	checkResult("Add New Taint", newNode, taint, result, true, err)

	// Update a Taint.
	taint.Value = "bar_1"
	newNode, result, err = AddOrUpdateTaint(node, taint)
	checkResult("Update Taint", newNode, taint, result, true, err)

	// Add a duplicate Taint.
	node = newNode
	newNode, result, err = AddOrUpdateTaint(node, taint)
	checkResult("Add Duplicate Taint", newNode, taint, result, false, err)
}
