# Simulation Provider for Testing

## Objective

Currently, all Machine Controller Manager (MCM) integration tests (IT) run only with an actual cloud provider, making it difficult to validate changes or perform stress tests with a large number of machines. Moreover, the tests don't run as part of the repository's CI/CD workflows since that requires a running kubernetes cluster with actual backing infrastructure. This leads to manual effort on either the developer or the reviewer to actually run the integration tests to ensure that there aren't any regressions.

This document proposes enhancing MCM with the capability to:
- Run a simulated machine controller provider for integration testing purposes.
- Introduce modifications to the simulated machine's lifecycle in the form of delays, errors, etc.
- Specify custom `MachineDeployment`s (MCD) and `MachineClass`es (MCC) for the end user to perform manual testing.

## Proposal

In the MCM repository tree, introduce a new package (tentatively named) `provider-simulation` which implements the `Driver` interface required by each supported cloudprovider. Refer to [machine_error_codes.md#machine-provider-interface](../development/machine_error_codes.md#machine-provider-interface) for more details.

### Initialisation

Cluster creation and lifecycle to be managed via `e2e-framework`, it handles the initialisation and creation of the minimal control plane needed for running the simulation provider. This fetches the specified `kube-apiserver` and `etcd` binaries (if not present) and runs them as local processes.

### Simulation Driver

* `CreateMachine()`:
  Creates a `node` for the specified `CreateMachineRequest`'s `Machine` and `MachineClass` as a representation for the existence of a backing VM. Returns a dummy `ProviderID`, `NodeName` and and `LastKnownState` message denoting successful instance creation. This can be modified to simulate quota (`ResourceExhausted`) errors or VM creation/node join delays etc.
* `InitializeMachine()`:
  Optional method that is used for network configuration for the VM. It can be skipped for the simulation provider. However, it can be used to simulate initialization errors/delays.
* `DeleteMachine()`:
  Since the deletion of the actual `node` is handled in `triggerDeletionFlow()`, for mocking the VM deletion from the CSP side, all that's needed is to stop tracking the machine (using `ProviderID`) as part of the quota (if defined) for the instance to which the machine belongs.
* `GetMachineStatus()`:
  Depending on whether any failures are to be injected for the simulation, this can either return success (denoted by returning the `ProviderID` and `NodeName`) or else `NotFound` or similar errors. For multiple VMs being returned for the queried machine, `OutOfRange` error is returned.
* `ListMachines()`:
  Returns a `map[providerID]machineName` for the specified `MachineClass`.
* `GetVolumeIDs()`:
  Skipped for the simulation provider for now. Can be implemented in the future if the testing requires it.

### Simulation Modifications

Machine lifecycle hooks where modifications to the `Driver` method implementation can be done for failure simulation (similar to [scheduling framework](https://kubernetes.io/docs/concepts/scheduling-eviction/scheduling-framework/)). This allows for convenient targeted manual testing and customized testing by choosing which failures to simulate and for how many machines.

This would be achieved by having 'defined' injection points in the `Driver` method calls where the specified failures/modifications can be triggered:
- `PreCreate`
- `PreNodeJoin`
- `PostCreate`
- `Init`
- `PreDelete`
- `PostDelete`
- `PreList`
- `MachineStatus`

### Simulation Configuration

One can enable the required modifications/injections via the simulator configuration. Any parameters (if required) for the modification can also be specified in the configuration.

```json
{
  "preCreate": {
    "minDelay": "10s",
    "maxDelay": "30s",
    "percentageOfMachines": "10%"
  },
  "preDelete": {
    "rateLimitError": {
      "errorDuration": "2m"
    }
  },
  "instanceQuota": {
    "m5.xlarge": 20
  }
}
```

### Integration with MCM

The simulation provider would be added as a new package located at `pkg/simulator`.
Proposed file structure:
```
pkg/simulator:
  - main.go
  - Makefile
  provider/
  - driver.go
  provider/simulation/
  - simulation.go
  - config.go
  provider/simulation/injections/
  - pre_create.go
  - pre_node_join.go
  - post_create.go
  - initialize.go
  - pre_delete.go
  - post_delete.go
  - pre_list.go
  - machine_status.go
  test/integration/controller/
  - controller_suite_test.go
  - controller_test.go
  - resource_tracker.go
```

#### High level Flow

1. Initialise control plane i.e. start `kube-apiserver`, `etcd` as local processes.
2. Deploy MCM CRDs.
3. Start `machine-controller-manager` and `machine-controller-simulation-provider` processes.
4. Deploy user specified MCD and MCC (fallback to in-tree IT specific test MCD and MCC)
5. Create dummy GNA secret (GNA secret, need to check if only passing a dummy environment variable for `GNA_SECRET_NAME` is sufficient or the secret itself needs to be created)
6. Run IT. (Set some test related environment variables to true)

## Target Scope

Only offer functionality needed to run IT standalone (as part of CI/CD pipelines), with optional support for adding points of failures. The simulation config skaffold and supported modifications doesn't have to be exhaustive in the first iteration, however it should be extensible enough to allow someone to add their customizations to the simulation provider at their specified modification point to test edge cases (restricted still to modification related to driver interface calls).

## Future Scope

- Add support for optionally building and running CA for manually testing. This would require passing path to the local checked in tree of autoscaler to keep dependencies to a minimum.
- If `minkapi` is extracted into a standalone project, the control plane setup (i.e. `kube-apiserver` + `etcd`) can be replaced by it for a leaner implementation allowing for stress testing with an even larger number of machines.
- Since the integration tests don't require any workload to be deployed, the control plane deployed components doesn't include `kube-scheduler`. If required for manual testing, this can be easily achieved by modifying the `e2e-framework` setup to include the scheduler as well.
