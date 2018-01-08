# Node-Controller-Manager

Node Controller Manager (NCM) manages VMs as another kubernetes custom resource. It provides a declarative way to manage VMs. The current implementation only supports AWS but can be (and will be) easily extended to support other cloud providers.

Example of managing VM:
```
kubectl create/get/delete instance vm1
``` 

### Key terminologies

Nodes/Instances/Machines/VMs are different terminologies used to represent similar things. We use these terms in the following way

1. VM: A virtual machine running on any cloud provider.
1. Node: Native kubernetes node objects. The objects you get to see when you do a *"kubectl get nodes"*. Although nodes can be either physical/virtual machines, for the purposes of our discussions it refers to a VM. 
1. Instance: A VM that is provisioned/managed by the Node Controller Manager.
1. Machine: Community's terminology for an Instance. The term instance shall be replaced by machines in the near future.

## Design of Node Controller Manager

See the design documentation in the `/docs/design` repository, please [find the design doc here](docs/design/README.md).

## To start using or developing the Node Controller Manager

See the documentation in the `/docs` repository, please [find the index here](docs/README.md).
