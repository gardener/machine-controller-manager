# GRPC based implementation of Cloud Providers - WIP

## Goal:
Currently the Cloud Providers' (CP) functionalities ( Create(), Delete(), List() ) are part of the Machine Controller Manager's (MCM)repository. Because of this, adding support for new CPs into MCM requires merging code into MCM which may not be required for core functionalities of MCM itself. Also, for various reasons it may not be feasible for all CPs to merge their code with MCM which is an Open Source project.

Because of these reasons, it was decided that the CP's code will be moved out in separate repositories so that they can be maintained separately by the respective teams. Idea is to make MCM act as a GRPC server, and CPs as GRPC clients. The CP can register themselves with the MCM using a GRPC service exposed by the MCM. Details of this approach is discussed below.

## How it works:
MCM acts as GRPC server and listens on a pre-defined port 5000. It implements below GRPC services. Details of each of these services are mentioned in next section.
* `Register()`
* `GetMachineClass()`
* `GetSecret()`

## GRPC services exposed by MCM:

### Register()
`rpc Register(stream DriverSide) returns (stream MCMside) {}`

The CP GRPC client calls this service to register itself with the MCM. The CP passes the `kind` and the `APIVersion` which it implements, and MCM maintains an internal map for all the registered clients. A GRPC stream is returned in response which is kept open througout the life of both the processes. MCM uses this stream to communicate with the client  for machine operations: `Create()`, `Delete()` or `List()`.
The CP client is responsible for reading the incoming messages continuously, and based on the `operationType` parameter embedded in the message, it is supposed to take the required action. This part is already handled in the package `grpc/infraclient`.
To add a new CP client, import the package, and implement the `ExternalDriverProvider` interface:

```
type ExternalDriverProvider interface {
	Create(machineclass *MachineClassMeta, credentials, machineID, machineName string) (string, string, error)
	Delete(machineclass *MachineClassMeta, credentials, machineID string) error
	List(machineclass *MachineClassMeta, credentials, machineID string) (map[string]string, error)
}
```

### GetMachineClass()
`rpc GetMachineClass(MachineClassMeta) returns (MachineClass) {}`

As part of the message from MCM for various machine operations, the name of the machine class is sent instead of the full machine class spec. The CP client is expected to use this GRPC service to get the full spec of the machine class. This optionally enables the client to cache the machine class spec, and make the call only if the machine calass spec is not already cached.

### GetSecret()
`rpc GetSecret(SecretMeta) returns (Secret) {}`

As part of the message from MCM for various machine operations, the Cloud Config (CC) and CP credentials are not sent. The CP client is expected to use this GRPC service to get the secret which has CC and CP's credentials from MCM. This enables the client to cache the CC and credentials, and to make the call only if the data is not already cached.

## How to add a new Cloud Provider's support
Import the package `grpc/infraclient` and `grpc/infrapb` from MCM (currently in MCM's "grpc-driver" branch)
* Implement the interface `ExternalDriverProvider`
    * `Create()`: Creates a new machine
    * `Delete()`: Deletes a machine
    * `List()`: Lists machines
* Use the interface `MachineClassDataProvider`
    * `GetMachineClass()`: Makes the call to MCM to get machine class spec
    * `GetSecret()`: Makes the call to MCM to get secret containing Cloud Config and CP's credentials

### Example implementation:

Refer GRPC based implementation for AWS client:
https://github.com/ggaurav10/aws-driver-grpc
