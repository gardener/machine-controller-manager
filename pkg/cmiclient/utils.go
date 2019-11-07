package cmiclient

import (
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	cmipb "github.com/gardener/machine-spec/lib/go/cmi"
	corev1 "k8s.io/api/core/v1"

	"github.com/golang/glog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func newMachineClient(driverName string) (machineClient cmipb.MachineClient, closer io.Closer, err error) {
	var conn *grpc.ClientConn
	conn, err = newGrpcConn(driverName)
	if err != nil {
		glog.Error(err)
		return nil, nil, err
	}

	machineClient = cmipb.NewMachineClient(conn)
	return machineClient, conn, nil
}

func newIdentityClient(driverName string) (identityClient cmipb.IdentityClient, closer io.Closer, err error) {
	var conn *grpc.ClientConn
	conn, err = newGrpcConn(driverName)
	if err != nil {
		glog.Error(err)
		return nil, nil, err
	}

	identityClient = cmipb.NewIdentityClient(conn)
	return identityClient, conn, nil
}

func getSecretData(secret *corev1.Secret) map[string][]byte {
	var (
		secretData map[string][]byte
	)

	if secret != nil {
		secretData = secret.Data
	}

	return secretData
}

func newGrpcConn(driverName string) (*grpc.ClientConn, error) {

	var name, addr string
	driverInfo := strings.Split(driverName, "//")
	if driverName != "" && len(driverInfo) == 2 {
		name = driverInfo[0]
		addr = driverInfo[1]
	} else {
		name = "grpc-default-driver"
		addr = "127.0.0.1:8080"
	}

	network := "tcp"

	glog.V(4).Infof("Creating new gRPC connection for [%s://%s] for driver: %s", network, addr, name)

	return grpc.Dial(
		addr,
		grpc.WithInsecure(),
		grpc.WithDialer(func(target string, timeout time.Duration) (net.Conn, error) {
			return net.Dial(network, target)
		}),
	)
}

// validatePluginRequest checks if plugin supports the request
func (c *CMIDriverClient) validatePluginRequest(capability cmipb.PluginCapability_RPC_Type) error {
	if capability == cmipb.PluginCapability_RPC_UNKNOWN {
		return nil
	}

	for _, cap := range c.Capabilities {
		if capability == cap.GetRpc().GetType() {
			return nil
		}
	}
	return status.Error(codes.Unimplemented, fmt.Sprintf("%s", capability))
}
