package controller

import (
	"fmt"
	"net/url"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"

	"github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
)

const machineNamePlaceholder = "<<MACHINE_NAME>>"

var urlEncodedMachineNamePlaceholder = url.QueryEscape(machineNamePlaceholder)

func (c *controller) addMachineNameToUserData(machine *v1alpha1.Machine, secret *corev1.Secret) error {
	var (
		userDataB []byte
		userDataS string
		exists    bool
	)

	if userDataB, exists = secret.Data["userData"]; !exists {
		// If userData key is not founds
		return fmt.Errorf("userdata field not found in secret for machine %q", machine.Name)
	}
	userDataS = string(userDataB)

	if strings.Contains(userDataS, machineNamePlaceholder) {
		klog.V(4).Infof("replacing placeholder %s with %s in user-data!", machineNamePlaceholder, machine.Name)
		userDataS = strings.ReplaceAll(userDataS, machineNamePlaceholder, machine.Name)
	} else if strings.Contains(userDataS, urlEncodedMachineNamePlaceholder) {
		klog.V(4).Infof("replacing url encoded placeholder %s with %s in user-data!", urlEncodedMachineNamePlaceholder, url.QueryEscape(machine.Name))
		userDataS = strings.ReplaceAll(userDataS, urlEncodedMachineNamePlaceholder, url.QueryEscape(machine.Name))
	} else {
		klog.Info("no machine name placeholder found in user-data, nothing to replace! Without machine name , node won't join if node-agent-authorizer is enabled.")
	}

	secret.Data["userData"] = []byte(userDataS)

	return nil
}
