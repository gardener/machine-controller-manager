# machine-controller-manager

[![CI Build status](https://concourse.ci.gardener.cloud/api/v1/teams/gardener/pipelines/machine-controller-manager-master/jobs/master-head-update-job/badge)](https://concourse.ci.gardener.cloud/teams/gardener/pipelines/machine-controller-manager-master/jobs/master-head-update-job)
[![Go Report Card](https://goreportcard.com/badge/github.com/gardener/machine-controller-manager)](https://goreportcard.com/report/github.com/gardener/machine-controller-manager)

**Note**
One can add support for a new cloud provider by following [Adding support for new provider](https://github.com/gardener/machine-controller-manager/blob/master/docs/development/cp_support_new.md). 

# Overview

Machine Controller Manager aka MCM is a group of cooperative controllers that manage the lifecycle of the worker machines. It is inspired by the design of Kube Controller Manager in which various sub controllers manage their respective Kubernetes Clients. MCM gives you the following benefits:

- seamlessly manage machines/nodes with a declarative API (of course, across different cloud providers)
- integrate generically with the cluster autoscaler
- plugin with tools such as the node-problem-detector
- transport the immutability design principle to machine/nodes
- implement e.g. rolling upgrades of machines/nodes

MCM supports following providers. These provider code is maintained externally (out-of-tree), and the links for the same are linked below: 
* [Alicloud](https://github.com/gardener/machine-controller-manager-provider-alicloud)
* [AWS](https://github.com/gardener/machine-controller-manager-provider-aws)
* [Azure](https://github.com/gardener/machine-controller-manager-provider-azure)
* [Equinix Metal](https://github.com/gardener/machine-controller-manager-provider-equinix-metal)
* [GCP](https://github.com/gardener/machine-controller-manager-provider-gcp)
* [KubeVirt](https://github.com/gardener/machine-controller-manager-provider-kubevirt)
* [Metal Stack](https://github.com/metal-stack/machine-controller-manager-provider-metal)
* [Openstack](https://github.com/gardener/machine-controller-manager-provider-openstack)
* [V Sphere](https://github.com/gardener/machine-controller-manager-provider-vsphere)
* [Yandex](https://github.com/gardener/machine-controller-manager-provider-yandex)

It can easily be extended to support other cloud providers as well.

Example of managing machine:
```
kubectl create/get/delete machine vm1
```

## Key terminologies

Nodes/Machines/VMs are different terminologies used to represent similar things. We use these terms in the following way

1. VM: A virtual machine running on any cloud provider. It could also refer to a physical machine (PM) in case of a bare metal setup.
1. Node: Native kubernetes node objects. The objects you get to see when you do a *"kubectl get nodes"*. Although nodes can be either physical/virtual machines, for the purposes of our discussions it refers to a VM.
1. Machine: A VM that is provisioned/managed by the Machine Controller Manager.

# Design of Machine Controller Manager

The design of the Machine Controller Manager is influenced by the Kube Controller Manager, where-in multiple sub-controllers are used to manage the Kubernetes clients.

## Design Principles

It's designed to run in the master plane of a Kubernetes cluster. It follows the best principles and practices of writing controllers, including, but not limited to:

- Reusing code from kube-controller-manager
- leader election to allow HA deployments of the controller
- `workqueues` and multiple thread-workers
- `SharedInformers` that limit to minimum network calls, de-serialization and provide helpful create/update/delete events for resources
- rate-limiting to allow back-off in case of network outages and general instability of other cluster components
- sending events to respected resources for easy debugging and overview
- Prometheus metrics, health and (optional) profiling endpoints

## Objects of Machine Controller Manager

Machine Controller Manager reconciles a set of Custom Resources namely `MachineDeployment`, `MachineSet` and `Machines` which are managed & monitored by their controllers MachineDeployment Controller, MachineSet Controller, Machine Controller respectively along with another cooperative controller called the Safety Controller.

Machine Controller Manager makes use of 4 CRD objects and 1 Kubernetes secret object to manage machines. They are as follows:

| Custom ResourceObject | Description |
| --- | --- |
| `MachineClass`| A `MachineClass` represents a template that contains cloud provider specific details used to create machines.|
| `Machine`| A `Machine` represents a VM which is backed by the cloud provider.|
| `MachineSet` | A `MachineSet` ensures that the specified number of `Machine` replicas are running at a given point of time.|
| `MachineDeployment`| A `MachineDeployment` provides a declarative update for `MachineSet` and `Machines`.|
| `Secret`| A `Secret` here is a Kubernetes secret that stores cloudconfig (initialization scripts used to create VMs) and cloud specific credentials.|

See [here](docs/documents/apis.md) for CRD API Documentation


## Components of Machine Controller Manager

<table>
    <thead>
        <tr>
            <th>Controller</th>
            <th>Description</th>
        </tr>
    </thead>
    <tbody>
        <tr>
            <td>MachineDeployment controller</td>
            <td>Machine Deployment controller reconciles the <code>MachineDeployment</code> objects and manages the lifecycle of <code>MachineSet</code> objects. <code>MachineDeployment</code> consumes provider specific <code>MachineClass</code> in its <code>spec.template.spec</code> which is the template of the VM spec that would be spawned on the cloud by MCM.</td>
        </tr>
        <tr>
            <td>MachineSet controller</td>
            <td>MachineSet controller reconciles the <code>MachineSet</code> objects and manages the lifecycle of <code>Machine</code> objects.</td>
        </tr>
        <tr>
            <td>Safety controller</td>
            <td>There is a Safety Controller responsible for handling the unidentified or unknown behaviours from the cloud providers. Safety Controller:
                <ul>
                    <li>
                        freezes the MachineDeployment controller and MachineSet controller if the number of <code>Machine</code> objects goes beyond a certain threshold on top of <code>Spec.replicas</code>. It can be configured by the flag <code>--safety-up</code> or <code>--safety-down</code> and also <code>--machine-safety-overshooting-period`</code>.
                    </li>
                    <li>
                        freezes the functionality of the MCM if either of the <code>target-apiserver</code> or the <code>control-apiserver</code> is not reachable.
                    </li>
                    <li>
                        unfreezes the MCM automatically once situation is resolved to normal. A <code>freeze</code> label is applied on <code>MachineDeployment</code>/<code>MachineSet</code> to enforce the freeze condition.
                    </li>
                </ul>
            </td>
        </tr>
    </tbody>
</table>

Along with the above Custom Controllers and Resources, MCM requires the `MachineClass` to use K8s `Secret` that stores cloudconfig (initialization scripts used to create VMs) and cloud specific credentials. All these controllers work in an co-operative manner. They form a parent-child relationship with `MachineDeployment` Controller being the grandparent, `MachineSet` Controller being the parent, and `Machine` Controller being the child.


## Development

To start using or developing the Machine Controller Manager, see the documentation in the `/docs` repository.

## FAQ
An FAQ is available [here](docs/FAQ.md).

## cluster-api Implementation
- `cluster-api` branch of machine-controller-manager implements the machine-api aspect of the [cluster-api project](https://github.com/kubernetes-sigs/cluster-api).
- Link: https://github.com/gardener/machine-controller-manager/tree/cluster-api
- Once cluster-api project gets stable, we may make `master` branch of MCM as well cluster-api compliant, with well-defined migration notes.
