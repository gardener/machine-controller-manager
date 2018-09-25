# GRPC based implementation of Cloud Providers - WIP

## Goal:
Currently the Cloud Providers' functionalities ( Create(), Delete(), List() ) are part of the Machine Controller Manager's (MCM) repository. Because of this, adding support for new Cloud Providers into MCM requires merging code into MCM which may not be required for core functionalities of MCM itself. Also, for various reasons it may not be feasible for all Cloud Providers to merge their code with MCM which is an Open Source project.

Because of these reasons, it was decided that the Cloud Providers' code will be moved out in separate repositories so that they can be maintained separately by the respective teams. Idea is to make MCM act as a GRPC server, and Cloud Providers as GRPC clients. The Cloud Provider clients can register themselves with the MCM using a GRPC service exposed by the MCM. Details of this approach is discussed below.

## How it works:
MCM acts as GRPC server and listens on a pre-defined port 5000. It implements below GRPC services. Details of each of these services are mentioned in next section.
* `Register()`
* `GetMachineClass()`
* `GetSecret()`

## GRPC services exposed by MCM:

### Register()
`rpc Register(stream DriverSide) returns (stream MCMside) {}`

The Cloud Provider GRPC client calls this service to register itself with the MCM. The Cloud Provider passes the `kind` and the `APIVersion` which it implements, and MCM maintains an internal map for all the registered clients. A GRPC stream is returned in response which is kept open througout the life of both the processes. MCM uses this stream to communicate with the client  for machine operations: `Create()`, `Delete()` or `List()`.
The Cloud Provider client is responsible for reading the incoming messages continuously, and based on the `operationType` parameter embedded in the message, it is supposed to take the required action. This part is already handled in the package `driver/grpc/client`.
To add a new Cloud Provider support to MCM, import the package, and implement the `ExternalDriverProvider` interface:

```
type ExternalDriverProvider interface {
	Create(machineclass *MachineClassMeta, credentials, machineID, machineName string) (string, string, error)
	Delete(machineclass *MachineClassMeta, credentials, machineID string) error
	List(machineclass *MachineClassMeta, credentials, machineID string) (map[string]string, error)
}
```

### GetMachineClass()
`rpc GetMachineClass(MachineClassMeta) returns (MachineClass) {}`

As part of the message from MCM for various machine operations, the name of the machine class is sent instead of the full machine class spec. The Cloud Provider client is expected to use this GRPC service to get the full spec of the machine class. This optionally enables the client to cache the machine class spec, and make the call only if the machine calass spec is not already cached.

### GetSecret()
`rpc GetSecret(SecretMeta) returns (Secret) {}`

As part of the message from MCM for various machine operations, the Cloud Config (CC) and Cloud Provider credentials are not sent. The Cloud Provider client is expected to use this GRPC service to get the secret which has CC and Cloud Provider's credentials from MCM. This enables the client to cache the CC and credentials, and to make the call only if the data is not already cached.

## How to add a new Cloud Provider's support
Import the package `driver/grpc/client` and `driver/grpc/service` from MCM (currently in MCM's "grpc-driver" branch)
* Implement the interface `ExternalDriverProvider`
    * `Create()`: Creates a new machine
    * `Delete()`: Deletes a machine
    * `List()`: Lists machines
* Use the interface `MachineClassDataProvider`
    * `GetMachineClass()`: Makes the call to MCM to get machine class spec
    * `GetSecret()`: Makes the call to MCM to get secret containing Cloud Config and Cloud Provider's credentials

### Example implementation:
This approach is implemented in MCM's "grpc-driver" branch:
https://github.com/gardener/machine-controller-manager/tree/grpc-driver

Refer GRPC based implementation for AWS client:
https://github.com/ggaurav10/aws-driver-grpc

## Alternatives:
### Cloud Provider Driver as GRPC server:
One approach would be to make MCM as a GRPC client and Cloud Provider as a GRPC server.
Here the Cloud Provider will provide the following GRPC services:
1. `Create()` creates a VM
1. `Delete()` deletes a VM
1. `List()` lists VMs

To provide machine class and secret for Cloud Provider, MCM would need to provide methods `GetMachineClass()` and `GetSecret()`. This can be implemented in 2 ways:
* By MCM acting as GRPC server also, and providing these methods as GRPC services. The Cloud Provider driver will also act as GRPC client and use these services to fetch data
* By driver server providing a GRPC stream to MCM client, so that the driver can send back GRPC messages to MCM requesting the machine class and secret. MCM will then respond with these details using the same stream

A disadvantage of this approach would be that each Cloud Provider will be opening a different port in the system which increases its attack surface.

### Cloud Provider Driver as Webhook:
Another approach would be to use webhooks instead of Cloud Provider code running in the system as another pod or a sidecar. This would require the Cloud Provider to contact API server to read machine class and secret. A better approach might be to implement a controller instead of webhooks. This brings us to next possible approach.

### Cloud Provider Driver as a controller:
Idea is to separate machine controller from rest of the controllers (machine set, machine deployment and safety controllers) and integrate it with the Cloud Provider's code. Basically, each Cloud Provider also implements its own machine controller which will have access to all the information (machine class and secret details) required to do Cloud operations.

This approach is currently a defalut approach in the Cluster API.
This approach is not invalidated by our implementation. Our implementation gives another (potentially, easier) way to achieve the same goals while retaining the possibility of using a separate dedicated machine controller for a given Cloud Provider.

#### Pros and cons of different approaches are further discussed [here] (https://github.com/gardener/machine-controller-manager/issues/178)
