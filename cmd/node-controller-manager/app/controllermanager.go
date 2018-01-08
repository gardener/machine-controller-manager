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

Modifications Copyright 2017 The Gardener Authors.
*/

package app

import (
	"fmt"
	"net"
	"net/http"
	"net/http/pprof"
	"os"
	goruntime "runtime"
	"strconv"
	"time"

	coreinformers "k8s.io/client-go/informers"

	nodeinformers "github.com/gardener/node-controller-manager/pkg/client/informers/externalversions"

	corecontroller "k8s.io/kubernetes/pkg/controller"

	nodecontroller "github.com/gardener/node-controller-manager/pkg/controller"

	"github.com/gardener/node-controller-manager/cmd/node-controller-manager/app/options"
	"github.com/golang/glog"
	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apiserver/pkg/server/healthz"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/pkg/version"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/client-go/tools/record"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/util/configz"
)

const (
	controllerManagerAgentName   = "node-controller-manager"
	controllerDiscoveryAgentName = "node-controller-discovery"
)

var nodeGVR = schema.GroupVersionResource{Group: "machine.sapcloud.io", Version: "v1alpha1", Resource: "awsmachineclasses"}

// Run runs the NCMServer.  This should never exit.
func Run(s *options.NCMServer) error {
	// To help debugging, immediately log version
	glog.V(4).Infof("Version: %+v", version.Get())
	if err := s.Validate(); err != nil {
		return err
	}

	var err error

	kubeconfig, err := clientcmd.BuildConfigFromFlags("", s.Kubeconfig)
	if err != nil {
		return err
	}

	// PROTOBUF WONT WORK
	// kubeconfig.ContentConfig.ContentType = s.ContentType
	// Override kubeconfig qps/burst settings from flags
	kubeconfig.QPS = s.KubeAPIQPS
	kubeconfig.Burst = int(s.KubeAPIBurst)
	kubeClient, err := kubernetes.NewForConfig(
		rest.AddUserAgent(kubeconfig, "node-controller-manager"),
	)
	if err != nil {
		glog.Fatalf("Invalid API configuration: %v", err)
	}
	leaderElectionClient := kubernetes.NewForConfigOrDie(rest.AddUserAgent(kubeconfig, "node-leader-election"))

	glog.V(4).Info("Starting http server and mux")
	go startHTTP(s)

	recorder := createRecorder(kubeClient)

	run := func(stop <-chan struct{}) {
		nodeClientBuilder := nodecontroller.SimpleClientBuilder{
			ClientConfig: kubeconfig,
		}

		coreClientBuilder := corecontroller.SimpleControllerClientBuilder{
			ClientConfig: kubeconfig,
		}

		err := StartControllers(s, kubeconfig, nodeClientBuilder, coreClientBuilder, recorder, stop)
		glog.Fatalf("error running controllers: %v", err)
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

	rl, err := resourcelock.New(s.LeaderElection.ResourceLock,
		"kube-system",
		"node-controller-manager",
		leaderElectionClient.CoreV1(),
		resourcelock.ResourceLockConfig{
			Identity:      id,
			EventRecorder: recorder,
		})
	if err != nil {
		glog.Fatalf("error creating lock: %v", err)
	}

	leaderelection.RunOrDie(leaderelection.LeaderElectionConfig{
		Lock:          rl,
		LeaseDuration: s.LeaderElection.LeaseDuration.Duration,
		RenewDeadline: s.LeaderElection.RenewDeadline.Duration,
		RetryPeriod:   s.LeaderElection.RetryPeriod.Duration,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: run,
			OnStoppedLeading: func() {
				glog.Fatalf("leaderelection lost")
			},
		},
	})
	panic("unreachable")
}

func StartControllers(s *options.NCMServer,
	coreKubeconfig *rest.Config,
	nodeClientBuilder nodecontroller.ClientBuilder,
	coreClientBuilder corecontroller.ControllerClientBuilder,
	recorder record.EventRecorder,
	stop <-chan struct{}) error {

	glog.V(5).Info("Getting available resources")
	availableResources, err := getAvailableResources(coreClientBuilder)
	if err != nil {
		return err
	}

	coreKubeconfig = rest.AddUserAgent(coreKubeconfig, controllerManagerAgentName)
	coreClient, err := kubernetes.NewForConfig(coreKubeconfig)
	if err != nil {
		glog.Fatal(err)
	}

	if availableResources[nodeGVR] { 
		glog.V(5).Infof("Creating shared informers; resync interval: %v", s.MinResyncPeriod)

		nodeInformerFactory := nodeinformers.NewSharedInformerFactory(
			nodeClientBuilder.ClientOrDie("node-shared-informers"),
			s.MinResyncPeriod.Duration,
		)

		coreInformerFactory := coreinformers.NewSharedInformerFactory(
			coreClientBuilder.ClientOrDie("core-shared-informers"),
			s.MinResyncPeriod.Duration,
		)
		// All shared informers are v1alpha1 API level
		nodeSharedInformers := nodeInformerFactory.Machine().V1alpha1()

		glog.V(5).Infof("Creating controllers...")
		nodeController, err := nodecontroller.NewController(
			coreClient,
			nodeClientBuilder.ClientOrDie(controllerManagerAgentName).MachineV1alpha1(),
			coreInformerFactory.Core().V1().Secrets(),
			coreInformerFactory.Core().V1().Nodes(),
			nodeSharedInformers.AWSMachineClasses(),
			nodeSharedInformers.Machines(),
			nodeSharedInformers.MachineSets(),
			nodeSharedInformers.MachineDeployments(),
			recorder,
		)
		if err != nil {
			return err
		}
		glog.V(1).Info("Starting shared informers")

		coreInformerFactory.Start(stop)
		nodeInformerFactory.Start(stop)

		glog.V(5).Info("Running controller")
		go nodeController.Run(int(s.ConcurrentNodeSyncs), stop)

	} else {
		return fmt.Errorf("unable to start machine controller: API GroupVersion %q is not available; found %#v", nodeGVR, availableResources)
	}

	select {}
}

// TODO: In general, any controller checking this needs to be dynamic so
//  users don't have to restart their controller manager if they change the apiserver.
// Until we get there, the structure here needs to be exposed for the construction of a proper ControllerContext.
func getAvailableResources(clientBuilder corecontroller.ControllerClientBuilder) (map[schema.GroupVersionResource]bool, error) {
	var discoveryClient discovery.DiscoveryInterface

	var healthzContent string
	// If apiserver is not running we should wait for some time and fail only then. This is particularly
	// important when we start apiserver and controller manager at the same time.
	err := wait.PollImmediate(time.Second, 10*time.Second, func() (bool, error) {
		client, err := clientBuilder.Client("controller-discovery")
		if err != nil {
			glog.Errorf("Failed to get api versions from server: %v", err)
			return false, nil
		}

		healthStatus := 0
		resp := client.Discovery().RESTClient().Get().AbsPath("/healthz").Do().StatusCode(&healthStatus)
		if healthStatus != http.StatusOK {
			glog.Errorf("Server isn't healthy yet.  Waiting a little while.")
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
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(glog.Infof)
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: v1core.New(kubeClient.CoreV1().RESTClient()).Events("")})
	return eventBroadcaster.NewRecorder(api.Scheme, v1.EventSource{Component: controllerManagerAgentName})
}

func startHTTP(s *options.NCMServer) {
	mux := http.NewServeMux()
	healthz.InstallHandler(mux)
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

	server := &http.Server{
		Addr:    net.JoinHostPort(s.Address, strconv.Itoa(int(s.Port))),
		Handler: mux,
	}
	glog.Fatal(server.ListenAndServe())
}
