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
    git commit -m "Renamed SampleProvider"
    git push origin master
    ```

## Code changes required

1. Update the `pkg/{provider-name}/apis/provider-spec.go` specification file to reflect the structure of the objects exchanged between the machine-controller-manager (AKA cmi-client) and the driver (AKA cmi-server). It typically contains the machine details and secrets (if required). Follow the sample spec provided already in the file. A sample provider specification can be found [here](https://github.com/prashanth26/machine-controller-manager-provider-gcp/blob/master/pkg/gcp/apis/provider-spec.go).
1. Fill in the methods described at `pkg/{provider-name}/machine-server.go` to manage VMs on your cloud provider.
    - Fill in the required methods `CreateMachine()`, `GetMachine()`, `DeleteMachine()`, `GetListOfVolumeIDsForExistingPVs()` and `ListMachines()` methods.
        - `GetListOfVolumeIDsForExistingPVs()` expects VolumeIDs to be decoded from the volumeSpec based on the cloud provider.
        - The request and response parameters for each of the methods to be implemented are well documented as comments and sample codes at `pkg/{provider-name}/machine-server.go`.
    - Optional methods like `ShutDownMachine()` are optional as implicit, however we strongly recommend you to implement them as well.
    - A sample provider implementation for these methods can be found [here](https://github.com/gardener/machine-controller-manager-provider-aws/blob/master/pkg/aws/machineserver.go).
1. Perform validation of APIs that you have described and make it a part of your methods as required.
1. Write unit tests to make it work with your implementation by running `make test`.
1. Re-generate the vendors, to update any new vendors imported.
    ```bash
    make revendor
    ```
1. Update the sample YAML files on `kubernetes/` directory to provide sample files through which the working of the cmi-server can be tested.
1. Update `README.md` to reflect any additional changes

## Testing your code changes

Make sure `$KUBECONFIG` points to the cluster where you wish to manage machines. `$NAMESPACE` represents the namespaces where MCM is looking for machine objects.

1. On the first terminal,
    - Run the cmi-server (driver) using the command below.
        ```bash
        go run app/controller/cmi-server.go --endpoint=tcp://127.0.0.1:8080
        ```
1. On the second terminal,
    - Clone the [latest MCM code](https://github.com/gardener/machine-controller-manager/tree/cmi-client) and switch to the `cmi-client` branch.
    - Deploy the required CRDs from the machine-controller-manager repo,
        ```bash
        kubectl apply -f kubernetes/crds.yaml
        ```
    - Run the machine-controller-manager in the `cmi-client` branch
        ```bash
        go run cmd/machine-controller-manager/controller_manager.go --control-kubeconfig=$KUBECONFIG --target-kubeconfig=$KUBECONFIG --namespace=$NAMESPACE
        ```
1. On the third terminal,
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

## Releasing your docker image

1. Make sure you have logged into gcloud/docker using the CLI.
1. To release your docker image, run the following.
```bash
    make release IMAGE_REPOSITORY=<link-to-image-repo>
```
1. A sample kubernetes deploy file can be found at `kubernetes/deployment.yaml`. Update the same (with your desired cmi-server and cmi-client images) to deploy your MCM.
