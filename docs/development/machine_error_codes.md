# Machine Error code handling

## Notational Conventions

The keywords "MUST", "MUST NOT", "REQUIRED", "SHALL", "SHALL NOT", "SHOULD", "SHOULD NOT", "RECOMMENDED", "NOT RECOMMENDED", "MAY", and "OPTIONAL" are to be interpreted as described in [RFC 2119](http://tools.ietf.org/html/rfc2119) (Bradner, S., "Key words for use in RFCs to Indicate Requirement Levels", BCP 14, RFC 2119, March 1997).

The key words "unspecified", "undefined", and "implementation-defined" are to be interpreted as described in the [rationale for the C99 standard](http://www.open-std.org/jtc1/sc22/wg14/www/C99RationaleV5.10.pdf#page=18).

An implementation is not compliant if it fails to satisfy one or more of the MUST, REQUIRED, or SHALL requirements for the protocols it implements.
An implementation is compliant if it satisfies all the MUST, REQUIRED, and SHALL requirements for the protocols it implements.

## Terminology

| Term            | Definition                                       |
|-----------------|--------------------------------------------------|
| CR              | Custom Resource (CR) is defined by a cluster admin using the Kubernetes Custom Resource Definition  primitive. |
| VM              | A Virtual Machine (VM) provisioned and managed by a provider. It could also refer to a physical machine in case of a bare metal provider.|
| Machine         | Machine refers to a VM that is provisioned/managed by MCM. It typically describes the metadata used to store/represent a Virtual Machine |
| Node            | Native kubernetes `Node` object. The objects you get to see when you do a "kubectl get nodes". Although nodes can be either physical/virtual machines, for the purposes of our discussions it refers to a VM. |
| MCM             | [Machine Controller Manager (MCM)](https://github.com/gardener/machine-controller-manager) is the controller used to manage higher level Machine Custom Resource (CR) such as machine-set and machine-deployment CRs. |
| Provider/Driver/MC      | `Provider` (or) `Driver` (or) `Machine Controller (MC)` is the driver responsible for managing machine objects present in the cluster from whom it manages these machines. A simple example could be creation/deletion of VM on the provider. |

## Pre-requisite

### MachineClass Resources

MCM introduces the CRD `MachineClass`. This is a blueprint for creating machines that join a certain cluster as nodes in a certain role. The provider only works with `MachineClass` resources that have the structure described here.

#### ProviderSpec

The `MachineClass` resource contains a `providerSpec` field that is passed in the `ProviderSpec` request field to CMI methods such as [CreateMachine](#createmachine). The `ProviderSpec` can be thought of as a machine template from which the VM specification must be adopted. It can contain key-value pairs of these specs. An example for these key-value pairs are given below.

| Parameter | Mandatory | Type | Description |
|---|---|---|---|
| `vmPool` | Yes | `string` | VM pool name, e.g. `TEST-WOKER-POOL` |
| `size` | Yes | `string` | VM size, e.g. `xsmall`, `small`, etc. Each size maps to a number of CPUs and memory size. |
| `rootFsSize` | No | `int` | Root (`/`) filesystem size in GB |
| `tags` | Yes | `map` | Tags to be put on the created VM |

Most of the `ProviderSpec` fields are not mandatory. If not specified, the provider passes an empty value in the respective `Create VM` parameter.

The `tags` can be used to map a VM to its corresponding machine object's Name

The `ProviderSpec` is validated by methods that receive it as a request field for presence of all mandatory parameters and tags, and for validity of all parameters.

#### Secrets

The `MachineClass` resource also contains a `secretRef` field that contains a reference to a secret. The keys of this secret are passed in the `Secrets` request field to CMI methods.

The secret can contain sensitive data such as
- `cloud-credentials` secret data used to authenticate at the provider
- `cloud-init` scripts used to initialize a new VM. The cloud-init script is expected to contain scripts to initialize the Kubelet and make it join the cluster.

#### Identifying Cluster Machines

To implement certain methods, the provider should be able to identify all machines associated with a particular Kubernetes cluster. This can be achieved using one/more of the below mentioned ways:

* Names of VMs created by the provider are prefixed by the cluster ID specified in the ProviderSpec.
* VMs created by the provider are tagged with the special tags like `kubernetes.io/cluster` (for the cluster ID) and `kubernetes.io/role` (for the role), specified in the ProviderSpec.
* Mapping `Resource Groups` to individual cluster.

### Error Scheme

All provider API calls defined in this spec MUST return a [machine error status](/pkg/util/provider/machinecodes/codes/codes.go), which is very similar to [standard machine status](https://github.com/grpc/grpc/blob/master/src/proto/grpc/status/status.proto).


### Machine Provider Interface

- The provider MUST have a unique way to map a `machine object` to a `VM` which triggers the deletion for the corresponding VM backing the machine object.
- The provider SHOULD have a unique way to map the `ProviderSpec` of a machine-class to a unique `Cluster`. This avoids deletion of other machines, not backed by the MCM.

#### `CreateMachine`

A Provider is REQUIRED to implement this interface method.
This interface method will be called by the MCM to provision a new VM on behalf of the requesting machine object.

- This call requests the provider to create a VM backing the machine-object.
- If VM backing the `Machine.Name` already exists, and is compatible with the specified `Machine` object in the `CreateMachineRequest`, the Provider MUST reply `0 OK` with the corresponding `CreateMachineResponse`.
- The provider can OPTIONALLY make use of the MachineClass supplied in the `MachineClass` in the `CreateMachineRequest` to communicate with the provider.
- The provider can OPTIONALLY make use of the secrets supplied in the `Secret` in the `CreateMachineRequest` to communicate with the provider.
- The provider can OPTIONALLY make use of the `Status.LastKnownState` in the `Machine` object to decode the state of the VM operation based on the last known state of the VM. This can be useful to restart/continue an operations which are mean't to be atomic.
- The provider MUST have a unique way to map a `machine object` to a `VM`. This could be implicitly provided by the provider by letting you set VM-names (or) could be explicitly specified by the provider using appropriate tags to map the same.
- This operation SHOULD be idempotent.

- The `CreateMachineResponse` returned by this method is expected to return
    - `ProviderID` that uniquely identifys the VM at the provider. This is expected to match with the node.Spec.ProviderID on the node object.
    - `NodeName` that is the expected name of the machine when it joins the cluster. It must match with the node name.
    - `LastKnownState` is an OPTIONAL field that can store details of the last known state of the VM. It can be used by future operation calls to determine current infrastucture state. This state is saved on the machine object.

```protobuf
// CreateMachine call is responsible for VM creation on the provider
CreateMachine(context.Context, *CreateMachineRequest) (*CreateMachineResponse, error)

// CreateMachineRequest is the create request for VM creation
type CreateMachineRequest struct {
	// Machine object from whom VM is to be created
	Machine *v1alpha1.Machine

	// MachineClass backing the machine object
	MachineClass *v1alpha1.MachineClass

	//  Secret backing the machineClass object
	Secret *corev1.Secret
}

// CreateMachineResponse is the create response for VM creation
type CreateMachineResponse struct {
	// ProviderID is the unique identification of the VM at the cloud provider.
	// ProviderID typically matches with the node.Spec.ProviderID on the node object.
	// Eg: gce://project-name/region/vm-ID
	ProviderID string

	// NodeName is the name of the node-object registered to kubernetes.
	NodeName string

	// LastKnownState represents the last state of the VM during an creation/deletion error
	LastKnownState string
}
```

##### CreateMachine Errors

If the provider is unable to complete the CreateMachine call successfully, it MUST return a non-ok ginterface method code in the machine status.
If the conditions defined below are encountered, the provider MUST return the specified machine error code.
The MCM MUST implement the specified error recovery behavior when it encounters the machine error code.

| machine Code | Condition | Description | Recovery Behavior | Auto Retry Required |
|-----------|-----------|-------------|-------------------|------------|
| 0 OK | Successful | The call was successful in creating/adopting a VM that matches supplied creation request. The `CreateMachineResponse` is returned with desired values |  | N |
| 1 CANCELED | Cancelled | Call was cancelled. Perform any pending clean-up tasks and return the call |  | N |
| 2 UNKNOWN | Something went wrong | Not enough information on what went wrong | Retry operation after sometime | Y |
| 3 INVALID_ARGUMENT | Re-check supplied parameters | Re-check the supplied `Machine.Name` and `ProviderSpec`. Make sure all parameters are in permitted range of values. Exact issue to be given in `.message` | Update providerSpec to fix issues. | N |
| 4 DEADLINE_EXCEEDED | Timeout | The call processing exceeded supplied deadline | Retry operation after sometime | Y |
| 6 ALREADY_EXISTS | Already exists but desired parameters doesn't match | Parameters of the existing VM don't match the ProviderSpec | Create machine with a different name | N |
| 7 PERMISSION_DENIED | Insufficent permissions | The requestor doesn't have enough permissions to create an VM and it's required dependencies | Update requestor permissions to grant the same | N |
| 8 RESOURCE_EXHAUSTED | Resource limits have been reached | The requestor doesn't have enough resource limits to process this creation request | Enhance resource limits associated with the user/account to process this | N |
| 9 PRECONDITION_FAILED | VM is in inconsistent state | The VM is in a state that is invalid for this operation | Manual intervention might be needed to fix the state of the VM | N |
| 10 ABORTED | Operation is pending | Indicates that there is already an operation pending for the specified machine | Wait until previous pending operation is processed | Y |
| 11 OUT_OF_RANGE | Resources were out of range  | The requested number of CPUs, memory size, of FS size in ProviderSpec falls outside of the corresponding valid range | Update request paramaters to request valid resource requests | N |
| 12 UNIMPLEMENTED | Not implemented | Unimplemented indicates operation is not implemented or not supported/enabled in this service. | Retry with an alternate logic or implement this method at the provider. Most methods by default are in this state | N |
| 13 INTERNAL | Major error | Means some invariants expected by underlying system has been broken. If you see one of these errors, something is very broken. | Needs manual intervension to fix this | N |
| 14 UNAVAILABLE | Not Available | Unavailable indicates the service is currently unavailable. | Retry operation after sometime | Y |
| 16 UNAUTHENTICATED | Missing provider credentials | Request does not have valid authentication credentials for the operation | Fix the provider credentials | N |

The status `message` MUST contain a human readable description of error, if the status `code` is not `OK`.
This string MAY be surfaced by MCM to end users.

#### `DeleteMachine`

A Provider is REQUIRED to implement this driver call.
This driver call will be called by the MCM to deprovision/delete/terminate a VM backed by the requesting machine object.

- If a VM corresponding to the specified machine-object's name does not exist or the artifacts associated with the VM do not exist anymore (after deletion), the Provider MUST reply `0 OK`.
- The provider SHALL only act on machines belonging to the cluster-id/cluster-name obtained from the `ProviderSpec`.
- The provider can OPTIONALY make use of the secrets supplied in the `Secrets` map in the `DeleteMachineRequest` to communicate with the provider.
- The provider can OPTIONALY make use of the `Spec.ProviderID` map in the `Machine` object.
- The provider can OPTIONALLY make use of the `Status.LastKnownState` in the `Machine` object to decode the state of the VM operation based on the last known state of the VM. This can be useful to restart/continue an operations which are mean't to be atomic.
- This operation SHOULD be idempotent.
- The provider must have a unique way to map a `machine object` to a `VM` which triggers the deletion for the corresponding VM backing the machine object.

- The `DeleteMachineResponse` returned by this method is expected to return
    - `LastKnownState` is an OPTIONAL field that can store details of the last known state of the VM. It can be used by future operation calls to determine current infrastucture state. This state is saved on the machine object.

```protobuf
// DeleteMachine call is responsible for VM deletion/termination on the provider
DeleteMachine(context.Context, *DeleteMachineRequest) (*DeleteMachineResponse, error)

// DeleteMachineRequest is the delete request for VM deletion
type DeleteMachineRequest struct {
	// Machine object from whom VM is to be deleted
	Machine *v1alpha1.Machine

	// MachineClass backing the machine object
	MachineClass *v1alpha1.MachineClass

	// Secret backing the machineClass object
	Secret *corev1.Secret
}

// DeleteMachineResponse is the delete response for VM deletion
type DeleteMachineResponse struct {
	// LastKnownState represents the last state of the VM during an creation/deletion error
	LastKnownState string
}
```

##### DeleteMachine Errors

If the provider is unable to complete the DeleteMachine call successfully, it MUST return a non-ok machine code in the machine status.
If the conditions defined below are encountered, the provider MUST return the specified machine error code.

| machine Code | Condition | Description | Recovery Behavior | Auto Retry Required |
|-----------|-----------|-------------|-------------------|------------|
| 0 OK | Successful | The call was successful in deleting a VM that matches supplied deletion request. |  | N |
| 1 CANCELED | Cancelled | Call was cancelled. Perform any pending clean-up tasks and return the call |  | N |
| 2 UNKNOWN | Something went wrong | Not enough information on what went wrong | Retry operation after sometime | Y |
| 3 INVALID_ARGUMENT | Re-check supplied parameters | Re-check the supplied `Machine.Name` and make sure that it is in the desired format and not a blank value. Exact issue to be given in `.message` | Update `Machine.Name` to fix issues. | N |
| 4 DEADLINE_EXCEEDED | Timeout | The call processing exceeded supplied deadline | Retry operation after sometime | Y |
| 7 PERMISSION_DENIED | Insufficent permissions | The requestor doesn't have enough permissions to delete an VM and it's required dependencies | Update requestor permissions to grant the same | N |
| 9 PRECONDITION_FAILED | VM is in inconsistent state | The VM is in a state that is invalid for this operation | Manual intervention might be needed to fix the state of the VM | N |
| 10 ABORTED | Operation is pending | Indicates that there is already an operation pending for the specified machine | Wait until previous pending operation is processed | Y |
| 12 UNIMPLEMENTED | Not implemented | Unimplemented indicates operation is not implemented or not supported/enabled in this service. | Retry with an alternate logic or implement this method at the provider. Most methods by default are in this state | N |
| 13 INTERNAL | Major error | Means some invariants expected by underlying system has been broken. If you see one of these errors, something is very broken. | Needs manual intervension to fix this | N |
| 14 UNAVAILABLE | Not Available | Unavailable indicates the service is currently unavailable. | Retry operation after sometime | Y |
| 16 UNAUTHENTICATED | Missing provider credentials | Request does not have valid authentication credentials for the operation | Fix the provider credentials | N |

The status `message` MUST contain a human readable description of error, if the status `code` is not `OK`.
This string MAY be surfaced by MCM to end users.

#### `GetMachineStatus`

A Provider can OPTIONALLY implement this driver call. Else should return a `UNIMPLEMENTED` status in error.
This call will be invoked by the MC to get the status of a machine.
This optional driver call helps in optimizing the working of the provider by avoiding unwanted calls to `CreateMachine()` and `DeleteMachine()`.

- If a VM corresponding to the specified machine object's `Machine.Name` exists on provider the `GetMachineStatusResponse` fields are to be filled similar to the `CreateMachineResponse`.
- The provider SHALL only act on machines belonging to the cluster-id/cluster-name obtained from the `ProviderSpec`.
- The provider can OPTIONALY make use of the secrets supplied in the `Secrets` map in the `GetMachineStatusRequest` to communicate with the provider.
- The provider can OPTIONALY make use of the VM unique ID (returned by the provider on machine creation) passed in the `ProviderID` map in the `GetMachineStatusRequest`.
- This operation MUST be idempotent.

```protobuf
// GetMachineStatus call get's the status of the VM backing the machine object on the provider
GetMachineStatus(context.Context, *GetMachineStatusRequest) (*GetMachineStatusResponse, error)

// GetMachineStatusRequest is the get request for VM info
type GetMachineStatusRequest struct {
	// Machine object from whom VM status is to be fetched
	Machine *v1alpha1.Machine

	// MachineClass backing the machine object
	MachineClass *v1alpha1.MachineClass

	//  Secret backing the machineClass object
	Secret *corev1.Secret
}

// GetMachineStatusResponse is the get response for VM info
type GetMachineStatusResponse struct {
	// ProviderID is the unique identification of the VM at the cloud provider.
	// ProviderID typically matches with the node.Spec.ProviderID on the node object.
	// Eg: gce://project-name/region/vm-ID
	ProviderID string

	// NodeName is the name of the node-object registered to kubernetes.
	NodeName string
}
```

##### GetMachineStatus Errors

If the provider is unable to complete the GetMachineStatus call successfully, it MUST return a non-ok machine code in the machine status.
If the conditions defined below are encountered, the provider MUST return the specified machine error code.

| machine Code | Condition | Description | Recovery Behavior | Auto Retry Required |
|-----------|-----------|-------------|-------------------|------------|
| 0 OK | Successful | The call was successful in getting machine details for given machine `Machine.Name` |  | N |
| 1 CANCELED | Cancelled | Call was cancelled. Perform any pending clean-up tasks and return the call |  | N |
| 2 UNKNOWN | Something went wrong | Not enough information on what went wrong | Retry operation after sometime | Y |
| 3 INVALID_ARGUMENT | Re-check supplied parameters | Re-check the supplied `Machine.Name` and make sure that it is in the desired format and not a blank value. Exact issue to be given in `.message` | Update `Machine.Name` to fix issues. | N |
| 4 DEADLINE_EXCEEDED | Timeout | The call processing exceeded supplied deadline | Retry operation after sometime | Y |
| 5 NOT_FOUND | Machine isn't found at provider | The machine could not be found at provider | Not required | N |
| 7 PERMISSION_DENIED | Insufficent permissions | The requestor doesn't have enough permissions to get details for the VM and it's required dependencies | Update requestor permissions to grant the same | N |
| 9 PRECONDITION_FAILED | VM is in inconsistent state | The VM is in a state that is invalid for this operation | Manual intervention might be needed to fix the state of the VM | N |
| 11 OUT_OF_RANGE | Multiple VMs found | Multiple VMs found with matching machine object names | Orphan VM handler to cleanup orphan VMs / Manual intervention maybe required if orphan VM handler isn't enabled.  | Y |
| 12 UNIMPLEMENTED | Not implemented | Unimplemented indicates operation is not implemented or not supported/enabled in this service. | Retry with an alternate logic or implement this method at the provider. Most methods by default are in this state | N |
| 13 INTERNAL | Major error | Means some invariants expected by underlying system has been broken. If you see one of these errors, something is very broken. | Needs manual intervension to fix this | N |
| 14 UNAVAILABLE | Not Available | Unavailable indicates the service is currently unavailable. | Retry operation after sometime | Y |
| 16 UNAUTHENTICATED | Missing provider credentials | Request does not have valid authentication credentials for the operation | Fix the provider credentials | N |

The status `message` MUST contain a human readable description of error, if the status `code` is not `OK`.
This string MAY be surfaced by MCM to end users.


#### `ListMachines`

A Provider can OPTIONALLY implement this driver call. Else should return a `UNIMPLEMENTED` status in error.
The Provider SHALL return the information about all the machines associated with the `MachineClass`.
Make sure to use appropriate filters to achieve the same to avoid data transfer overheads.
This optional driver call helps in cleaning up orphan VMs present in the cluster. If not implemented, any orphan VM that might have been created incorrectly by the MCM/Provider (due to bugs in code/infra) might require manual clean up.

- If the Provider succeeded in returning a list of `Machine.Name` with their corresponding `ProviderID`, then return `0 OK`.
- The `ListMachineResponse` contains a map of `MachineList` whose
    - Key is expected to contain the `ProviderID` &
    - Value is expected to contain the `Machine.Name` corresponding to it's kubernetes machine CR object
- The provider can OPTIONALY make use of the secrets supplied in the `Secrets` map in the `ListMachinesRequest` to communicate with the provider.

```protobuf
// ListMachines lists all the machines that might have been created by the supplied machineClass
ListMachines(context.Context, *ListMachinesRequest) (*ListMachinesResponse, error)

// ListMachinesRequest is the request object to get a list of VMs belonging to a machineClass
type ListMachinesRequest struct {
	// MachineClass object
	MachineClass *v1alpha1.MachineClass

	// Secret backing the machineClass object
	Secret *corev1.Secret
}

// ListMachinesResponse is the response object of the list of VMs belonging to a machineClass
type ListMachinesResponse struct {
	// MachineList is the map of list of machines. Format for the map should be <ProviderID, MachineName>.
	MachineList map[string]string
}
```

##### ListMachines Errors

If the provider is unable to complete the ListMachines call successfully, it MUST return a non-ok machine code in the machine status.
If the conditions defined below are encountered, the provider MUST return the specified machine error code.
The MCM MUST implement the specified error recovery behavior when it encounters the machine error code.

| machine Code | Condition | Description | Recovery Behavior | Auto Retry Required |
|-----------|-----------|-------------|-------------------|------------|
| 0 OK | Successful | The call for listing all VMs associated with `ProviderSpec` was successful. |  | N |
| 1 CANCELED | Cancelled | Call was cancelled. Perform any pending clean-up tasks and return the call |  | N |
| 2 UNKNOWN | Something went wrong | Not enough information on what went wrong | Retry operation after sometime | Y |
| 3 INVALID_ARGUMENT | Re-check supplied parameters | Re-check the supplied `ProviderSpec` and make sure that all required fields are present in their desired value format. Exact issue to be given in `.message` | Update `ProviderSpec` to fix issues. | N |
| 4 DEADLINE_EXCEEDED | Timeout | The call processing exceeded supplied deadline | Retry operation after sometime | Y |
| 7 PERMISSION_DENIED | Insufficent permissions | The requestor doesn't have enough permissions to list VMs and it's required dependencies | Update requestor permissions to grant the same | N |
| 12 UNIMPLEMENTED | Not implemented | Unimplemented indicates operation is not implemented or not supported/enabled in this service. | Retry with an alternate logic or implement this method at the provider. Most methods by default are in this state | N |
| 13 INTERNAL | Major error | Means some invariants expected by underlying system has been broken. If you see one of these errors, something is very broken. | Needs manual intervension to fix this | N |
| 14 UNAVAILABLE | Not Available | Unavailable indicates the service is currently unavailable. | Retry operation after sometime | Y |
| 16 UNAUTHENTICATED | Missing provider credentials | Request does not have valid authentication credentials for the operation | Fix the provider credentials | N |

The status `message` MUST contain a human readable description of error, if the status `code` is not `OK`.
This string MAY be surfaced by MCM to end users.

#### `GetVolumeIDs`

A Provider can OPTIONALLY implement this driver call. Else should return a `UNIMPLEMENTED` status in error.
This driver call will be called by the MCM to get the `VolumeIDs` for the list of `PersistantVolumes (PVs)` supplied.
This OPTIONAL (but recommended) driver call helps in serailzied eviction of pods with PVs while draining of machines. This implies applications backed by PVs would be evicted one by one, leading to shorter application downtimes.

- On succesful returnal of a list of `Volume-IDs` for all supplied `PVSpecs`, the Provider MUST reply `0 OK`.
- The `GetVolumeIDsResponse` is expected to return a repeated list of `strings` consisting of the `VolumeIDs` for `PVSpec` that could be extracted.
- If for any `PV` the Provider wasn't able to identify the `Volume-ID`, the provider MAY chose to ignore it and return the `Volume-IDs` for the rest of the `PVs` for whom the `Volume-ID` was found.
- Getting the `VolumeID` from the `PVSpec` depends on the Cloud-provider. You can extract this information by parsing the `PVSpec` based on the `ProviderType`
    - https://github.com/kubernetes/api/blob/release-1.15/core/v1/types.go#L297-L339
    - https://github.com/kubernetes/api/blob/release-1.15//core/v1/types.go#L175-L257
- This operation MUST be idempotent.

```protobuf
// GetVolumeIDsRequest is the request object to get a list of VolumeIDs for a PVSpec
type GetVolumeIDsRequest struct {
	// PVSpecsList is a list of PV specs for whom volume-IDs are required
	// Plugin should parse this raw data into pre-defined list of PVSpecs
	PVSpecs []*corev1.PersistentVolumeSpec
}

// GetVolumeIDsResponse is the response object of the list of VolumeIDs for a PVSpec
type GetVolumeIDsResponse struct {
	// VolumeIDs is a list of VolumeIDs.
	VolumeIDs []string
}
```

##### GetVolumeIDs Errors

| machine Code | Condition | Description | Recovery Behavior | Auto Retry Required |
|-----------|-----------|-------------|-------------------|------------|
| 0 OK | Successful | The call getting list of `VolumeIDs` for the list of `PersistantVolumes` was successful. |  | N |
| 1 CANCELED | Cancelled | Call was cancelled. Perform any pending clean-up tasks and return the call |  | N |
| 2 UNKNOWN | Something went wrong | Not enough information on what went wrong | Retry operation after sometime | Y |
| 3 INVALID_ARGUMENT | Re-check supplied parameters | Re-check the supplied `PVSpecList` and make sure that it is in the desired format. Exact issue to be given in `.message` | Update `PVSpecList` to fix issues. | N |
| 4 DEADLINE_EXCEEDED | Timeout | The call processing exceeded supplied deadline | Retry operation after sometime | Y |
| 12 UNIMPLEMENTED | Not implemented | Unimplemented indicates operation is not implemented or not supported/enabled in this service. | Retry with an alternate logic or implement this method at the provider. Most methods by default are in this state | N |
| 13 INTERNAL | Major error | Means some invariants expected by underlying system has been broken. If you see one of these errors, something is very broken. | Needs manual intervension to fix this | N |
| 14 UNAVAILABLE | Not Available | Unavailable indicates the service is currently unavailable. | Retry operation after sometime | Y |

The status `message` MUST contain a human readable description of error, if the status `code` is not `OK`.
This string MAY be surfaced by MCM to end users.

#### `GenerateMachineClassForMigration`

A Provider SHOULD implement this driver call, else it MUST return a `UNIMPLEMENTED` status in error.
This driver call will be called by the Machine Controller to try to perform a machineClass migration for an unknown machineClass Kind. This helps in migration of one kind of machineClass to another kind. For instance an machineClass custom resource of `AWSMachineClass` to `MachineClass`.

- On successful generation of machine class the Provider MUST reply `0 OK` (or) `nil` error.
- `GenerateMachineClassForMigrationRequest` expects the provider-specific machine class (eg. AWSMachineClass)
to be supplied as the `ProviderSpecificMachineClass`. The provider is responsible for unmarshalling the golang struct. It also passes a reference to an existing `MachineClass` object.
- The provider is expected to fill in this`MachineClass` object based on the conversions.
- An optional `ClassSpec` containing the `type ClassSpec struct` is also provided to decode the provider info.
- `GenerateMachineClassForMigration` is only responsible for filling up the passed `MachineClass` object. 
- The task of creating the new `CR` of the new kind (MachineClass) with the same name as the previous one and also annotating the old machineClass CR with a migrated annotation and migrating existing references is done by the calling library implicitly.
- This operation MUST be idempotent.

```protobuf
// GenerateMachineClassForMigrationRequest is the request for generating the generic machineClass
// for the provider specific machine class
type GenerateMachineClassForMigrationRequest struct {
	// ProviderSpecificMachineClass is provider specfic machine class object.
	// E.g. AWSMachineClass
	ProviderSpecificMachineClass interface{}
	// MachineClass is the machine class object generated that is to be filled up
	MachineClass *v1alpha1.MachineClass
	// ClassSpec contains the class spec object to determine the machineClass kind
	ClassSpec *v1alpha1.ClassSpec
}

// GenerateMachineClassForMigrationResponse is the response for generating the generic machineClass
// for the provider specific machine class
type GenerateMachineClassForMigrationResponse struct{}
```

##### MigrateMachineClass Errors

| machine Code | Condition | Description | Recovery Behavior | Auto Retry Required |
|-----------|-----------|-------------|-------------------|------------|
| 0 OK | Successful | Migration of provider specific machine class was successful | Machine reconcilation is retried once the new class has been created | Y |
| 12 UNIMPLEMENTED | Not implemented | Unimplemented indicates operation is not implemented or not supported/enabled in this provider. | None | N |
| 13 INTERNAL | Major error | Means some invariants expected by underlying system has been broken. If you see one of these errors, something is very broken. | Might need manual intervension to fix this | Y |

The status `message` MUST contain a human readable description of error, if the status `code` is not `OK`.
This string MAY be surfaced by MCM to end users.

## Configuration and Operation

### Supervised Lifecycle Management

* For Providers packaged in software form:
  * Provider Packages SHOULD use a well-documented container image format (e.g., Docker, OCI).
  * The chosen package image format MAY expose configurable Provider properties as environment variables, unless otherwise indicated in the section below.
    Variables so exposed SHOULD be assigned default values in the image manifest.
  * A Provider Supervisor MAY programmatically evaluate or otherwise scan a Provider Package’s image manifest in order to discover configurable environment variables.
  * A Provider SHALL NOT assume that an operator or Provider Supervisor will scan an image manifest for environment variables.

#### Environment Variables

* Variables defined by this specification SHALL be identifiable by their `MC_` name prefix.
* Configuration properties not defined by the MC specification SHALL NOT use the same `MC_` name prefix; this prefix is reserved for common configuration properties defined by the MC specification.
* The Provider Supervisor SHOULD supply all RECOMMENDED MC environment variables to a Provider.
* The Provider Supervisor SHALL supply all REQUIRED MC environment variables to a Provider.

##### Logging

* Providers SHOULD generate log messages to ONLY standard output and/or standard error.
  * In this case the Provider Supervisor SHALL assume responsibility for all log lifecycle management.
* Provider implementations that deviate from the above recommendation SHALL clearly and unambiguously document the following:
  * Logging configuration flags and/or variables, including working sample configurations.
  * Default log destination(s) (where do the logs go if no configuration is specified?)
  * Log lifecycle management ownership and related guidance (size limits, rate limits, rolling, archiving, expunging, etc.) applicable to the logging mechanism embedded within the Provider.
* Providers SHOULD NOT write potentially sensitive data to logs (e.g. secrets).

##### Available Services

* Provider Packages MAY support all or a subset of CMI services; service combinations MAY be configurable at runtime by the Provider Supervisor.
  * This specification does not dictate the mechanism by which mode of operation MUST be discovered, and instead places that burden upon the VM Provider.
* Misconfigured provider software SHOULD fail-fast with an OS-appropriate error code.

##### Linux Capabilities

* Providers SHOULD clearly document any additionally required capabilities and/or security context.

##### Cgroup Isolation

* A Provider MAY be constrained by cgroups.

##### Resource Requirements

* VM Providers SHOULD unambiguously document all of a Provider’s resource requirements.

### Deploying
* **Recommended:** The MCM and Provider are typically expected to run as two containers inside a common `Pod`.
* However, for the security reasons they could execute on seperate Pods provided they have a secure way to exchange data between them.
