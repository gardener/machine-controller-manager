package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"log"

	"github.com/gardener/node-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/gardener/node-controller-manager/pkg/driver"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
)

var (
	secret_filename       string
	machineclass_filename string
	class_kind            string
	machine_name          string
	machine_id            string
)

// func NewDriver(machineId string, secretRef *corev1.Secret, classKind string, machineClass interface{}, machineName string) Driver {

// AddFlags adds flags for a specific CMServer to the specified FlagSet
func CreateFlags() {

	flag.StringVar(&secret_filename, "secret", "", "infrastructure secret")
	flag.StringVar(&machineclass_filename, "machineclass", "", "infrastructure machineclass")
	flag.StringVar(&class_kind, "classkind", "", "infrastructure class kind")
	flag.StringVar(&machine_name, "machinename", "", "machine name")
	flag.StringVar(&machine_id, "machineid", "", "machine id")

	flag.Parse()
}

func main() {

	CreateFlags()

	var (
		machineclass interface{}
	)

	if machine_name == "" {
		log.Fatalf("machine name required")
	}

	if machineclass_filename == "" {
		log.Fatalf("machine class filename required")
	}

	if secret_filename == "" {
		log.Fatalf("secret filename required")
	}
	secret := corev1.Secret{}
	err := Read(secret_filename, &secret)
	if err != nil {
		log.Fatalf("Could not parse secret yaml: %s", err)
	}

	switch class_kind {
	case "OpenStackMachineClass", "openstack":
		class := v1alpha1.OpenStackMachineClass{}
		machineclass = &class
		class_kind = "OpenStackMachineClass"

	case "AWSMachineClass", "aws":
		class := v1alpha1.AWSMachineClass{}
		machineclass = &class
		class_kind = "AWSMachineClass"

	case "AzureMachineClass", "azure":
		class := v1alpha1.AzureMachineClass{}
		machineclass = &class
		class_kind = "AzureMachineClass"

	case "GCPMachineClass", "gcp":
		class := v1alpha1.GCPMachineClass{}
		machineclass = &class
		class_kind = "GCPMachineClass"

	default:
		log.Fatalf("Unknown class kind %s", class_kind)
	}
	err = Read(machineclass_filename, machineclass)
	if err != nil {
		log.Fatalf("Could not parse machine class yaml: %s", err)
	}

	driver := driver.NewDriver(machine_id, &secret, class_kind, machineclass, machine_name)

	if machine_id == "" {
		id, name, err := driver.Create()
		if err != nil {
			log.Fatalf("Could not create %s : %s", machine_name, err)
		}
		fmt.Printf("Machine id: %s", id)
		fmt.Printf("Name: %s", name)
	} else {
		err = driver.Delete()
		if err != nil {
			log.Fatalf("Could not delete %s : %s", machine_id, err)
		}
	}

}

func Read(fileName string, decodedObj interface{}) error {
	m, err := ioutil.ReadFile(fileName)
	if err != nil {
		log.Fatalf("Could not read %s: %s", fileName, err)
	}
	decoder := yaml.NewYAMLOrJSONDecoder(bytes.NewReader(m), 1024)

	return decoder.Decode(decodedObj)
}
