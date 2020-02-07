/*
Copyright 2014 The Kubernetes Authors.

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
https://github.com/kubernetes/kubernetes/release-1.8/cmd/kube-controller-manager/app/controllermanager.go

Modifications Copyright (c) 2017 SAP SE or an SAP affiliate company. All rights reserved.
*/

package app

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/pprof"
	"os"
	goruntime "runtime"
	"strconv"
	"time"

	machinescheme "github.com/gardener/machine-controller-manager/pkg/client/clientset/versioned/scheme"
	machineinformers "github.com/gardener/machine-controller-manager/pkg/client/informers/externalversions"
	machinecontroller "github.com/gardener/machine-controller-manager/pkg/controller"
	corecontroller "github.com/gardener/machine-controller-manager/pkg/util/controller"
	coreinformers "k8s.io/client-go/informers"
	kubescheme "k8s.io/client-go/kubernetes/scheme"

	"github.com/gardener/machine-controller-manager/cmd/machine-controller-manager/app/options"
	"github.com/gardener/machine-controller-manager/pkg/handlers"
	"github.com/gardener/machine-controller-manager/pkg/util/configz"
	"github.com/prometheus/client_golang/prometheus"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/pkg/version"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog"
)

const (
	controllerManagerAgentName   = "machine-controller-manager"
	controllerDiscoveryAgentName = "machine-controller-discovery"
)

var openStackGVR = schema.GroupVersionResource{Group: "machine.sapcloud.io", Version: "v1alpha1", Resource: "openstackmachineclasses"}
var awsGVR = schema.GroupVersionResource{Group: "machine.sapcloud.io", Version: "v1alpha1", Resource: "awsmachineclasses"}
var azureGVR = schema.GroupVersionResource{Group: "machine.sapcloud.io", Version: "v1alpha1", Resource: "azuremachineclasses"}
var gcpGVR = schema.GroupVersionResource{Group: "machine.sapcloud.io", Version: "v1alpha1", Resource: "gcpmachineclasses"}
var alicloudGVR = schema.GroupVersionResource{Group: "machine.sapcloud.io", Version: "v1alpha1", Resource: "alicloudmachineclasses"}
var packetGVR = schema.GroupVersionResource{Group: "machine.sapcloud.io", Version: "v1alpha1", Resource: "packetmachineclasses"}
var metalGVR = schema.GroupVersionResource{Group: "machine.sapcloud.io", Version: "v1alpha1", Resource: "metalmachineclasses"}

// Run runs the MCMServer.  This should never exit.
func Run(s *options.MCMServer) error {
	// To help debugging, immediately log version
	klog.V(4).Infof("Version: %+v", version.Get())
	if err := s.Validate(); err != nil {
		return err
	}

	var err error

	//kubeconfig for the cluster for which machine-controller-manager will create machines.
	targetkubeconfig, err := clientcmd.BuildConfigFromFlags("", s.TargetKubeconfig)
	if err != nil {
		return err
	}

	controlkubeconfig := targetkubeconfig

	if s.ControlKubeconfig != "" {
		if s.ControlKubeconfig == "inClusterConfig" {
			//use inClusterConfig when controller is running inside clus
			controlkubeconfig, err = clientcmd.BuildConfigFromFlags("", "")
		} else {
			//kubeconfig for the seedcluster where MachineCRDs are supposed to be registered.
			controlkubeconfig, err = clientcmd.BuildConfigFromFlags("", s.ControlKubeconfig)
		}
		if err != nil {
			return err
		}
	}

	// PROTOBUF WONT WORK
	// kubeconfig.ContentConfig.ContentType = s.ContentType
	// Override kubeconfig qps/burst settings from flags
	targetkubeconfig.QPS = s.KubeAPIQPS
	controlkubeconfig.QPS = s.KubeAPIQPS
	targetkubeconfig.Burst = int(s.KubeAPIBurst)
	controlkubeconfig.Burst = int(s.KubeAPIBurst)

	kubeClientControl, err := kubernetes.NewForConfig(
		rest.AddUserAgent(controlkubeconfig, "machine-controller-manager"),
	)
	if err != nil {
		klog.Fatalf("Invalid API configuration for kubeconfig-control: %v", err)
	}

	leaderElectionClient := kubernetes.NewForConfigOrDie(rest.AddUserAgent(controlkubeconfig, "machine-leader-election"))
	klog.V(4).Info("Starting http server and mux")
	go startHTTP(s)

	recorder := createRecorder(kubeClientControl)

	run := func(ctx context.Context) {
		var stop <-chan struct{}
		// Control plane client used to interact with machine APIs
		controlMachineClientBuilder := machinecontroller.SimpleClientBuilder{
			ClientConfig: controlkubeconfig,
		}
		// Control plane client used to interact with core kubernetes objects
		controlCoreClientBuilder := corecontroller.SimpleControllerClientBuilder{
			ClientConfig: controlkubeconfig,
		}
		// Target plane client used to interact with core kubernetes objects
		targetCoreClientBuilder := corecontroller.SimpleControllerClientBuilder{
			ClientConfig: targetkubeconfig,
		}

		err := StartControllers(
			s,
			controlkubeconfig,
			targetkubeconfig,
			controlMachineClientBuilder,
			controlCoreClientBuilder,
			targetCoreClientBuilder,
			recorder,
			stop,
		)

		klog.Fatalf("error running controllers: %v", err)
		panic("unreachable")

	}

	if !s.LeaderElection.LeaderElect {
		run(nil)
		panic("unreachable")
	}

	id, err := os.Hostname()
	if err != nil {
		return err
	}

	rl, err := resourcelock.New(
		s.LeaderElection.ResourceLock,
		s.Namespace,
		"machine-controller-manager",
		leaderElectionClient.CoreV1(),
		leaderElectionClient.CoordinationV1(),
		resourcelock.ResourceLockConfig{
			Identity:      id,
			EventRecorder: recorder,
		},
	)
	if err != nil {
		klog.Fatalf("error creating lock: %v", err)
	}

	ctx := context.TODO()
	leaderelection.RunOrDie(ctx, leaderelection.LeaderElectionConfig{
		Lock:          rl,
		LeaseDuration: s.LeaderElection.LeaseDuration.Duration,
		RenewDeadline: s.LeaderElection.RenewDeadline.Duration,
		RetryPeriod:   s.LeaderElection.RetryPeriod.Duration,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: run,
			OnStoppedLeading: func() {
				klog.Fatalf("leaderelection lost")
			},
		},
	})
	panic("unreachable")
}

// StartControllers starts all the controllers which are a part of machine-controller-manager
func StartControllers(s *options.MCMServer,
	controlCoreKubeconfig *rest.Config,
	targetCoreKubeconfig *rest.Config,
	controlMachineClientBuilder machinecontroller.ClientBuilder,
	controlCoreClientBuilder corecontroller.ClientBuilder,
	targetCoreClientBuilder corecontroller.ClientBuilder,
	recorder record.EventRecorder,
	stop <-chan struct{}) error {

	klog.V(5).Info("Getting available resources")
	availableResources, err := getAvailableResources(controlCoreClientBuilder)
	if err != nil {
		return err
	}

	controlMachineClient := controlMachineClientBuilder.ClientOrDie(controllerManagerAgentName).MachineV1alpha1()

	controlCoreKubeconfig = rest.AddUserAgent(controlCoreKubeconfig, controllerManagerAgentName)
	controlCoreClient, err := kubernetes.NewForConfig(controlCoreKubeconfig)
	if err != nil {
		klog.Fatal(err)
	}

	targetCoreKubeconfig = rest.AddUserAgent(targetCoreKubeconfig, controllerManagerAgentName)
	targetCoreClient, err := kubernetes.NewForConfig(targetCoreKubeconfig)
	if err != nil {
		klog.Fatal(err)
	}

	if availableResources[awsGVR] || availableResources[azureGVR] || availableResources[gcpGVR] || availableResources[openStackGVR] || availableResources[alicloudGVR] || availableResources[packetGVR] || availableResources[metalGVR] {
		klog.V(5).Infof("Creating shared informers; resync interval: %v", s.MinResyncPeriod)

		controlMachineInformerFactory := machineinformers.NewFilteredSharedInformerFactory(
			controlMachineClientBuilder.ClientOrDie("control-machine-shared-informers"),
			s.MinResyncPeriod.Duration,
			s.Namespace,
			nil,
		)

		controlCoreInformerFactory := coreinformers.NewFilteredSharedInformerFactory(
			controlCoreClientBuilder.ClientOrDie("control-core-shared-informers"),
			s.MinResyncPeriod.Duration,
			s.Namespace,
			nil,
		)

		targetCoreInformerFactory := coreinformers.NewSharedInformerFactory(
			targetCoreClientBuilder.ClientOrDie("target-core-shared-informers"),
			s.MinResyncPeriod.Duration,
		)

		// All shared informers are v1alpha1 API level
		machineSharedInformers := controlMachineInformerFactory.Machine().V1alpha1()

		klog.V(5).Infof("Creating controllers...")
		machineController, err := machinecontroller.NewController(
			s.Namespace,
			controlMachineClient,
			controlCoreClient,
			targetCoreClient,
			targetCoreInformerFactory.Core().V1().PersistentVolumeClaims(),
			targetCoreInformerFactory.Core().V1().PersistentVolumes(),
			controlCoreInformerFactory.Core().V1().Secrets(),
			targetCoreInformerFactory.Core().V1().Nodes(),
			machineSharedInformers.OpenStackMachineClasses(),
			machineSharedInformers.AWSMachineClasses(),
			machineSharedInformers.AzureMachineClasses(),
			machineSharedInformers.GCPMachineClasses(),
			machineSharedInformers.AlicloudMachineClasses(),
			machineSharedInformers.PacketMachineClasses(),
			machineSharedInformers.MetalMachineClasses(),
			machineSharedInformers.Machines(),
			machineSharedInformers.MachineSets(),
			machineSharedInformers.MachineDeployments(),
			recorder,
			s.SafetyOptions,
			s.NodeConditions,
		)
		if err != nil {
			return err
		}
		klog.V(1).Info("Starting shared informers")

		controlMachineInformerFactory.Start(stop)
		controlCoreInformerFactory.Start(stop)
		targetCoreInformerFactory.Start(stop)

		klog.V(5).Info("Running controller")
		go machineController.Run(int(s.ConcurrentNodeSyncs), stop)

	} else {
		return fmt.Errorf("unable to start machine controller: API GroupVersion %q or %q or %q or %q or %q or %q or %q is not available; found %#v", awsGVR, azureGVR, gcpGVR, openStackGVR, alicloudGVR, packetGVR, metalGVR, availableResources)
	}

	select {}
}

// TODO: In general, any controller checking this needs to be dynamic so
//  users don't have to restart their controller manager if they change the apiserver.
// Until we get there, the structure here needs to be exposed for the construction of a proper ControllerContext.
func getAvailableResources(clientBuilder corecontroller.ClientBuilder) (map[schema.GroupVersionResource]bool, error) {
	var discoveryClient discovery.DiscoveryInterface

	var healthzContent string
	// If apiserver is not running we should wait for some time and fail only then. This is particularly
	// important when we start apiserver and controller manager at the same time.
	err := wait.PollImmediate(time.Second, 10*time.Second, func() (bool, error) {
		client, err := clientBuilder.Client("controller-discovery")
		if err != nil {
			klog.Errorf("Failed to get api versions from server: %v", err)
			return false, nil
		}

		healthStatus := 0
		resp := client.Discovery().RESTClient().Get().AbsPath("/healthz").Do().StatusCode(&healthStatus)
		if healthStatus != http.StatusOK {
			klog.Errorf("Server isn't healthy yet.  Waiting a little while.")
			return false, nil
		}
		content, _ := resp.Raw()
		healthzContent = string(content)

		discoveryClient = client.Discovery()
		return true, nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get api versions from server: %v: %v", healthzContent, err)
	}

	resourceMap, err := discoveryClient.ServerResources()
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("unable to get all supported resources from server: %v", err))
	}
	if len(resourceMap) == 0 {
		return nil, fmt.Errorf("unable to get any supported resources from server")
	}

	allResources := map[schema.GroupVersionResource]bool{}
	for _, apiResourceList := range resourceMap {
		version, err := schema.ParseGroupVersion(apiResourceList.GroupVersion)
		if err != nil {
			return nil, err
		}
		for _, apiResource := range apiResourceList.APIResources {
			allResources[version.WithResource(apiResource.Name)] = true
		}
	}

	return allResources, nil
}

func createRecorder(kubeClient *kubernetes.Clientset) record.EventRecorder {
	machinescheme.AddToScheme(kubescheme.Scheme)
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.Infof)
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: v1core.New(kubeClient.CoreV1().RESTClient()).Events("")})
	return eventBroadcaster.NewRecorder(kubescheme.Scheme, v1.EventSource{Component: controllerManagerAgentName})
}

func startHTTP(s *options.MCMServer) {
	mux := http.NewServeMux()
	if s.EnableProfiling {
		mux.HandleFunc("/debug/pprof/", pprof.Index)
		mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
		mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
		if s.EnableContentionProfiling {
			goruntime.SetBlockProfileRate(1)
		}
	}
	configz.InstallHandler(mux)
	mux.Handle("/metrics", prometheus.Handler())
	handlers.UpdateHealth(true)
	mux.HandleFunc("/healthz", handlers.Healthz)

	server := &http.Server{
		Addr:    net.JoinHostPort(s.Address, strconv.Itoa(int(s.Port))),
		Handler: mux,
	}
	klog.Fatal(server.ListenAndServe())
}
