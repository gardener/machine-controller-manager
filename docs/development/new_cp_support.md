# Adding support for a new provider

Steps to be followed while implementing a new (hyperscale) provider are mentioned below,

## Setting up your repository

1. Create a new empty repository named `machine-controller-manager-provider-{provider-name}` on github username/project. Do not initialize this repository with a README.
1. Copy the remote repository `URL` (HTTPS/SSH) to this repository which is displayed once you create this repository.
1. Now on your local system, create directories as required. {your-github-username} given below could also be {github-project} depending on where you have created the new repository.
    ```bash
    mkdir -p $GOPATH/src/github.com/{your-github-username}
    ```
1. Navigate to this created directory.
    ```bash
    cd $GOPATH/src/github.com/{your-github-username}
    ```
1. Clone [this repository](https://github.com/gardener/machine-controller-manager-provider-sampleprovider) on your local machine.
    ```bash
    git clone git@github.com:gardener/machine-controller-manager-provider-sampleprovider.git
    ```
1. Rename the directory from `machine-controller-manager-provider-sampleprovider` to `machine-controller-manager-provider-{provider-name}`.
    ```bash
    mv machine-controller-manager-provider-sampleprovider machine-controller-manager-provider-{provider-name}
    ```
1. Navigate into the newly created directory.
    ```bash
    cd machine-controller-manager-provider-{provider-name}
    ```
1. Update the remote `origin` URL to the newly created repository's URL you had copied above.
    ```bash
    git remote set-url origin git@github.com:{your-github-username}/machine-controller-manager-provider-{provider-name}.git
    ```
1. Rename github project from `gardener` to `{github-org/your-github-username}` where ever you have cloned the repository above. Use the hack script given below to do the same.
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
    git commit -m "Renamed SampleProvide to {provide-name}r"
    git push origin master
    ```

## Code changes required

The contract between he machine-controller-manager (AKA cmi-client) and the plugin/driver (AKA cmi-plugin) has been [documented here](https://github.com/gardener/machine-spec/blob/master/spec.md) and the [gRPC proto file can be found here](https://github.com/gardener/machine-spec/blob/master/cmi.proto). You may refer to them for any queries.

:warning:
- Keep in mind that, **there should to be a unique way to map between machine objects and VMs**. This can be done by mapping machine object names with VM-Name/ tags/ other metadata.
- Optionally there should also be a unique way to map a VM to it's machine class object. This can be done by tagging VM objects with tags/resource-groups associated with the machine class.

#### Steps to integrate

1. Update the `pkg/{provider-name}/apis/provider_spec.go` specification file to reflect the structure of the objects exchanged between the cmi-client and cmi-plugin. It typically contains the machine details and secrets (if required). Follow the sample spec provided already in the file. A sample provider specification can be found [here](https://github.com/gardener/machine-controller-manager-provider-aws/blob/master/pkg/aws/apis/aws_provider_spec.go).
1. Fill in the methods described at `pkg/{provider-name}/machine_server.go` to manage VMs on your cloud provider. Comments are provided above each method to help you fill them up with desired `REQUEST` and `RESPONSE` parameters.
    - A sample provider implementation for these methods can be found [here](https://github.com/gardener/machine-controller-manager-provider-aws/blob/master/pkg/aws/machine_server.go).
    - Fill in the required methods `CreateMachine()`, and `DeleteMachine()` methods.
    - Optionally fill in methods like `GetMachineStatus()`, `ListMachines()`, `GetVolumeIDs()` and `ShutDownMachine()`. You may choose to fill these, once the working of the required methods seem to be working.
        - `GetVolumeIDs()` expects VolumeIDs to be decoded from the volumeSpec based on the cloud provider.
1. Update methods given at `pkg/{provider-name}/identity_server.go` if required. This is specially true if you plugin only plans to support a subset of desired RPCs. Update the supported capabilities for Machine Service [here](https://github.com/gardener/machine-controller-manager-provider-sampleprovider/blob/master/pkg/sampleprovider/identity_server.go#L63-L68). All capability support is enabled by default.
1. Perform validation of APIs that you have described and make it a part of your methods as required.
1. Write unit tests to make it work with your implementation by running `make test`.
    ```bash
    make test
    ```
1. Re-generate the vendors, to update any new vendors imported.
    ```bash
    make revendor
    ```
1. Update the sample YAML files on `kubernetes/` directory to provide sample files through which the working of the cmi-plugin can be tested.
1. Update `README.md` to reflect any additional changes

## Testing your code changes

Make sure `$KUBECONFIG` points to the cluster where you wish to manage machines. `$NAMESPACE` represents the namespaces where MCM is looking for machine objects.

1. On the first terminal running at `$GOPATH/src/github.com/{github-org/your-github-username}/machine-controller-manager-provider-{provider-name}`,
    - Run the cmi-plugin (driver) using the command below.
        ```bash
        go run app/controller/cmi-plugin.go --endpoint=tcp://127.0.0.1:8080
        ```
1. On the second terminal pointing to `$GOPATH/src/github.com/gardener`,
    - Clone the [latest MCM code](https://github.com/gardener/machine-controller-manager/tree/cmi-client)
        ```bash
        git clone git@github.com:gardener/machine-controller-manager.git
        ```
    - Navigate to the newly created directory.
        ```bash
        cd machine-controller-manager
        ```
    - Switch to the `cmi-client` branch.
        ```bash
        git checkout cmi-client
        ```
    - Deploy the required CRDs from the machine-controller-manager repo,
        ```bash
        kubectl apply -f kubernetes/crds.yaml
        ```
    - Run the machine-controller-manager in the `cmi-client` branch
        ```bash
        go run cmd/machine-controller-manager/controller_manager.go --control-kubeconfig=$KUBECONFIG --target-kubeconfig=$KUBECONFIG --namespace=$NAMESPACE --v=3
        ```
1. On the third terminal pointing to `$GOPATH/src/github.com/gardener/machine-controller-manager`
    - Fill in the object files given below and deploy them as described below.
    - Deploy the `machine-class`
        ```bash
        kubectl apply -f kubernetes/machine-class.yaml
        ```
    - Deploy the `kubernetes secret` if required.
        ```bash
        kubectl apply -f kubernetes/secret.yaml
        ```
    - Deploy the `machine` object and make sure it joins the cluster successfully.
        ```bash
        kubectl apply -f kubernetes/machine.yaml
        ```
    - Once machine joins, you can test by deploying a machine-deployment.
    - Deploy the `machine-deployment` object and make sure it joins the cluster successfully.
        ```bash
        kubectl apply -f kubernetes/machine-deployment.yaml
        ```
    - Make sure to delete both the `machine` and `machine-deployment` object after use.
        ```bash
        kubectl delete -f kubernetes/machine.yaml
        kubectl delete -f kubernetes/machine-deployment.yaml
        ```

## Releasing your docker image

1. Make sure you have logged into gcloud/docker using the CLI.
2. To release your docker image, run the following.
```bash
    make release IMAGE_REPOSITORY=<link-to-image-repo>
```
3. A sample kubernetes deploy file can be found at `$GOPATH/src/github.com/{github-org/your-github-username}/machine-controller-manager-provider-{provider-name}/kubernetes/deployment.yaml`. Update the same (with your desired cmi-plugin and cmi-client images) to deploy your MCM.
