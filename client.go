package main

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

type Client struct {
	clientset       *kubernetes.Clientset
	informerFactory informers.SharedInformerFactory
	registry        *Registry
}

func extractBuoyAnnotations(annotations map[string]string) map[string]string {
	buoyAnnotations := make(map[string]string)
	for key, value := range annotations {
		if strings.HasPrefix(key, "buoy.sh/") {
			buoyAnnotations[key] = value
		}
	}
	return buoyAnnotations
}

func buildKubeConfig() (*rest.Config, error) {
	config, err := rest.InClusterConfig()
	if err == nil {
		return config, nil
	}
	kubeconfig := filepath.Join(homedir.HomeDir(), ".kube", "config")
	config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to build config: %w", err)
	}
	return config, nil
}

func NewClient() (*Client, error) {
	config, err := buildKubeConfig()
	if err != nil {
		return nil, err
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create clientset: %w", err)
	}
	informerFactory := informers.NewSharedInformerFactory(clientset, 30*time.Second)
	return &Client{
		clientset:       clientset,
		informerFactory: informerFactory,
		registry:        NewRegistry(),
	}, nil
}

func (c *Client) Start(ctx context.Context) error {
	c.setupPodInformer()
	c.setupDeploymentInformer()
	c.setupStatefulSetInformer()
	c.setupDaemonSetInformer()
	c.informerFactory.Start(ctx.Done())
	synced := c.informerFactory.WaitForCacheSync(ctx.Done())
	for informerType, ok := range synced {
		if !ok {
			return fmt.Errorf("failed to sync cache for %v", informerType)
		}
	}
	slog.Info("⚓ Informers started and synced")
	return nil
}

func (c *Client) setupPodInformer() {
	podInformer := c.informerFactory.Core().V1().Pods().Informer()
	podInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.handlePodAdd,
		UpdateFunc: c.handlePodUpdate,
	})
}

func (c *Client) setupDeploymentInformer() {
	deployInformer := c.informerFactory.Apps().V1().Deployments().Informer()
	deployInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.handleDeploymentAdd,
		DeleteFunc: c.handleDeploymentDelete,
	})
}

func (c *Client) setupStatefulSetInformer() {
	stsInformer := c.informerFactory.Apps().V1().StatefulSets().Informer()
	stsInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.handleStatefulSetAdd,
		DeleteFunc: c.handleStatefulSetDelete,
	})
}

func (c *Client) setupDaemonSetInformer() {
	dsInformer := c.informerFactory.Apps().V1().DaemonSets().Informer()
	dsInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.handleDaemonSetAdd,
		DeleteFunc: c.handleDaemonSetDelete,
	})
}

func (c *Client) handlePodAdd(obj interface{}) {
	pod := obj.(*corev1.Pod)
	c.syncPod(pod)
}

func (c *Client) handlePodUpdate(oldObj, newObj interface{}) {
	pod := newObj.(*corev1.Pod)
	c.syncPod(pod)
}

func (c *Client) syncPod(pod *corev1.Pod) {
	parentNS, parentName, parentKind := c.getGrandparent(pod)
	if parentName == "" || !c.isWatched(parentNS, parentName, parentKind) {
		return
	}
	for _, status := range pod.Status.ContainerStatuses {
		if status.ImageID != "" {
			c.registry.UpdateContainer(parentNS, parentName, parentKind, status.Name, status.ImageID)
		}
	}
}

func (c *Client) getGrandparent(pod *corev1.Pod) (string, string, string) {
	if len(pod.OwnerReferences) == 0 {
		return "", "", ""
	}
	owner := pod.OwnerReferences[0]
	if owner.Kind == "ReplicaSet" {
		return c.resolveReplicaSetOwner(pod.Namespace, owner.Name)
	}
	return pod.Namespace, owner.Name, owner.Kind
}

func (c *Client) resolveReplicaSetOwner(namespace, rsName string) (string, string, string) {
	rs, err := c.clientset.AppsV1().ReplicaSets(namespace).Get(context.Background(), rsName, metav1.GetOptions{})
	if err != nil {
		slog.Debug("Failed to resolve ReplicaSet owner", "namespace", namespace, "replicaset", rsName, "error", err)
		return "", "", ""
	}
	if len(rs.OwnerReferences) == 0 {
		return "", "", ""
	}
	owner := rs.OwnerReferences[0]
	return namespace, owner.Name, owner.Kind
}

func (c *Client) isWatched(namespace, name, kind string) bool {
	_, exists := c.registry.Get(namespace, name, kind)
	return exists
}

func (c *Client) handleDeploymentAdd(obj interface{}) {
	deploy := obj.(*appsv1.Deployment)
	if deploy.Annotations["buoy.sh/watch"] != "true" {
		return
	}
	c.registerResource(deploy.Namespace, deploy.Name, "Deployment", deploy.Annotations)
}

func (c *Client) handleDeploymentDelete(obj interface{}) {
	deploy := obj.(*appsv1.Deployment)
	c.registry.Delete(deploy.Namespace, deploy.Name, "Deployment")
}

func (c *Client) handleStatefulSetAdd(obj interface{}) {
	sts := obj.(*appsv1.StatefulSet)
	if sts.Annotations["buoy.sh/watch"] != "true" {
		return
	}
	c.registerResource(sts.Namespace, sts.Name, "StatefulSet", sts.Annotations)
}

func (c *Client) handleStatefulSetDelete(obj interface{}) {
	sts := obj.(*appsv1.StatefulSet)
	c.registry.Delete(sts.Namespace, sts.Name, "StatefulSet")
}

func (c *Client) handleDaemonSetAdd(obj interface{}) {
	ds := obj.(*appsv1.DaemonSet)
	if ds.Annotations["buoy.sh/watch"] != "true" {
		return
	}
	c.registerResource(ds.Namespace, ds.Name, "DaemonSet", ds.Annotations)
}

func (c *Client) handleDaemonSetDelete(obj interface{}) {
	ds := obj.(*appsv1.DaemonSet)
	c.registry.Delete(ds.Namespace, ds.Name, "DaemonSet")
}

func (c *Client) registerResource(namespace, name, kind string, annotations map[string]string) {
	res := &ObservedResource{
		Namespace:        namespace,
		Name:             name,
		Kind:             kind,
		Annotations:      extractBuoyAnnotations(annotations),
		Containers:       make(map[string]string),
		Schedule:         getSchedule(annotations),
		RequiresApproval: requiresApproval(annotations),
		RemoteSHA:        make(map[string]string),
		LiveSHA:          make(map[string]string),
	}
	c.registry.Set(res)
	slog.Info("⚓ Watching resource",
		"name", name,
		"namespace", namespace,
		"kind", kind,
		"schedule", res.Schedule,
		"requires_approval", res.RequiresApproval,
	)
}

func getSchedule(annotations map[string]string) string {
	if schedule, ok := annotations["buoy.sh/watchSchedule"]; ok {
		return schedule
	}
	return "@hourly"
}

func requiresApproval(annotations map[string]string) bool {
	approval, ok := annotations["buoy.sh/updateApproval"]
	if !ok {
		return false
	}
	return approval == "required" || approval == "true"
}
