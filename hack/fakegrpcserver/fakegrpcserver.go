package main

import (
	svr "github.com/gardener/machine-controller-manager/pkg/grpc/infraserver"
)

func main() {
	svr.StartServer()
}
