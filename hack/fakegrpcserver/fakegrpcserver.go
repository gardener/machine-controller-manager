package main

import (
	"log"
	"path"
	"time"

	svr "github.com/gardener/machine-controller-manager/pkg/grpc/infraserver"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/testdata"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	tls      = false //"Connection uses TLS if true, else plain TCP"
	certFile = ""    //"The TLS cert file"
	keyFile  = ""    //"The TLS key file"
	port     = 10000 //"The server port"
)

func main() {
	server := &svr.ExternalDriverManager{
		Port:   port,
		UseTLS: tls,
	}
	if tls {
		if certFile == "" {
			certFile = testdata.Path("server1.pem")
		}
		if keyFile == "" {
			keyFile = testdata.Path("server1.key")
		}
		creds, err := credentials.NewServerTLSFromFile(certFile, keyFile)
		if err != nil {
			log.Fatalf("Failed to generate credentials %v", err)
		}
		server.Options = []grpc.ServerOption{grpc.Creds(creds)}
	}

	server.Start()

	// Test go routine to test end to end flow
	go func() {
		for {
			driver, _ := server.GetDriver(metav1.TypeMeta{
				Kind:       "driver",
				APIVersion: path.Join("poc", "alpha"),
			})

			time.Sleep(5 * time.Second)
			log.Printf("Calling create")
			driver.Create("fakeDriver", "a", "b")

			time.Sleep(5 * time.Second)
			log.Printf("Calling delete")
			driver.Delete("fakeDriver", "a", "b")
		}
	}()
}
