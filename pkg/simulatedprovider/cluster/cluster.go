// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package cluster

import (
	"context"
	"fmt"
	"os"
	"time"

	_ "embed"

	crd "github.com/gardener/machine-controller-manager/kubernetes"
	"github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	controller "github.com/gardener/machine-controller-manager/pkg/util/provider/machinecontroller"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	e2efwkenv "sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/envfuncs"
	"sigs.k8s.io/e2e-framework/support"
	"sigs.k8s.io/e2e-framework/support/kwok"
)

const kwokVersion = "v0.8.0"

//go:embed kwok-config.yaml
var kwokctlConfig []byte

// Env specifies the virtual cluster details/configuration.
type Env struct {
	Name      string
	Namespace string
	Ctx       context.Context
	Cfg       *envconf.Config
}

// New returns a kwokctl created cluster's Environment.
// Name is used as cluster name and optionally a namespace can
// be specified to be created as part of the cluster creation.
func New(name, namespace string) Env {
	// We create an environment and populate its context with the required
	// provider details and cluster name, without this the delete call
	// fails because it expects this information to be present in the passed
	// context.
	kwokProvider := kwok.NewProvider().SetDefaults().WithName(name)
	ctx := context.WithValue(
		context.Background(), support.ClusterNameContextKey(name), kwokProvider,
	)

	return Env{
		Name:      name,
		Namespace: namespace,
		Ctx:       ctx,
		Cfg:       e2efwkenv.New().EnvConf(),
	}
}

// SetupCluster creates a cluster containing MCM CRDs and starts a watch for MachineClass
// objects to create fake secrets for the class' SecretRef and CredentialsSecretRef.
func (env *Env) SetupCluster() (err error) {
	if err = env.createCluster(); err != nil {
		return
	}

	scheme := env.Cfg.Client().Resources().GetScheme()
	if err = apiextensionsv1.AddToScheme(scheme); err != nil {
		return
	}

	if err = env.deployCRDs(); err != nil {
		return
	}

	// Register MCM API objects with the cluster scheme
	if err = v1alpha1.AddToScheme(scheme); err != nil {
		return
	}

	// For every MachineClass that's added, create fake secrets that
	// are part of its SecretRef and CredentialsSecretRef.
	// This is needed for the testing framework as well as for MCM
	// machine reconciliation, wherein it passes the secret when
	// issuing a CreateMachine() call. Ref triggerCreationFlow()
	return env.watchMCCForSecretRefs()
}

// DeleteCluster removes the cluster corresponding to the Env.
func (env *Env) DeleteCluster() (err error) {
	destroyClusterFunc := envfuncs.DestroyCluster(env.Name)
	_, err = destroyClusterFunc(env.Ctx, env.Cfg)
	return
}

// ExportLogs saves the control plane logs in the specified directory.
func (env *Env) ExportLogs(dir string) (err error) {
	exportLogsFunc := envfuncs.ExportClusterLogs(env.Name, dir)
	_, err = exportLogsFunc(env.Ctx, env.Cfg)
	return
}

func (env *Env) createCluster() (err error) {
	// Using the direct path doesn't work since it looks for the file
	// relative to the caller, so the config file is embedded and a
	// temporary file path is passed in order to create the cluster.
	configFile, err := os.CreateTemp("", "kwokctl-config-*")
	if err = os.WriteFile(configFile.Name(), kwokctlConfig, 0600); err != nil {
		return
	}
	defer os.Remove(configFile.Name())

	// Cluster creation is done manually rather than using `CreateClusterWithConfig` helper
	// from e2e-fwk since we wish to override the specified kwokVersion with our own.
	kwokProvider := kwok.NewProvider()
	k := kwokProvider.SetDefaults().WithName(env.Name).WithVersion(kwokVersion)
	kubecfg, err := k.CreateWithConfig(env.Ctx, configFile.Name())
	if err != nil {
		return
	}
	// update envconfig  with kubeconfig
	env.Cfg.WithKubeconfigFile(kubecfg)
	// stall, wait for pods initializations
	err = k.WaitForControlPlane(env.Ctx, env.Cfg.Client())
	if err != nil {
		return
	}
	// store entire cluster value in ctx for future access using the cluster name
	env.Ctx = context.WithValue(env.Ctx, support.ClusterNameContextKey(env.Name), k)

	createNamespaceFunc := envfuncs.CreateNamespace(env.Namespace)
	env.Ctx, err = createNamespaceFunc(env.Ctx, env.Cfg)
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return
	}
	return nil
}

func (env *Env) deployCRDs() (err error) {
	for _, crd := range crd.CRDs {
		err = env.Cfg.Client().Resources().Create(env.Ctx, crd)
		if err != nil && !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("failed to create CRD %q: %w", crd.Name, err)
		}

		// This checks if the CRDs are registered before any MCM watches are
		// started or objects are deployed.
		err = wait.PollUntilContextTimeout(env.Ctx, 100*time.Millisecond, 1*time.Second, false,
			func(ctx context.Context) (bool, error) {
				crdObj := &apiextensionsv1.CustomResourceDefinition{}
				err := env.Cfg.Client().Resources().Get(ctx, crd.Name, "", crdObj)
				if err != nil {
					return false, err
				}
				return true, nil
			},
		)
	}

	return
}

func (env *Env) watchMCCForSecretRefs() error {
	return env.Cfg.Client().Resources().
		Watch(&v1alpha1.MachineClassList{}, func(listOpts *metav1.ListOptions) {
			// Create Secrets for 'any' observed MCCs. Ref:
			// https://kubernetes.io/docs/reference/using-api/api-concepts/#semantics-for-watch
			listOpts.ResourceVersion = "0"
		}).
		WithAddFunc(func(obj any) {
			mcc, ok := obj.(*v1alpha1.MachineClass)
			if !ok || mcc.CredentialsSecretRef == nil || mcc.SecretRef == nil {
				return
			}
			err := env.createFakeSecret(mcc.CredentialsSecretRef.Name, mcc.CredentialsSecretRef.Namespace)
			if err != nil && !apierrors.IsAlreadyExists(err) {
				fmt.Printf("ERR: Creating secret %q for %q: %v\n",
					mcc.CredentialsSecretRef.Name, mcc.Name, err,
				)
				return
			}
			err = env.createFakeSecret(mcc.SecretRef.Name, mcc.SecretRef.Namespace)
			if err != nil && !apierrors.IsAlreadyExists(err) {
				fmt.Printf("ERR: Creating secret %q for %q: %v\n",
					mcc.SecretRef.Name, mcc.Name, err,
				)
				return
			}
		}).
		Start(env.Ctx)
}

func (env *Env) createFakeSecret(name, namespace string) (err error) {
	secret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: map[string][]byte{},
		StringData: map[string]string{
			"userData": fmt.Sprintf(
				"fake-data%s%s",
				controller.BootstrapTokenPlaceholder,
				controller.MachineNamePlaceholder,
			),
		},
		Type: corev1.SecretTypeOpaque,
	}
	err = env.Cfg.Client().Resources().Create(env.Ctx, &secret)
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("error creating secret %s: %w", name, err)
	}
	return
}
