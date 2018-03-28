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
https://github.com/kubernetes/kubernetes/blob/release-1.8/pkg/controller/client_builder.go
*/

package controller

import (
	clientgoclientset "k8s.io/client-go/kubernetes"
	clientset "k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"

	"github.com/golang/glog"
)

// ControllerClientBuilder allows you to get clients and configs for controllers
type ControllerClientBuilder interface {
	Config(name string) (*restclient.Config, error)
	ConfigOrDie(name string) *restclient.Config
	Client(name string) (clientset.Interface, error)
	ClientOrDie(name string) clientset.Interface
	ClientGoClient(name string) (clientgoclientset.Interface, error)
	ClientGoClientOrDie(name string) clientgoclientset.Interface
}

// SimpleControllerClientBuilder returns a fixed client with different user agents
type SimpleControllerClientBuilder struct {
	// ClientConfig is a skeleton config to clone and use as the basis for each controller client
	ClientConfig *restclient.Config
}

func (b SimpleControllerClientBuilder) Config(name string) (*restclient.Config, error) {
	clientConfig := *b.ClientConfig
	return restclient.AddUserAgent(&clientConfig, name), nil
}

func (b SimpleControllerClientBuilder) ConfigOrDie(name string) *restclient.Config {
	clientConfig, err := b.Config(name)
	if err != nil {
		glog.Fatal(err)
	}
	return clientConfig
}

func (b SimpleControllerClientBuilder) Client(name string) (clientset.Interface, error) {
	clientConfig, err := b.Config(name)
	if err != nil {
		return nil, err
	}
	return clientset.NewForConfig(clientConfig)
}

func (b SimpleControllerClientBuilder) ClientOrDie(name string) clientset.Interface {
	client, err := b.Client(name)
	if err != nil {
		glog.Fatal(err)
	}
	return client
}

func (b SimpleControllerClientBuilder) ClientGoClient(name string) (clientgoclientset.Interface, error) {
	clientConfig, err := b.Config(name)
	if err != nil {
		return nil, err
	}
	return clientgoclientset.NewForConfig(clientConfig)
}

func (b SimpleControllerClientBuilder) ClientGoClientOrDie(name string) clientgoclientset.Interface {
	client, err := b.ClientGoClient(name)
	if err != nil {
		glog.Fatal(err)
	}
	return client
}
