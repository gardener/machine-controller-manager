## Dependency management

We use golang modules to manage golang dependencies. In order to add a new package dependency to the project, you can perform `go get <PACKAGE>@<VERSION>` or edit the `go.mod` file and append the package along with the version you want to use.

### Updating dependencies

The `Makefile` contains a rule called `tidy` which performs `go mod tidy`.

`go mod tidy` makes sure go.mod matches the source code in the module. It adds any missing modules necessary to build the current module's packages and dependencies, and it removes unused modules that don't provide any relevant packages.

```bash
$ make tidy
```

The dependencies are installed into the go mod cache folder.

:warning: Make sure you test the code after you have updated the dependencies!
