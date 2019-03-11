# machine-controller-manager

[![Go Report Card](https://goreportcard.com/badge/github.com/gardener/machine-controller-manager)](https://goreportcard.com/report/github.com/gardener/machine-controller-manager)

Machine Controller Manager (MCM) manages VMs as another kubernetes custom resource. It provides a declarative way to manage VMs. The current implementation supports AWS, GCP, Azure, Alicloud, Packet and Openstack. It can easily be extended to support other cloud providers as well.

Example of managing machine:
```
kubectl create/get/delete machine vm1
```

### Key terminologies

Nodes/Machines/VMs are different terminologies used to represent similar things. We use these terms in the following way

1. VM: A virtual machine running on any cloud provider.
1. Node: Native kubernetes node objects. The objects you get to see when you do a *"kubectl get nodes"*. Although nodes can be either physical/virtual machines, for the purposes of our discussions it refers to a VM.
1. Machine: A VM that is provisioned/managed by the Machine Controller Manager.

## Design of Machine Controller Manager

See the design documentation in the `/docs/design` repository, please [find the design doc here](docs/design/README.md).

## To start using or developing the Machine Controller Manager

See the documentation in the `/docs` repository, please [find the index here](docs/README.md).

## Cluster-api Implementation
- `cluster-api` branch of machine-controller-manager implements the machine-api aspect of the [cluster-api project](https://github.com/kubernetes-sigs/cluster-api).
- Link: https://github.com/gardener/machine-controller-manager/tree/cluster-api
- Once cluster-api project gets stable, we may make `master` branch of MCM as well cluster-api compliant, with well-defined migration notes.
