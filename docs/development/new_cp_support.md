# Adding support for a new provider

Steps to be followed while implementing a new (hyperscale) provider are mentioned below,

### Setting up your repository

1. Fork [this repository](https://github.com/prashanth26/machine-controller-manager-provider-foobar) on your github.
1. Create directories as required and nagivate into this path. {your-github-username} could also be {github-project}.
    ```bash
    mkdir -p $GOPATH/src/github.com/{your-github-username}
    cd $GOPATH/src/github.com/{your-github-username}
    ```
1. Rename the repository from `machine-controller-manager-provider-foobar` to `machine-controller-manager-provider-{provider-name}`
1. Clone this newly forked repository.
    ```bash
    git clone https://github.com/{your-github-username}/machine-controller-manager-provider-{provider-name}
    ```
1. Navigate into the newly created repository.
    ```bash
    cd machine-controller-manager-provider-{provider-name}
    ```
1. Rename all files and code from `foobar` to your desired `{provider-name}`. Use the hack script given below to do the same. Provider name is case sensitive.
    ```bash
    make rename-provider PROVIDER_NAME={provider-name}
    eg:
        make rename-provider PROVIDER_NAME=AmazonWebServices (or)
        make rename-provider PROVIDER_NAME=AWS
    ```
1. Now commit your changes and push it upstream.
    ```bash
    git commit -a
    git push origin master
    ```

### Code changes required

1. TODO

