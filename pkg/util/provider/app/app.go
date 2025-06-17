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

Modifications Copyright SAP SE or an SAP affiliate company and Gardener contributors
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

	"github.com/Masterminds/semver/v3"
	machinescheme "github.com/gardener/machine-controller-manager/pkg/client/clientset/versioned/scheme"
	machineinformers "github.com/gardener/machine-controller-manager/pkg/client/informers/externalversions"
	coreclientbuilder "github.com/gardener/machine-controller-manager/pkg/util/clientbuilder/core"
	machineclientbuilder "github.com/gardener/machine-controller-manager/pkg/util/clientbuilder/machine"
	machinecontroller "github.com/gardener/machine-controller-manager/pkg/util/provider/machinecontroller"
	kubernetesinformers "k8s.io/client-go/informers"
	kubescheme "k8s.io/client-go/kubernetes/scheme"

	"github.com/gardener/machine-controller-manager/pkg/apis/constants"
	"github.com/gardener/machine-controller-manager/pkg/handlers"
	"github.com/gardener/machine-controller-manager/pkg/util/configz"
	"github.com/gardener/machine-controller-manager/pkg/util/provider/app/options"
	"github.com/gardener/machine-controller-manager/pkg/util/provider/driver"
	"github.com/gardener/machine-controller-manager/pkg/version"
	prometheus "github.com/prometheus/client_golang/prometheus/promhttp"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
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
func Run(s *options.MCServer, driver driver.Driver) error {
	// To help debugging, immediately log version
	version.LogVersionInfoWithLevel(4)
	if err := s.Validate(); err != nil {
		return err
	}

	var err error

	// kubeconfig for the cluster for which machine-controller-manager will create machines.
	var targetkubeconfig *rest.Config
	if s.TargetKubeconfig != constants.TargetKubeconfigDisabledValue {
		targetkubeconfig, err = clientcmd.BuildConfigFromFlags("", s.TargetKubeconfig)
		if err != nil {
			return err
		}
	}

	var controlkubeconfig *rest.Config
	if s.ControlKubeconfig != "" {
		if s.ControlKubeconfig == "inClusterConfig" {
			// use inClusterConfig when controller is running inside cluster
			controlkubeconfig, err = clientcmd.BuildConfigFromFlags("", "")
		} else {
			// kubeconfig for the seed cluster where MachineCRDs are supposed to be registered.
			controlkubeconfig, err = clientcmd.BuildConfigFromFlags("", s.ControlKubeconfig)
		}
		if err != nil {
			return err
		}
	} else {
		if s.TargetKubeconfig == constants.TargetKubeconfigDisabledValue {
			return fmt.Errorf("--control-kubeconfig cannot be empty if --target-kubeconfig=%s is specified", constants.TargetKubeconfigDisabledValue)
		}
		controlkubeconfig = targetkubeconfig
	}

	// PROTOBUF WONT WORK
	// kubeconfig.ContentConfig.ContentType = s.ContentType
	// Override kubeconfig qps/burst settings from flags
	if targetkubeconfig != nil {
		targetkubeconfig.QPS = s.KubeAPIQPS
		targetkubeconfig.Burst = int(s.KubeAPIBurst)
	}
	controlkubeconfig.QPS = s.KubeAPIQPS
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

	run := func(_ context.Context) {
		var stop <-chan struct{}
		// Control plane client used to interact with machine APIs
		controlMachineClientBuilder := machineclientbuilder.SimpleClientBuilder{
			ClientConfig: controlkubeconfig,
		}
		// Control plane client used to interact with core kubernetes objects
		controlCoreClientBuilder := coreclientbuilder.SimpleControllerClientBuilder{
			ClientConfig: controlkubeconfig,
		}
		// Target plane client used to interact with core kubernetes objects
		var targetCoreClientBuilder coreclientbuilder.ClientBuilder
		if targetkubeconfig != nil {
			targetCoreClientBuilder = coreclientbuilder.SimpleControllerClientBuilder{
				ClientConfig: targetkubeconfig,
			}
		}

		err := StartControllers(
			s,
			controlkubeconfig,
			targetkubeconfig,
			controlMachineClientBuilder,
			controlCoreClientBuilder,
			targetCoreClientBuilder,
			driver,
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

// StartControllers starts all the controllers which are a part of machine-controller
func StartControllers(s *options.MCServer,
	controlCoreKubeconfig *rest.Config,
	targetCoreKubeconfig *rest.Config,
	controlMachineClientBuilder machineclientbuilder.ClientBuilder,
	controlCoreClientBuilder coreclientbuilder.ClientBuilder,
	targetCoreClientBuilder coreclientbuilder.ClientBuilder,
	driver driver.Driver,
	recorder record.EventRecorder,
	stop <-chan struct{}) error {

	klog.V(4).Info("Getting available resources")
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

	var (
		targetCoreClient        kubernetes.Interface
		targetKubernetesVersion *semver.Version
	)
	if targetCoreKubeconfig != nil {
		targetCoreKubeconfig = rest.AddUserAgent(targetCoreKubeconfig, controllerManagerAgentName)
		targetCoreClient, err = kubernetes.NewForConfig(targetCoreKubeconfig)
		if err != nil {
			klog.Fatal(err)
		}

		targetServerVersion, err := targetCoreClient.Discovery().ServerVersion()
		if err != nil {
			return err
		}
		targetKubernetesVersion, err = semver.NewVersion(targetServerVersion.GitVersion)
		if err != nil {
			return err
		}
	}

	if !availableResources[machineGVR] {
		return fmt.Errorf("unable to start machine controller: API GroupVersion %q is not available; \nFound: %#v", machineGVR, availableResources)
	}
	klog.V(4).Infof("Creating shared informers; resync interval: %v", s.MinResyncPeriod)

	controlMachineInformerFactory := machineinformers.NewFilteredSharedInformerFactory(
		controlMachineClientBuilder.ClientOrDie("control-machine-shared-informers"),
		s.MinResyncPeriod.Duration,
		s.Namespace,
		nil,
	)

	controlCoreInformerFactory := kubernetesinformers.NewFilteredSharedInformerFactory(
		controlCoreClientBuilder.ClientOrDie("control-core-shared-informers"),
		s.MinResyncPeriod.Duration,
		s.Namespace,
		nil,
	)

	var targetCoreInformerFactory kubernetesinformers.SharedInformerFactory
	if targetCoreClientBuilder != nil {
		targetCoreInformerFactory = kubernetesinformers.NewSharedInformerFactory(
			targetCoreClientBuilder.ClientOrDie("target-core-shared-informers"),
			s.MinResyncPeriod.Duration,
		)
	}

	// All shared informers are v1alpha1 API level
	machineSharedInformers := controlMachineInformerFactory.Machine().V1alpha1()

	klog.V(4).Infof("Creating controllers...")
	machineController, err := machinecontroller.NewController(
		s.Namespace,
		controlMachineClient,
		controlCoreClient,
		targetCoreClient,
		driver,
		targetCoreInformerFactory,
		controlCoreInformerFactory.Core().V1().Secrets(),
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

	controlMachineInformerFactory.Start(stop)
	controlCoreInformerFactory.Start(stop)
	if targetCoreInformerFactory != nil {
		targetCoreInformerFactory.Start(stop)
	}

	klog.V(4).Info("Running controller")
	go machineController.Run(int(s.ConcurrentNodeSyncs), stop)

	select {}
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
	utilruntime.Must(machinescheme.AddToScheme(kubescheme.Scheme))
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

	server := &http.Server{ // #nosec  G112 (CWE-400) -- Only used for metrics, profiling and health checks.
		Addr:    net.JoinHostPort(s.Address, strconv.Itoa(int(s.Port))),
		Handler: mux,
	}
	klog.Fatal(server.ListenAndServe())
}
