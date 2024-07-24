package controller

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	bootstraptokenapi "k8s.io/cluster-bootstrap/token/api"
	bootstraptokenutil "k8s.io/cluster-bootstrap/token/util"

	"github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
)

var _ = Describe("userdata", func() {
	var (
		ctx               context.Context
		targetCoreClient  kubernetes.Interface
		machineController *controller

		machine         *v1alpha1.Machine
		tokenSecretName string
		userDataSecret  *corev1.Secret
	)

	const userDataTemplate = `"#!/bin/bash

mkdir -p "/var/lib/gardener-node-agent/credentials"

cat << EOF > "/var/lib/gardener-node-agent/credentials/bootstrap-token"
%s
	EOF
chmod "0640" "/var/lib/gardener-node-agent/credentials/bootstrap-token"
mkdir -p "/var/lib/gardener-node-agent"

mkdir -p "/var/lib/gardener-node-agent/credentials"

cat << EOF > "/var/lib/gardener-node-agent/credentials/machine-name"
%s
	EOF
chmod "0640" "/var/lib/gardener-node-agent/credentials/machine-name"
`

	getBootstrapToken := func() string {
		tokenSecret, err := targetCoreClient.CoreV1().Secrets(metav1.NamespaceSystem).Get(ctx, tokenSecretName, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		token := bootstraptokenutil.TokenFromIDAndSecret(
			string(tokenSecret.Data[bootstraptokenapi.BootstrapTokenIDKey]),
			string(tokenSecret.Data[bootstraptokenapi.BootstrapTokenSecretKey]),
		)
		return token
	}

	BeforeEach(func() {
		ctx = context.Background()
		targetCoreClient = fake.NewSimpleClientset()
		machineController = &controller{
			targetCoreClient: targetCoreClient,
		}

		machine = &v1alpha1.Machine{ObjectMeta: metav1.ObjectMeta{Name: "foo-machine"}}
		_, tokenSecretName = getTokenIDAndSecretName(machine.Name)
		userDataSecret = &corev1.Secret{
			Data: map[string][]byte{
				"userData": []byte(fmt.Sprintf(userDataTemplate, "<<BOOTSTRAP_TOKEN>>", "<<MACHINE_NAME>>")),
			},
		}
	})

	Describe("#addBootstrapTokenToUserData", func() {
		It("should generate a bootstrap token replace the magic string with it", func() {
			Expect(machineController.addBootstrapTokenToUserData(ctx, machine, userDataSecret)).To(Succeed())
			Expect(userDataSecret.Data["userData"]).To(Equal([]byte(fmt.Sprintf(userDataTemplate, getBootstrapToken(), "<<MACHINE_NAME>>"))))
		})

		It("should do nothing if there is no magic string", func() {
			userDataSecret.Data["userData"] = []byte("foobar")
			Expect(machineController.addBootstrapTokenToUserData(ctx, machine, userDataSecret)).To(Succeed())
			Expect(userDataSecret.Data["userData"]).To(Equal([]byte("foobar")))
		})

		It("should fail if the secret does not contain a userData key", func() {
			delete(userDataSecret.Data, "userData")
			Expect(machineController.addBootstrapTokenToUserData(ctx, machine, userDataSecret)).To(MatchError(ContainSubstring("userdata field not found in secret for machine")))
		})
	})

	Describe("#addMachineNameToUserData", func() {
		It("should replace the magic string with the machine name", func() {
			Expect(machineController.addMachineNameToUserData(machine, userDataSecret)).To(Succeed())
			Expect(userDataSecret.Data["userData"]).To(Equal([]byte(fmt.Sprintf(userDataTemplate, "<<BOOTSTRAP_TOKEN>>", machine.Name))))
		})

		It("should do nothing if there is no magic string", func() {
			userDataSecret.Data["userData"] = []byte("foobar")
			Expect(machineController.addMachineNameToUserData(machine, userDataSecret)).To(Succeed())
			Expect(userDataSecret.Data["userData"]).To(Equal([]byte("foobar")))
		})

		It("should fail if the secret does not contain a userData key", func() {
			delete(userDataSecret.Data, "userData")
			Expect(machineController.addMachineNameToUserData(machine, userDataSecret)).To(MatchError(ContainSubstring("userdata field not found in secret for machine")))
		})
	})
})
