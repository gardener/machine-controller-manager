## Dependency management

We use [Dep](https://github.com/golang/dep) to manage golang dependencies.. In order to add a new package dependency to the project, you can perform `dep ensure -add <PACKAGE>` or edit the `Gopkg.toml` file and append the package along with the version you want to use as a new `[[constraint]]`.

### Updating dependencies

The `Makefile` contains a rule called `revendor` which performs a `dep ensure -update`. This updates all the dependencies to its latest versions (respecting the constraints specified in the `Gopkg.toml` file). The command also installs the packages which do not yet exist in the `vendor` folder but are specified in the `Gopkg.toml` (in case you have added new ones).

```bash
$ make revendor
```

The dependencies are installed into the `vendor` folder which **should be added** to the VCS.

:warning: Make sure you test the code after you have updated the dependencies!
