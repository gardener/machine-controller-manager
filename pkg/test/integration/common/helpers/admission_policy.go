// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helpers

import (
	"context"
	"fmt"
	"log"
	"strings"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// VAPName is the name of the ValidatingAdmissionPolicy that blocks kubelet from updating node leases and node status.
	VAPName = "integration-test-block-node-heartbeats"
	// VAPBName is the name of the ValidatingAdmissionPolicyBinding that binds the above ValidatingAdmissionPolicy to the cluster.
	VAPBName = "integration-test-block-node-heartbeats-binding"
)

// CreateVAPToBlockKubeletUpdates is a utility method to create a ValidatingAdmissionPolicy and ValidatingAdmissionPolicyBinding
// to block kubelet from updating node leases and node status.
// This is used to cause nodes to go into the NotReady state to test the machine preservation feature of MCM.
// TODO: pass context from caller
func (c *Cluster) CreateVAPToBlockKubeletUpdates(ctx context.Context, nodeNames []string) error {
	var err error
	for _, noName := range nodeNames {
		_, err := c.Clientset.CoreV1().Nodes().Get(ctx, noName, metav1.GetOptions{})
		if err != nil {
			log.Printf("error fetching node: %v", err)
			return err
		}
	}

	VAPolicy := &admissionregistrationv1.ValidatingAdmissionPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: VAPName,
		},
		Spec: admissionregistrationv1.ValidatingAdmissionPolicySpec{
			MatchConstraints: &admissionregistrationv1.MatchResources{
				ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
					{
						RuleWithOperations: admissionregistrationv1.RuleWithOperations{
							Operations: []admissionregistrationv1.OperationType{
								admissionregistrationv1.Update,
							},
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{""},
								APIVersions: []string{"v1"},
								Resources:   []string{"nodes/status"},
							},
						},
					},
					{
						RuleWithOperations: admissionregistrationv1.RuleWithOperations{
							Operations: []admissionregistrationv1.OperationType{
								admissionregistrationv1.Update,
							},
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{"coordination.k8s.io"},
								APIVersions: []string{"v1"},
								Resources:   []string{"leases"},
							},
						},
					},
				},
			},
			Validations: []admissionregistrationv1.Validation{
				{
					Expression: blockedKubeletExpression(nodeNames),
					Message:    "blocking kubelet heartbeat for test",
				},
			},
		},
	}

	VAPBinding := &admissionregistrationv1.ValidatingAdmissionPolicyBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: VAPBName,
		},
		Spec: admissionregistrationv1.ValidatingAdmissionPolicyBindingSpec{
			PolicyName: VAPName,
			ValidationActions: []admissionregistrationv1.ValidationAction{
				admissionregistrationv1.Deny,
			},
		},
	}

	_, err = c.Clientset.AdmissionregistrationV1().ValidatingAdmissionPolicies().Create(ctx, VAPolicy, metav1.CreateOptions{})
	if errors.IsAlreadyExists(err) {
		existingVAPolicy, err := c.Clientset.AdmissionregistrationV1().ValidatingAdmissionPolicies().Get(ctx, VAPName, metav1.GetOptions{})
		if err != nil {
			log.Printf("error fetching validating admission policy: %v", err)
			return err
		}
		VAPolicy.ResourceVersion = existingVAPolicy.ResourceVersion

		_, err = c.Clientset.AdmissionregistrationV1().ValidatingAdmissionPolicies().Update(ctx, VAPolicy, metav1.UpdateOptions{})
		if err != nil {
			log.Printf("error updating validating admission policy: %v", err)
			return err
		}
	} else if err != nil {
		log.Printf("error creating validating admission policy: %v", err)
		return err
	}

	_, err = c.Clientset.AdmissionregistrationV1().ValidatingAdmissionPolicyBindings().Create(ctx, VAPBinding, metav1.CreateOptions{})
	if errors.IsAlreadyExists(err) {
		existingVAPBinding, err := c.Clientset.AdmissionregistrationV1().ValidatingAdmissionPolicyBindings().Get(ctx, VAPBName, metav1.GetOptions{})
		if err != nil {
			log.Printf("error fetching validating admission policy binding: %v", err)
			return err
		}
		VAPBinding.ResourceVersion = existingVAPBinding.ResourceVersion

		_, err = c.Clientset.AdmissionregistrationV1().ValidatingAdmissionPolicyBindings().Update(ctx, VAPBinding, metav1.UpdateOptions{})
		if err != nil {
			log.Printf("error updating validating admission policy binding: %v", err)
			return err
		}
	} else if err != nil {
		log.Printf("error creating validating admission policy binding: %v", err)
		return err
	}

	return nil
}

func blockedKubeletExpression(nodes []string) string {
	users := make([]string, 0, len(nodes))

	for _, node := range nodes {
		users = append(users, fmt.Sprintf(`"system:node:%s"`, node))
	}

	return fmt.Sprintf(
		"!(request.userInfo.username in [%s])",
		strings.Join(users, ", "),
	)
}

// DeleteVAPToRestartKubeletUpdates deletes the ValidatingAdmissionPolicy and ValidatingAdmissionPolicyBinding that were created to block kubelet from updating node leases and node status.
func (c *Cluster) DeleteVAPToRestartKubeletUpdates(ctx context.Context) error {
	err := c.Clientset.AdmissionregistrationV1().ValidatingAdmissionPolicies().Delete(ctx, VAPName, metav1.DeleteOptions{})
	if err != nil {
		log.Printf("error deleting validating admission policy: %v", err)
		return err
	}

	err = c.Clientset.AdmissionregistrationV1().ValidatingAdmissionPolicyBindings().Delete(ctx, VAPBName, metav1.DeleteOptions{})
	if err != nil {
		log.Printf("error deleting validating admission policy binding: %v", err)
		return err
	}

	return nil
}
