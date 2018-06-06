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
package infraserver

import (
	"fmt"
	"path"
	"time"

	"github.com/gardener/machine-controller-manager/pkg/grpc/infraclient"
	"github.com/golang/glog"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type testdata struct {
	machineClass *MachineClassMeta
	credentials  string
	machineID    string
	machineName  string
	providerID   string
	nodeName     string
	err          error
}

var _ = Describe("ExternalDriverManager", func() {
	DescribeTable("##Start",
		func(machineClassType *metav1.TypeMeta, creates, deletes []*testdata) {
			server := &ExternalDriverManager{
				Port: 50000,
			}

			defer server.Stop()
			server.Start()

			fakeDriverProvider := &fakeExternalDriverProvider{
				machineClassType: machineClassType,
				creates:          creates,
				deletes:          deletes,
			}
			externalDriver := infraclient.NewExternalDriver("localhost:50000", []grpc.DialOption{
				grpc.WithInsecure(),
			}, fakeDriverProvider)

			defer externalDriver.Stop()
			externalDriver.Start()

			var (
				driver Driver
				err    error
			)
			for i := 0; i < 1; i++ {
				time.Sleep(2 * time.Second)
				driver, err = server.GetDriver(*machineClassType)
				glog.Infof("%v", err)
				if err == nil {
					break
				}
			}

			Expect(err).To(BeNil())
			Expect(driver).To(Not(BeNil()))

			for _, t := range creates {
				providerID, nodeName, err := driver.Create(t.machineClass, t.credentials, t.machineID, t.machineName)
				Expect(providerID).To(BeEquivalentTo(t.providerID))
				Expect(nodeName).To(BeEquivalentTo(t.nodeName))
				if t.err == nil {
					Expect(err).To(BeNil())
				} else {
					Expect(err.Error()).To(BeEquivalentTo(t.err.Error()))
				}
			}

			for _, t := range deletes {
				err := driver.Delete(t.credentials, t.machineID)
				if t.err == nil {
					Expect(err).To(BeNil())
				} else {
					Expect(err.Error()).To(BeEquivalentTo(t.err.Error()))
				}
			}
		},
		Entry("happy path", &metav1.TypeMeta{
			Kind:       "driver",
			APIVersion: path.Join("poc", "alpha"),
		}, []*testdata{&testdata{
			credentials: "c",
			machineID:   "a",
			machineName: "b",
			providerID:  "fakeID",
			nodeName:    "fakename",
			err:         nil,
		}}, []*testdata{&testdata{
			credentials: "c",
			machineID:   "a",
			err:         nil,
		}}),
	)
})

type fakeExternalDriverProvider struct {
	machineClassType *metav1.TypeMeta
	creates          []*testdata
	deletes          []*testdata
}

func (f *fakeExternalDriverProvider) Register() metav1.TypeMeta {
	return *f.machineClassType
}

func (f *fakeExternalDriverProvider) Create(machineclass *infraclient.MachineClassMeta, credentials, machineID, machineName string) (string, string, error) {
	for _, t := range f.creates {
		if t.machineID == machineID {
			return t.providerID, t.nodeName, t.err
		}
	}
	return "", "", fmt.Errorf("No fake data found for %v", machineID)
}

func (f *fakeExternalDriverProvider) Delete(credentials, machineID string) error {
	for _, t := range f.creates {
		if t.machineID == machineID {
			return t.err
		}
	}
	return fmt.Errorf("No fake data found for %v", machineID)
}

func (f *fakeExternalDriverProvider) List(machineID string) (map[string]string, error) {
	//TODO
	return nil, nil
}
