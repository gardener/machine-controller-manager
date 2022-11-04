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
	"sync"
	"time"

	"github.com/Masterminds/semver"
	machinescheme "github.com/gardener/machine-controller-manager/pkg/client/clientset/versioned/scheme"
	machineinformers "github.com/gardener/machine-controller-manager/pkg/client/informers/externalversions"
	coreclientbuilder "github.com/gardener/machine-controller-manager/pkg/util/clientbuilder/core"
	machineclientbuilder "github.com/gardener/machine-controller-manager/pkg/util/clientbuilder/machine"
	"github.com/gardener/machine-controller-manager/pkg/util/k8sutils"
	machinecontroller "github.com/gardener/machine-controller-manager/pkg/util/provider/machinecontroller"
	coreinformers "k8s.io/client-go/informers"
	kubescheme "k8s.io/client-go/kubernetes/scheme"

	"github.com/gardener/machine-controller-manager/pkg/handlers"
	"github.com/gardener/machine-controller-manager/pkg/util/configz"
	"github.com/gardener/machine-controller-manager/pkg/util/provider/app/options"
	"github.com/gardener/machine-controller-manager/pkg/util/provider/driver"
	prometheus "github.com/prometheus/client_golang/prometheus/promhttp"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/discovery"
	policyv1informers "k8s.io/client-go/informers/policy/v1"
	policyv1beta1informers "k8s.io/client-go/informers/policy/v1beta1"
	"k8s.io/client-go/kubernetes"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/pkg/version"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
)

const (
	controllerManagerAgentName = "machine-controller"
)

var (
	machineGVR = schema.GroupVersionResource{Group: "machine.sapcloud.io", Version: "v1alpha1", Resource: "machines"}
)

// Run runs the MCServer. This should never exit.
func Run(ctx context.Context, s *options.MCServer, driver driver.Driver) error {
	// To help debugging, immediately log version
	klog.V(4).Infof("Version: %+v", version.Get())
	if err := s.Validate(); err != nil {
		return err
	}

	var err error

	// kubeconfig for the cluster for which machine-controller will create machines.
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
		rest.AddUserAgent(controlkubeconfig, "machine-controller"),
	)
	if err != nil {
		klog.Fatalf("Invalid API configuration for kubeconfig-control: %v", err)
	}

	leaderElectionClient := kubernetes.NewForConfigOrDie(rest.AddUserAgent(controlkubeconfig, "machine-leader-election"))
	klog.V(4).Info("Starting http server and mux")
	go startHTTP(s)

	recorder := createRecorder(kubeClientControl)

	waitGroup := sync.WaitGroup{}
	waitGroup.Add(1)
	startControllers := func(ctx context.Context) {
		// Control plane client used to interact with machine APIs
		controlMachineClientBuilder := machineclientbuilder.SimpleClientBuilder{
			ClientConfig: controlkubeconfig,
		}
		// Control plane client used to interact with core kubernetes objects
		controlCoreClientBuilder := coreclientbuilder.SimpleControllerClientBuilder{
			ClientConfig: controlkubeconfig,
		}
		// Target plane client used to interact with core kubernetes objects
		targetCoreClientBuilder := coreclientbuilder.SimpleControllerClientBuilder{
			ClientConfig: targetkubeconfig,
		}

		if err := StartControllers(
			ctx,
			s,
			controlkubeconfig,
			targetkubeconfig,
			controlMachineClientBuilder,
			controlCoreClientBuilder,
			targetCoreClientBuilder,
			driver,
			recorder,
		); err != nil {
			klog.Fatalf("failed to start controllers: %v", err)
		}
		waitGroup.Done()
	}

	if !s.LeaderElection.LeaderElect {
		startControllers(ctx)
	}

	id, err := os.Hostname()
	if err != nil {
		return err
	}

	rl, err := resourcelock.New(
		s.LeaderElection.ResourceLock,
		s.Namespace,
		"machine-controller",
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

	leaderelection.RunOrDie(ctx, leaderelection.LeaderElectionConfig{
		Lock:          rl,
		LeaseDuration: s.LeaderElection.LeaseDuration.Duration,
		RenewDeadline: s.LeaderElection.RenewDeadline.Duration,
		RetryPeriod:   s.LeaderElection.RetryPeriod.Duration,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: startControllers,
			OnStoppedLeading: func() {
				klog.Errorf("leaderelection lost")
				waitGroup.Wait()
			},
		},
	})

	return nil
}

// StartControllers starts all the controllers which are a part of machine-controller
func StartControllers(
	ctx context.Context,
	s *options.MCServer,
	controlCoreKubeconfig *rest.Config,
	targetCoreKubeconfig *rest.Config,
	controlMachineClientBuilder machineclientbuilder.ClientBuilder,
	controlCoreClientBuilder coreclientbuilder.ClientBuilder,
	targetCoreClientBuilder coreclientbuilder.ClientBuilder,
	driver driver.Driver,
	recorder record.EventRecorder,
) error {

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

	targetServerVersion, err := targetCoreClient.Discovery().ServerVersion()
	if err != nil {
		return err
	}
	targetKubernetesVersion, err := semver.NewVersion(targetServerVersion.GitVersion)
	if err != nil {
		return err
	}

	if availableResources[machineGVR] {
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

		var (
			pdbV1Informer      policyv1informers.PodDisruptionBudgetInformer
			pdbV1beta1Informer policyv1beta1informers.PodDisruptionBudgetInformer
		)

		if k8sutils.ConstraintK8sGreaterEqual121.Check(targetKubernetesVersion) {
			pdbV1Informer = targetCoreInformerFactory.Policy().V1().PodDisruptionBudgets()
		} else {
			pdbV1beta1Informer = targetCoreInformerFactory.Policy().V1beta1().PodDisruptionBudgets()
		}

		// All shared informers are v1alpha1 API level
		machineSharedInformers := controlMachineInformerFactory.Machine().V1alpha1()

		klog.V(5).Infof("Creating controllers...")
		machineController, err := machinecontroller.NewController(
			s.Namespace,
			controlMachineClient,
			controlCoreClient,
			targetCoreClient,
			driver,
			targetCoreInformerFactory.Core().V1().PersistentVolumeClaims(),
			targetCoreInformerFactory.Core().V1().PersistentVolumes(),
			controlCoreInformerFactory.Core().V1().Secrets(),
			targetCoreInformerFactory.Core().V1().Nodes(),
			pdbV1beta1Informer,
			pdbV1Informer,
			targetCoreInformerFactory.Storage().V1().VolumeAttachments(),
			machineSharedInformers.MachineClasses(),
			machineSharedInformers.Machines(),
			recorder,
			s.SafetyOptions,
			s.NodeConditions,
			s.BootstrapTokenAuthExtraGroups,
			targetKubernetesVersion,
		)
		if err != nil {
			return err
		}
		klog.V(1).Info("Starting shared informers")

		controlMachineInformerFactory.Start(ctx.Done())
		controlCoreInformerFactory.Start(ctx.Done())
		targetCoreInformerFactory.Start(ctx.Done())

		klog.V(5).Info("Running controller")
		var waitGroup sync.WaitGroup
		waitGroup.Add(1)
		go func() {
			machineController.Run(ctx, int(s.ConcurrentNodeSyncs))
			waitGroup.Done()
		}()
		waitGroup.Wait()
	} else {
		return fmt.Errorf("unable to start machine controller: API GroupVersion %q is not available; \nFound: %#v", machineGVR, availableResources)
	}

	return nil
}

// TODO: In general, any controller checking this needs to be dynamic so
// users don't have to restart their controller manager if they change the apiserver.
// Until we get there, the structure here needs to be exposed for the construction of a proper ControllerContext.
func getAvailableResources(clientBuilder coreclientbuilder.ClientBuilder) (map[schema.GroupVersionResource]bool, error) {
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
		resp := client.Discovery().RESTClient().Get().AbsPath("/healthz").Do(context.Background()).StatusCode(&healthStatus)
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

	_, resources, err := discoveryClient.ServerGroupsAndResources()
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("unable to get all supported resources from server: %v", err))
	}
	if len(resources) == 0 {
		return nil, fmt.Errorf("unable to get any supported resources from server")
	}

	allResources := map[schema.GroupVersionResource]bool{}
	for _, apiResourceList := range resources {
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

func startHTTP(s *options.MCServer) {
	mux := http.NewServeMux()
	if s.EnableProfiling {
		mux.HandleFunc("/debug/pprof/", pprof.Index)
		mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
		mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
		mux.HandleFunc("/debug/pprof/goroutine", pprof.Handler("goroutine").ServeHTTP)
		mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		mux.HandleFunc("/debug/pprof/heap", pprof.Handler("heap").ServeHTTP)
		mux.HandleFunc("/debug/pprof/threadcreate", pprof.Handler("threadcreate").ServeHTTP)
		mux.HandleFunc("/debug/pprof/block", pprof.Handler("block").ServeHTTP)
		mux.HandleFunc("/debug/pprof/mutex", pprof.Handler("mutex").ServeHTTP)
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
