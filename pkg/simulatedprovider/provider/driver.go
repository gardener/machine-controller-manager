// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package provider

import (
	"context"
	"fmt"
	"sync"
	"time"

	machineclientset "github.com/gardener/machine-controller-manager/pkg/client/clientset/versioned"
	machineclientbuilder "github.com/gardener/machine-controller-manager/pkg/util/clientbuilder/machine"
	"github.com/gardener/machine-controller-manager/pkg/util/provider/driver"
	"github.com/gardener/machine-controller-manager/pkg/util/provider/machinecodes/codes"
	"github.com/gardener/machine-controller-manager/pkg/util/provider/machinecodes/status"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
)

const (
	clientNameMachineController = "machine-controller"

	labelCreateMachine     = "create_machine"
	labelInitializeMachine = "initialize_machine"
	labelDeleteMachine     = "delete_machine"
	labelListMachines      = "list_machine"
	labelGetMachineStatus  = "get_machine_status"
	labelGetVolumeIDs      = "get_volume_ids"
)

// Ref: ../../../docs/development/machine_error_codes.md
// Ref: ../../util/provider/driver/driver.go
var _ driver.Driver = &DriverImpl{}

// DriverImpl is the struct that implements the MCM driver.Driver interface
// It also maintains a map `managedNodes` of all the nodes currently managed by MCM
type DriverImpl struct {
	client        *kubernetes.Clientset
	machineClient machineclientset.Interface
	// Map from node.Name to managedNodeInfo
	managedNodes sync.Map
}

type managedNodeInfo struct {
	ProviderID       string
	MachineClassName string
}

// NewDriver returns a DriverImpl object containing the client, machineclient and a map
// of nodes managed by MCM.
func NewDriver(ctx context.Context, kubecfg string) (driver.Driver, error) {
	config, err := clientcmd.BuildConfigFromFlags("", kubecfg)
	if err != nil {
		return nil, fmt.Errorf("cannot create rest.Config from kubeconfig %q: %w", kubecfg, err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("cannot create clientset from kubeconfig %q: %w", kubecfg, err)
	}

	mcb := machineclientbuilder.SimpleClientBuilder{
		ClientConfig: config,
	}
	machineClient, err := mcb.Client(clientNameMachineController)
	if err != nil {
		return nil, fmt.Errorf("cannot create machine client: %w", err)
	}

	d := &DriverImpl{
		client:        clientset,
		machineClient: machineClient,
	}

	err = d.initializeMCMManagedNodes(ctx)
	if err != nil {
		return nil, err
	}
	return d, nil
}

// CreateMachine handles a machine creation request. For the simulated provider, it creates
// a node and attempts to transition it to 'Ready' state.
func (d *DriverImpl) CreateMachine(ctx context.Context, req *driver.CreateMachineRequest) (resp *driver.CreateMachineResponse, err error) {
	defer driverAPIMetricRecorderFn(labelCreateMachine, &err)()

	var node *corev1.Node
	if _, found := d.managedNodes.Load(req.Machine.Name); !found {
		if req.MachineClass.NodeTemplate == nil {
			err = fmt.Errorf("cannot build a node as `machineClass.NodeTemplate` is nil")
			return
		}
		node = d.buildNode(req.Machine, req.MachineClass)
		_, err = d.client.CoreV1().Nodes().Create(ctx, node, metav1.CreateOptions{})
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		klog.Infof("Created node %q", node.Name)
		// The node is considered 'managed' even when it hasn't transitioned to 'Ready'.
		d.managedNodes.Store(node.Name, managedNodeInfo{
			ProviderID:       node.Spec.ProviderID,
			MachineClassName: req.MachineClass.Name,
		})
	}

	err = d.transitionNodeToReady(ctx, req.Machine.Name)
	if err != nil {
		return
	}

	resp = &driver.CreateMachineResponse{
		ProviderID:     node.Spec.ProviderID,
		NodeName:       node.Name,
		LastKnownState: fmt.Sprintf("Instance %q created at %q", node.Name, time.Now()),
	}
	return
}

// InitializeMachine handles VM initialization. Currently, un-implemented.
func (d *DriverImpl) InitializeMachine(_ context.Context, _ *driver.InitializeMachineRequest) (resp *driver.InitializeMachineResponse, err error) {
	defer driverAPIMetricRecorderFn(labelInitializeMachine, &err)()
	err = status.Error(
		codes.Unimplemented, "simulation provider does not implement InitializeMachine",
	)
	return
}

// DeleteMachine handles a machine deletion request. For the simulated provider, it just
// removes the corresponding node from the managedNodes map.
func (d *DriverImpl) DeleteMachine(_ context.Context, req *driver.DeleteMachineRequest) (resp *driver.DeleteMachineResponse, err error) {
	defer driverAPIMetricRecorderFn(labelDeleteMachine, &err)()

	d.managedNodes.Delete(req.Machine.Name)

	resp = &driver.DeleteMachineResponse{}
	return
}

// GetMachineStatus handles a machine get status request.
func (d *DriverImpl) GetMachineStatus(_ context.Context, req *driver.GetMachineStatusRequest) (resp *driver.GetMachineStatusResponse, err error) {
	defer driverAPIMetricRecorderFn(labelGetMachineStatus, &err)()

	nodeInfo, ok := d.managedNodes.Load(req.Machine.Name)
	if !ok {
		err = status.Error(
			codes.NotFound, fmt.Sprintf("instance %q not found", req.Machine.Name),
		)
		return
	}

	resp = &driver.GetMachineStatusResponse{
		NodeName:   req.Machine.Name,
		ProviderID: nodeInfo.(managedNodeInfo).ProviderID,
	}
	return
}

// ListMachines lists all the machines possibly created by a machineClass.
// Returns a `map[providerID]machineName` for the specified `MachineClass`.
func (d *DriverImpl) ListMachines(_ context.Context, req *driver.ListMachinesRequest) (resp *driver.ListMachinesResponse, err error) {
	defer driverAPIMetricRecorderFn(labelListMachines, &err)()

	resp = &driver.ListMachinesResponse{
		MachineList: make(map[string]string),
	}

	d.managedNodes.Range(func(nodeName, nodeInfoObj any) bool {
		nodeInfo, ok := nodeInfoObj.(managedNodeInfo)
		if ok && nodeInfo.MachineClassName == req.MachineClass.Name {
			resp.MachineList[nodeInfo.ProviderID] = nodeName.(string)
		}
		return true
	})
	return
}

// GetVolumeIDs returns a list of Volume IDs for all PV Specs for whom a provider volume was found. Currently, un-implemented.
func (d *DriverImpl) GetVolumeIDs(_ context.Context, _ *driver.GetVolumeIDsRequest) (resp *driver.GetVolumeIDsResponse, err error) {
	defer driverAPIMetricRecorderFn(labelGetVolumeIDs, &err)()
	err = status.Error(
		codes.Unimplemented, "simulation provider does not implement GetVolumeIDs",
	)
	return
}
