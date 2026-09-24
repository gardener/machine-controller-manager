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
- `Create`
- `NodeJoin`
- `Init`
- `Delete`
- `List`
- `MachineStatus`

### Simulation Configuration

One can enable the required modifications/injections via the simulator configuration. Any parameters (if required) for the modification can also be specified in the configuration.

```json
{
  "create": {
    "minDelay": "10s",
    "maxDelay": "30s",
    "percentageOfMachines": "10%"
  },
  "delete": {
    "rateLimitError": {
      "errorDuration": "2m"
    }
    "percentageOfMachines": "40%"
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
pkg/simulatedprovider:
  - Makefile
  cmd/machine-controller/
  - main.go 
  provider/
  - driver.go # Driver Interface Implementation
  cluster/
  - cluster.go # Utility to setup and destroy clusters
  provider/simulation/
  - simulation.go
  - config.go
  provider/simulation/injections/
  - create.go
  - node_join.go
  - initialize.go
  - delete.go
  - list.go
  - machine_status.go
  test/integration/controller/
  - controller_suite_test.go
  - controller_test.go
```

#### High level Flow

1. Initialise control plane i.e. start `kube-apiserver`, `etcd` as local processes.
2. Deploy MCM CRDs.
3. Start `machine-controller-manager` and `machine-controller-manager-provider-simulator` processes.
4. Deploy user specified MCD and MCC (fallback to in-tree IT specific test MCD and MCC)
5. Create dummy secrets for the `credentialsSecretRef` and `secretRef` for the MCC.
6. Run IT. (Set some test related environment variables to true)

### `clustersim` tool for standalone testing

While the `provider-simulation` and the virtual cluster setup is enough for IT support, for manual testing purposes, it would be beneficial to have a standalone tool that can run both MCM and the provider standalone alongwith the control plane. This `clustersim` would aid in manual testing by allowing the user to set up their environment in different ways:

1. Manual fetching of the required MCD and MCC, then creating a configuration for the required virtual cluster. This would be achieved by something like this:
   - Target the control plane of the cluster whose data you wish to fetch
   - Store the MCCs and MCDs of the cluster:
     ```
     kubectl get mcc -oyaml > mcc.yaml
     kubectl get mcd -oyaml > mcd.yaml
     ```
   - Run `clustersim setup <demo-cluster> --mcc=./mcc.yaml --mcd=./mcd.yaml` with the fetched data; this would create a directory `./gen/<demo-cluster>/` containing the following files
     ```
     ./gen/<demo-cluster>
     + start-config-mcm.json # Contains the launch flags for mcm
     + start-config-mc.json # Contains the launch flags for mc-provider-simulation
     + mcc.yaml
     + mcd.yaml
     ```
   - Additionally the `setup` would also build the required binaries for `mcm` and `mc-provider-simulation` (in `gen/bin` directories, using their corresponding `make build` targets) which can later be used when running the testing environment.
     
2. Alternatively, to make the developer experience easier, in future the `clustersim` command could have a subcommand to accept cluster info; which can be used to automatically fetch the required MCC and MCD and then setup the cluster directory with the files as explained above. Usage for gardener clusters could look like: `clustersim copyshoot <demo-cluster> --landscape --project --shoot`.

3. The third way to set-up a testing environment could be via the usage of synthetic generated data for the MCC and MCD. In this case, the user can specify what kind of instances and zones would they want the generated data to be for. An example usage would be something like: `clustersim gendata <demo-cluster> --instances "m5.large,m5.xlarge" --zones "eu-west-1a,eu-central-2b"`. This would generate MCCs containing the specified parameters and the corresponding MCDs. Then the cluster directory would be created as elucidated above.

The above would be three different ways in which a developer can set up their testing environment, based on the amount of control they'd want over the scenario. After this is done, the `clustersim` tool should also provide a way for the user to start the virtual cluster with the specified components that they wish to run. 

This would be achieved via a `start` subcommand for `clustersim`:
```
clustersim start <demo-cluster> --components 'mcm,mc' --dir 'gen/<demo-cluster>'
```

This would first validate and check that all the required data for the demo-cluster is present and then construct a virtual control-plane cluster running `kube-apiserver`, `etcd` and `kube-scheduler`. Additionally it would also run the specified components `mcm` and `mc` with their launch parameters part of their respective `start-config-*.json` files.

Furthermore, when launching these processes, it would also copy the running cluster's `kubeconfig` in the generated cluster directory for ease in debugging, and save the process PIDs as well for status tracking and cleanup purposes. The logs for the services would also be stored in the cluster directory.

Finally, a `stop` subcommand can be used to either destory the entire cluster and kill all the running processes (exporting their logs) or kill only the specified processes. This would use the PIDs stored above as part of the `start` subcommand.

## Target Scope

Only offer functionality needed to run IT standalone (as part of CI/CD pipelines), with optional support for adding points of failures. The simulation config skaffold and supported modifications doesn't have to be exhaustive in the first iteration, however it should be extensible enough to allow someone to add their customizations to the simulation provider at their specified modification point to test edge cases (restricted still to modification related to driver interface calls).

## Future Scope

- Add support for optionally building and running CA for manually testing. This would require passing path to the local checked in tree of autoscaler to keep dependencies to a minimum.
- If `minkapi` is extracted into a standalone project, the control plane setup (i.e. `kube-apiserver` + `etcd`) can be replaced by it for a leaner implementation allowing for stress testing with an even larger number of machines.
- Since the integration tests don't require any workload to be deployed, the control plane deployed components doesn't include `kube-scheduler`. If required for manual testing, this can be easily achieved by modifying the `e2e-framework` setup to include the scheduler as well.
- The support for `clustersim` subcommands `copyshoot` and `gendata` would be part of future scope of this proposal, and not tackled in the initial stages of implementation.
- Tests which are added as part of scalability/stress testing should be explicitly marked in the framework; to ensure that these tests are not run as part of each PR testing. This is required in order to prevent flaky/long tests from hindering development while still being useful as a sanity/performance check before releases to ensure no regressions between different minor versions.
