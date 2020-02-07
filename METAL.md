# Metal usage


## CLI

build the cli with:

```bash
go build -o managevm cmd/machine-controller-manager-cli/main.go
```

this creates a cli which can be used outside of k8s to start a machine

```bash
# managevm
```

in order to start a machine at metal execute:

```bash

./managevm -machinename a-machine -classkind MetalMachineClass -machineclass kubernetes/machine_classes/metal-machine-class.yaml -secret kubernetes/Secrets/metal-secret.yaml
```
