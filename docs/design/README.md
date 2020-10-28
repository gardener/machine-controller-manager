# Design of Machine Controller Manager

<!-- TOC -->

- [Design of Machine Controller Manager](#design-of-machine-controller-manager)
	- [Design Principles](#design-principles)
	- [Objects of Machine Controller Manager](#objects-of-machine-controller-manager)
	- [Components of Machine Controller Manager](#components-of-machine-controller-manager)
	- [Future Plans](#future-plans)
		- [Todos Doc](#todos-doc)

<!-- /TOC -->

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

Machine Controller Manager makes use of 4 CRD objects and 1 Kubernetes secret object to manage machines. They are as follows,
1. `MachineClass`: Represents a template that contains cloud provider specific details used to create machines.
1. `Machine`: Represents a VM which is backed by the cloud provider.
1. `MachineSet`: Represents a group of machines managed by the Machine Controller Manager.
1. `MachineDeployment`: Represents a group of machine-sets managed by the Machine Controller Manager to allow updating machines.
1. `Secret`: Represents a Kubernetes secret that stores cloudconfig (initialization scripts used to create VMs) and cloud specific credentials

## Components of Machine Controller Manager

Machine Controller Manager is made up of 3 sub-controllers as of now. They are -
1. `Machine` Controller: Used to create/update/delete machines. It is the only controller which actually talks to the cloud providers.
1. `MachineSet` Controller: Used to manage `MachineSets`. This controller ensures that desired number of machines are always up and running healthy.
1. `MachineDeployment` Controller: Used to update machines from one version to another by manipulating the `MachineSet` objects.
1. Machine Safety Controller: A safety net controller that terminates orphan VMs and freezes `MachineSet`/`MachineDeployment` objects which are overshooting or timing out while trying to join nodes to the cluster.

All these controllers work in an co-operative manner. They form a parent-child relationship with `MachineDeployment` Controller being the grandparent, `MachineSet` Controller being the parent, and `Machine` Controller being the child.
