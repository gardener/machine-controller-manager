## Clustersim

It is a tool that can be used for manual MCM testing, allowing a developer to create an environment where they can deploy custom `MachineClass`es and `MachineDeployment`s in a virtual environment running both the MCM and the simulated provider processes.

The command line program contains a bunch of helper sub-commands allowing one to set-up the cluster in numerous ways before they actually run any processes.

### `help`

```
./bin/clustersim -h
Manages and sets up virtual cluster running simulated Machine Controller Manager.

Usage:
clustersim {--clusters name}... command [--dir /tmp]
clustersim [command]

Examples:
clustersim --clusters "test" setup

Available Commands:
build       Builds the specified components (machine-controller-manager, simulated provider).
completion  Generate the autocompletion script for the specified shell
copyshoot   Fetches MachineDeployments and MachineClasses from specified shoot.
gencluster  Generates MachineClasses and MachineDeployments with the specified parameters.
help        Help about any command
setup       Generates the launch configuration for the cluster components.
start       Creates kwok cluster and starts specified components using their launch config.
stop        Stops specified components using their pid and destroys the kwok cluster.

Flags:
--clusters strings   comma separated list of cluster(s) to target (required)
--dir string         optional flag to specify the directory for clusters data (default "./gen")
-h, --help               help for clustersim
```

The main command takes two flags, one optional and one required.
- The flag to specify clusters' data directory is optional, and if unspecified falls back to `./gen` directory.
- The flag to list out the cluster names is required by all subcommands except the `build` command.

### `build`

Builds the specified components, fallsback to building both `machine-controller-manager` and the `simulated-provider` if unspecifed. Requires a mandatory flag `source-dir` pointing to the `machine-controller-manager` source root directory.

```
./bin/clustersim build -h
Builds the specified components (machine-controller-manager, simulated provider).

Usage:
clustersim build {--source-dir ..} [--components mcm]...

Examples:
clustersim build --source-dir ../../../../ --components "mcm,mc"

Flags:
-c, --components strings   comma separated list of components to build (default [mcm,mc])
-h, --help                 help for build
--source-dir string    machine-controller-manager source root directory path (required)
```

### `setup`

Generates the launch configuration for the cluster components. **Requires** the `mcc.yaml` and the `mcd.yaml` files to already be present in the specified clusters' directories. These can be manually fetched by targeting a cluster or be constructed via the `gencluster` subcommand.

The launch configuration consists of the path for the components binaries and their corresponding launch flags.

### `copyshoot`

Rather than manually fetching the data for a cluster for `setup`, `clustersim` also provides a gardener shoot specific `copyshoot` subcommand which can fetch the `MachineClass`es and `MachineDeployment`s of a live cluster in an automated fashion provided the user has the required permission to access the specified shoot's data.

It can also fetch the launch flags for the components from the actual cluster as well, and sanitizes the flags for the virtual cluster's usage.

```
./bin/clustersim copyshoot -h
Fetches MachineDeployments and MachineClasses from specified shoot.

Usage:
clustersim copyshoot {--clusters name} {-l landscape} {-p project} {-s shoot} [-f true|false]

Flags:
-f, --fetch-config       fetch component configuration from cluster (default true)
-h, --help               help for copyshoot
-l, --landscape string   gardener landscape name (required)
-p, --project string     gardener project name (required)
-s, --shoot string       gardener shoot name (required)
```

### `gencluster`

In case a developer needs to quickly generate some dummy data for testing, a `gencluster` helper command is provided, which takes specified `zones` and `instances` and constructs MCCs and MCDs for their cross-product. Additionally it generates the required launch configuration as well.

```
./bin/clustersim gencluster -h
Generates MachineClasses and MachineDeployments with the specified parameters.

Usage:
clustersim gencluster {--clusters name} {--zones zone}... {--instances inst}...

Flags:
-h, --help                help for gencluster
-i, --instances strings   comma separated list of instances to use (required)
-z, --zones strings       comma separated list of zones to use (required)
```

It utilizes the templates present in [./templates](./templates) in order to construct the required mcc and mcd. And uses the data present in [./templates/instances.json](./templates/instances.json) to get the required `arch` and the `capacity` for the specified instances.

### `start`

This runs the actual virtual cluster and deploys the MCC and MCD belonging to the clusters, it also creates two files per component that it runs (mcm, machine-controller):
- `<process>.pid`: denoting the PID for the component, used by the `stop` command to kill the process for the specified cluster.
- `<process>.log`: consisting of the components' log file.

### `stop`

Kills the specified clusters' specified components and destroys the virtual cluster.

```
./bin/clustersim stop -h
Stops specified components using their pid and destroys the kwok cluster.

Usage:
clustersim stop {--clusters name}... [--components mcm]... [--killall true|false]

Examples:
clustersim --clusters "test" stop

Flags:
-c, --components strings   comma separated list of components to stop (default [mcm,mc])
-h, --help                 help for stop
--killall              stop all components and destroy all clusters (default true)
```
