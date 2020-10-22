package framework

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	gardenerutils "github.com/gardener/gardener/pkg/utils"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
	"github.com/gardener/gardener/pkg/utils/retry"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	k8sretry "k8s.io/client-go/util/retry"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// WaitUntilDaemonSetIsRunning waits until the daemon set with <daemonSetName> is running
func (f *CommonFramework) WaitUntilDaemonSetIsRunning(ctx context.Context, k8sClient client.Client, daemonSetName, daemonSetNamespace string) error {
	return retry.Until(ctx, defaultPollInterval, func(ctx context.Context) (done bool, err error) {
		daemonSet := &appsv1.DaemonSet{}
		if err := k8sClient.Get(ctx, client.ObjectKey{Namespace: daemonSetNamespace, Name: daemonSetName}, daemonSet); err != nil {
			return retry.MinorError(err)
		}

		if err := health.CheckDaemonSet(daemonSet); err != nil {
			f.Logger.Infof("Waiting for %s to be ready!!", daemonSetName)
			return retry.MinorError(fmt.Errorf("daemon set %s is not healthy: %v", daemonSetName, err))
		}

		f.Logger.Infof("%s is now ready!!", daemonSetName)
		return retry.Ok()
	})
}

// WaitUntilStatefulSetIsRunning waits until the stateful set with <statefulSetName> is running
func (f *CommonFramework) WaitUntilStatefulSetIsRunning(ctx context.Context, statefulSetName, statefulSetNamespace string, c kubernetes.Interface) error {
	return retry.Until(ctx, defaultPollInterval, func(ctx context.Context) (done bool, err error) {
		statefulSet := &appsv1.StatefulSet{}
		if err := c.DirectClient().Get(ctx, client.ObjectKey{Namespace: statefulSetNamespace, Name: statefulSetName}, statefulSet); err != nil {
			return retry.MinorError(err)
		}

		if err := health.CheckStatefulSet(statefulSet); err != nil {
			f.Logger.Infof("Waiting for %s to be ready!!", statefulSetName)
			return retry.MinorError(fmt.Errorf("stateful set %s is not healthy: %v", statefulSetName, err))
		}

		f.Logger.Infof("%s is now ready!!", statefulSetName)
		return retry.Ok()
	})
}

// WaitUntilDeploymentIsReady waits until the given deployment is ready
func (f *CommonFramework) WaitUntilDeploymentIsReady(ctx context.Context, name string, namespace string, k8sClient kubernetes.Interface) error {
	return retry.Until(ctx, defaultPollInterval, func(ctx context.Context) (done bool, err error) {
		deployment := &appsv1.Deployment{}
		if err := k8sClient.DirectClient().Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, deployment); err != nil {
			if apierrors.IsNotFound(err) {
				f.Logger.Infof("Waiting for deployment %s/%s to be ready!", namespace, name)
				return retry.MinorError(fmt.Errorf("deployment %q in namespace %q does not exist", name, namespace))
			}
			return retry.SevereError(err)
		}

		err = health.CheckDeployment(deployment)
		if err != nil {
			f.Logger.Infof("Waiting for deployment %s/%s to be ready!", namespace, name)
			return retry.MinorError(fmt.Errorf("deployment %q in namespace %q is not healthy", name, namespace))
		}
		return retry.Ok()
	})
}

// WaitUntilDeploymentsWithLabelsIsReady wait until pod with labels <podLabels> is running
func (f *CommonFramework) WaitUntilDeploymentsWithLabelsIsReady(ctx context.Context, deploymentLabels labels.Selector, namespace string, k8sClient kubernetes.Interface) error {
	return retry.Until(ctx, defaultPollInterval, func(ctx context.Context) (done bool, err error) {
		deployments := &appsv1.DeploymentList{}
		if err := k8sClient.DirectClient().List(ctx, deployments, client.MatchingLabelsSelector{Selector: deploymentLabels}, client.InNamespace(namespace)); err != nil {
			if apierrors.IsNotFound(err) {
				f.Logger.Infof("Waiting for deployments with labels: %v to be ready!!", deploymentLabels.String())
				return retry.MinorError(fmt.Errorf("no deployments with labels %s exist", deploymentLabels.String()))
			}
			return retry.SevereError(err)
		}

		for _, deployment := range deployments.Items {
			err = health.CheckDeployment(&deployment)
			if err != nil {
				f.Logger.Infof("Waiting for deployments with labels: %v to be ready!!", deploymentLabels)
				return retry.MinorError(fmt.Errorf("deployment %s is not healthy: %v", deployment.Name, err))
			}
		}
		return retry.Ok()
	})
}

// WaitUntilNamespaceIsDeleted waits until a namespace has been deleted
func (f *CommonFramework) WaitUntilNamespaceIsDeleted(ctx context.Context, k8sClient kubernetes.Interface, ns string) error {
	return retry.Until(ctx, defaultPollInterval, func(ctx context.Context) (bool, error) {
		if err := k8sClient.DirectClient().Get(ctx, client.ObjectKey{Name: ns}, &corev1.Namespace{}); err != nil {
			if apierrors.IsNotFound(err) {
				return retry.Ok()
			}
			return retry.MinorError(err)
		}
		return retry.MinorError(errors.Errorf("Namespace %s is not deleted yet", ns))
	})
}

// WaitForNNodesToBeHealthy waits for exactly <n> Nodes to be healthy within a given timeout
func WaitForNNodesToBeHealthy(ctx context.Context, k8sClient kubernetes.Interface, n int, timeout time.Duration) error {
	return WaitForNNodesToBeHealthyInWorkerPool(ctx, k8sClient, n, nil, timeout)
}

// WaitForNNodesToBeHealthyInWorkerPool waits for exactly <n> Nodes in a given worker pool to be healthy within a given timeout
func WaitForNNodesToBeHealthyInWorkerPool(ctx context.Context, k8sClient kubernetes.Interface, n int, workerGroup *string, timeout time.Duration) error {
	return retry.UntilTimeout(ctx, defaultPollInterval, timeout, func(ctx context.Context) (done bool, err error) {
		nodeList, err := GetAllNodesInWorkerPool(ctx, k8sClient, workerGroup)
		if err != nil {
			return retry.SevereError(err)
		}

		nodeCount := len(nodeList.Items)
		if nodeCount != n {
			return retry.MinorError(fmt.Errorf("waiting for exactly %d nodes to be ready: only %d nodes registered in the cluster", n, nodeCount))
		}

		for _, node := range nodeList.Items {
			if err := health.CheckNode(&node); err != nil {
				return retry.MinorError(fmt.Errorf("waiting for exactly %d nodes to be ready: node %q is not healthy: %v", n, node.Name, err))
			}
		}

		return retry.Ok()
	})
}

// GetAllNodes fetches all nodes
func GetAllNodes(ctx context.Context, c kubernetes.Interface) (*corev1.NodeList, error) {
	return GetAllNodesInWorkerPool(ctx, c, nil)
}

// GetAllNodesInWorkerPool fetches all nodes of a specific worker group
func GetAllNodesInWorkerPool(ctx context.Context, c kubernetes.Interface, workerGroup *string) (*corev1.NodeList, error) {
	nodeList := &corev1.NodeList{}

	selectorOption := &client.MatchingLabelsSelector{}
	if workerGroup != nil && len(*workerGroup) > 0 {
		selectorOption.Selector = labels.SelectorFromSet(labels.Set{"worker.gardener.cloud/pool": *workerGroup})
	}

	err := c.DirectClient().List(ctx, nodeList, selectorOption)
	return nodeList, err
}

// GetPodsByLabels fetches all pods with the desired set of labels <labelsMap>
func GetPodsByLabels(ctx context.Context, labelsSelector labels.Selector, c kubernetes.Interface, namespace string) (*corev1.PodList, error) {
	podList := &corev1.PodList{}
	err := c.DirectClient().List(ctx, podList,
		client.InNamespace(namespace),
		client.MatchingLabelsSelector{Selector: labelsSelector})
	if err != nil {
		return nil, err
	}
	return podList, nil
}

// GetFirstRunningPodWithLabels fetches the first running pod with the desired set of labels <labelsMap>
func GetFirstRunningPodWithLabels(ctx context.Context, labelsMap labels.Selector, namespace string, client kubernetes.Interface) (*corev1.Pod, error) {
	var (
		podList *corev1.PodList
		err     error
	)
	podList, err = GetPodsByLabels(ctx, labelsMap, client, namespace)
	if err != nil {
		return nil, err
	}
	if len(podList.Items) == 0 {
		return nil, ErrNoRunningPodsFound
	}

	for _, pod := range podList.Items {
		if health.IsPodReady(&pod) {
			return &pod, nil
		}
	}

	return nil, ErrNoRunningPodsFound
}

// PodExecByLabel executes a command inside pods filtered by label
func PodExecByLabel(ctx context.Context, podLabels labels.Selector, podContainer, command, namespace string, client kubernetes.Interface) (io.Reader, error) {
	pod, err := GetFirstRunningPodWithLabels(ctx, podLabels, namespace, client)
	if err != nil {
		return nil, err
	}

	return NewPodExecutor(client).Execute(ctx, pod.Namespace, pod.Name, podContainer, command)
}

// DeleteAndWaitForResource deletes a kubernetes resource and waits for its deletion
func DeleteAndWaitForResource(ctx context.Context, k8sClient kubernetes.Interface, resource runtime.Object, timeout time.Duration) error {
	if err := kutil.DeleteObject(ctx, k8sClient.DirectClient(), resource); err != nil {
		return err
	}
	return retry.UntilTimeout(ctx, 5*time.Second, timeout, func(ctx context.Context) (done bool, err error) {
		newResource := resource.DeepCopyObject()
		key, err := client.ObjectKeyFromObject(resource)
		if err != nil {
			return retry.MinorError(err)
		}

		if err := k8sClient.DirectClient().Get(ctx, key, newResource); err != nil {
			if apierrors.IsNotFound(err) {
				return retry.Ok()
			}
			return retry.MinorError(err)
		}
		return retry.MinorError(errors.New("Object still exists"))
	})
}

// ScaleDeployment scales a deployment and waits until it is scaled
func ScaleDeployment(timeout time.Duration, client client.Client, desiredReplicas *int32, name, namespace string) (*int32, error) {
	if desiredReplicas == nil {
		return nil, nil
	}

	ctxSetup, cancelCtxSetup := context.WithTimeout(context.Background(), timeout)
	defer cancelCtxSetup()

	replicas, err := GetDeploymentReplicas(ctxSetup, client, namespace, name)
	if apierrors.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve the replica count of the %s deployment: '%v'", name, err)
	}
	if replicas == nil || *replicas == *desiredReplicas {
		return replicas, nil
	}

	// scale the deployment
	if err := kubernetes.ScaleDeployment(ctxSetup, client, kutil.Key(namespace, name), *desiredReplicas); err != nil {
		return nil, fmt.Errorf("failed to scale the replica count of the %s deployment: '%v'", name, err)
	}

	// wait until scaled
	if err := WaitUntilDeploymentScaled(ctxSetup, client, namespace, name, *desiredReplicas); err != nil {
		return nil, fmt.Errorf("failed to wait until the %s deployment is scaled: '%v'", name, err)
	}
	return replicas, nil
}

// WaitUntilDeploymentScaled waits until the deployment has the desired replica count in the status
func WaitUntilDeploymentScaled(ctx context.Context, client client.Client, namespace, name string, desiredReplicas int32) error {
	return retry.Until(ctx, 5*time.Second, func(ctx context.Context) (done bool, err error) {
		deployment := &appsv1.Deployment{}
		if err := client.Get(ctx, kutil.Key(namespace, name), deployment); err != nil {
			return retry.SevereError(err)
		}
		if deployment.Spec.Replicas == nil || *deployment.Spec.Replicas != desiredReplicas {
			return retry.SevereError(fmt.Errorf("waiting for deployment scale failed. spec.replicas does not match the desired replicas"))
		}

		if deployment.Status.Replicas == desiredReplicas && deployment.Status.AvailableReplicas == desiredReplicas {
			return retry.Ok()
		}

		return retry.MinorError(fmt.Errorf("deployment currently has '%d' replicas. Desired: %d", deployment.Status.AvailableReplicas, desiredReplicas))
	})
}

// GetDeploymentReplicas gets the spec.Replicas count from a deployment
func GetDeploymentReplicas(ctx context.Context, client client.Client, namespace, name string) (*int32, error) {
	deployment := &appsv1.Deployment{}
	if err := client.Get(ctx, kutil.Key(namespace, name), deployment); err != nil {
		return nil, err
	}
	replicas := deployment.Spec.Replicas
	return replicas, nil
}

// ShootCreationCompleted checks if a shoot is successfully reconciled. In case it is not, it also returns a descriptive message stating the reason.
func ShootCreationCompleted(newShoot *gardencorev1beta1.Shoot) (bool, string) {
	if newShoot.Generation != newShoot.Status.ObservedGeneration {
		return false, "shoot generation did not equal observed generation"
	}
	if len(newShoot.Status.Conditions) == 0 && newShoot.Status.LastOperation == nil {
		return false, "no conditions and last operation present yet"
	}

	for _, condition := range newShoot.Status.Conditions {
		if condition.Status != gardencorev1beta1.ConditionTrue {
			return false, fmt.Sprintf("condition type %s is not true yet, had message %s with reason %s", condition.Type, condition.Message, condition.Reason)
		}
	}

	if newShoot.Status.LastOperation != nil {
		if newShoot.Status.LastOperation.Type == gardencorev1beta1.LastOperationTypeCreate ||
			newShoot.Status.LastOperation.Type == gardencorev1beta1.LastOperationTypeReconcile {
			if newShoot.Status.LastOperation.State != gardencorev1beta1.LastOperationStateSucceeded {
				return false, "last operation type was create or reconcile but state was not succeeded"
			}
		}
	}

	return true, ""
}

// DownloadKubeconfig downloads the shoot Kubeconfig
func DownloadKubeconfig(ctx context.Context, client kubernetes.Interface, namespace, name, downloadPath string) error {
	kubeconfig, err := GetObjectFromSecret(ctx, client, namespace, name, KubeconfigSecretKeyName)
	if err != nil {
		return err
	}
	if downloadPath != "" {
		err = ioutil.WriteFile(downloadPath, []byte(kubeconfig), 0755)
		if err != nil {
			return err
		}
	}
	return nil
}

// UpdateSecret updates the Secret with an backoff
func UpdateSecret(ctx context.Context, k8sClient kubernetes.Interface, secret *corev1.Secret) error {
	if err := k8sretry.RetryOnConflict(k8sretry.DefaultBackoff, func() (err error) {
		existingSecret := &corev1.Secret{}
		if err = k8sClient.DirectClient().Get(ctx, client.ObjectKey{Namespace: secret.Namespace, Name: secret.Name}, existingSecret); err != nil {
			return err
		}
		existingSecret.Data = secret.Data
		return k8sClient.DirectClient().Update(ctx, existingSecret)
	}); err != nil {
		return err
	}
	return nil
}

// GetObjectFromSecret returns object from secret
func GetObjectFromSecret(ctx context.Context, k8sClient kubernetes.Interface, namespace, secretName, objectKey string) (string, error) {
	secret := &corev1.Secret{}
	err := k8sClient.DirectClient().Get(ctx, client.ObjectKey{Namespace: namespace, Name: secretName}, secret)
	if err != nil {
		return "", err
	}

	if _, ok := secret.Data[objectKey]; ok {
		return string(secret.Data[objectKey]), nil
	}
	return "", fmt.Errorf("secret %s/%s did not contain object key %q", namespace, secretName, objectKey)
}

// NewClientFromServiceAccount returns a kubernetes client for a service account.
func NewClientFromServiceAccount(ctx context.Context, k8sClient kubernetes.Interface, account *corev1.ServiceAccount) (kubernetes.Interface, error) {
	secret := &corev1.Secret{}
	err := k8sClient.DirectClient().Get(ctx, client.ObjectKey{Namespace: account.Namespace, Name: account.Secrets[0].Name}, secret)
	if err != nil {
		return nil, err
	}

	serviceAccountConfig := &rest.Config{
		Host: k8sClient.RESTConfig().Host,
		TLSClientConfig: rest.TLSClientConfig{
			Insecure: false,
			CAData:   secret.Data["ca.crt"],
		},
		BearerToken: string(secret.Data["token"]),
	}

	return kubernetes.NewWithConfig(
		kubernetes.WithRESTConfig(serviceAccountConfig),
		kubernetes.WithClientOptions(
			client.Options{
				Scheme: kubernetes.GardenScheme,
			}),
	)
}

// WaitUntilPodIsRunning waits until the pod with <podName> is running
func WaitUntilPodIsRunning(ctx context.Context, log *logrus.Logger, podName, podNamespace string, c kubernetes.Interface) error {
	return retry.Until(ctx, defaultPollInterval, func(ctx context.Context) (done bool, err error) {
		pod := &corev1.Pod{}
		if err := c.DirectClient().Get(ctx, client.ObjectKey{Namespace: podNamespace, Name: podName}, pod); err != nil {
			return retry.SevereError(err)
		}
		if !health.IsPodReady(pod) {
			log.Infof("Waiting for %s to be ready!!", podName)
			log.Infof("Waiting for %s to be ready!!", podName)
			return retry.MinorError(fmt.Errorf(`pod "%s/%s" is not ready: %v`, podNamespace, podName, err))
		}

		return retry.Ok()
	})
}

// WaitUntilPodIsRunningWithLabels waits until the pod with <podLabels> is running
func (f *CommonFramework) WaitUntilPodIsRunningWithLabels(ctx context.Context, labels labels.Selector, podNamespace string, c kubernetes.Interface) error {
	return retry.Until(ctx, defaultPollInterval, func(ctx context.Context) (done bool, err error) {
		pod, err := GetFirstRunningPodWithLabels(ctx, labels, podNamespace, c)

		if err != nil {
			return retry.SevereError(err)
		}

		if !health.IsPodReady(pod) {
			f.Logger.Infof("Waiting for %s to be ready!!", pod.GetName())
			return retry.MinorError(fmt.Errorf(`pod "%s/%s" is not ready: %v`, pod.GetNamespace(), pod.GetName(), err))
		}

		return retry.Ok()
	})
}

// DeployRootPod deploys a pod with root permissions for testing purposes.
func DeployRootPod(ctx context.Context, c client.Client, namespace string, nodename *string) (*corev1.Pod, error) {
	podPriority := int32(0)
	allowedCharacters := "0123456789abcdefghijklmnopqrstuvwxyz"
	id, err := gardenerutils.GenerateRandomStringFromCharset(3, allowedCharacters)
	if err != nil {
		return nil, err
	}

	rootPod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("rootpod-%s", id),
			Namespace: namespace,
			Annotations: map[string]string{
				"kubernetes.io/psp": "gardener.privileged",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "root-container",
					Image: "busybox",
					Command: []string{
						"sleep",
						"10000000",
					},
					Resources:                corev1.ResourceRequirements{},
					TerminationMessagePath:   "/dev/termination-log",
					TerminationMessagePolicy: corev1.TerminationMessageReadFile,
					ImagePullPolicy:          corev1.PullIfNotPresent,
					SecurityContext: &corev1.SecurityContext{
						Privileged: pointer.BoolPtr(true),
					},
					Stdin: true,
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "root-volume",
							MountPath: "/hostroot",
						},
					},
				},
			},
			HostNetwork: true,
			HostPID:     true,
			Priority:    &podPriority,
			Volumes: []corev1.Volume{
				{
					Name: "root-volume",
					VolumeSource: corev1.VolumeSource{
						HostPath: &corev1.HostPathVolumeSource{
							Path: "/",
						},
					},
				},
			},
		},
	}

	if nodename != nil {
		rootPod.Spec.NodeName = *nodename
	}

	if err := c.Create(ctx, &rootPod); err != nil {
		return nil, err
	}
	return &rootPod, nil
}
