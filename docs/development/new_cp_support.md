# Adding support for a new provider

Steps to be followed while implementing a new (hyperscale) provider are mentioned below,

### Setting up your repository

1. Fork [this repository](https://github.com/gardener/machine-controller-manager-provider-sampleprovider) on your github.
1. Rename the repository from `machine-controller-manager-provider-sampleprovider` to `machine-controller-manager-provider-{provider-name}` by updating the same in your github repository settings.
1. Create directories as required and nagivate into this path. {your-github-username} could also be {github-project}.
    ```bash
    mkdir -p $GOPATH/src/github.com/{your-github-username}
    cd $GOPATH/src/github.com/{your-github-username}
    ```
1. Clone this newly forked repository.
    ```bash
    git clone https://github.com/{your-github-username}/machine-controller-manager-provider-{provider-name}
    ```
1. Navigate into the newly created repository.
    ```bash
    cd machine-controller-manager-provider-{provider-name}
    ```
1. Rename github project from `gardener` to `{github-org}` or `{your-github-username}` where ever you have forked the repository above. Use the hack script given below to do the same.
    ```bash
    make rename-project PROJECT_NAME={github-org/your-github-username}
    eg:
        make rename-project PROJECT_NAME=gardener (or)
        make rename-project PROJECT_NAME=githubusername
    ```
1. Rename all files and code from `SampleProvider` to your desired `{provider-name}`. Use the hack script given below to do the same. {provider-name} is case sensitive.
    ```bash
    make rename-provider PROVIDER_NAME={provider-name}
    eg:
        make rename-provider PROVIDER_NAME=AmazonWebServices (or)
        make rename-provider PROVIDER_NAME=AWS
    ```
1. Now commit your changes and push it upstream.
    ```bash
    git add -A
    git commit
    git push origin master
    ```

### Code changes required

1. Update the `pkg/{provider-name}/apis/provider-spec.go` specification file to reflect the structure of the objects exchanged between the machine-controller-manager and the driver. It typically contains the machine details and secrets (if required). Follow the sample spec provided already in the file. A sample provider specification can be found [here](https://github.com/prashanth26/machine-controller-manager-provider-gcp/blob/master/pkg/gcp/apis/provider-spec.go). 
    ```bash
    vim pkg/{provider-name}/apis/provider-spec.go
    ```
1. Fill in the create/list/delete methods `pkg/{provider-name}/machineserver.go` to create/list/delete VMs on your cloud provider. A sample provider implementation for these methods can be found [here](https://github.com/prashanth26/machine-controller-manager-provider-gcp/blob/master/pkg/gcp/machineserver.go).
    ```bash
    vim pkg/{provider-name}/machineserver.go
    ```
1. Re-generate the vendors, to update any new vendors imported.
    ```bash
    make revendor
    ```

### Testing your code changes

Make sure `$KUBECONFIG` points to the cluster where you wish to manage machines. `$NAMESPACE` represents the namespaces where MCM is looking for machine objects.

1. Run the provider driver
```bash
   go run app/controller/main.go --endpoint=tcp://127.0.0.1:8080
```
1. Run the machine-controller-manager in the `cmi-client` branch
```bash
   go run cmd/machine-controller-manager/controller_manager.go --control-kubeconfig=$KUBECONFIG --target-kubeconfig=$KUBECONFIG --namespace=$NAMESPACE
```
1. Create machineClass, secrets, machines and machinedeployment
