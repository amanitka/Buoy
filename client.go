package main

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

type Client struct {
	clientset *kubernetes.Clientset
}

func extractAnnotations(annotations map[string]string) map[string]string {
	buoyAnnotations := make(map[string]string)
	for key, value := range annotations {
		if strings.HasPrefix(key, "buoy.sh/") {
			buoyAnnotations[key] = value
		}
	}
	return buoyAnnotations
}

func extractInfo(name string, kind string, ns string, annotations map[string]string, containers []corev1.Container) WatchableResource {
	var containerData []ContainerInfo
	for _, c := range containers {
		containerData = append(containerData, ContainerInfo{
			Name:  c.Name,
			Image: c.Image,
		})
	}
	return WatchableResource{
		Name:          name,
		Namespace:     ns,
		Kind:          kind,
		Annotations:   extractAnnotations(annotations),
		ContainerData: containerData,
	}
}

func NewClient() (*Client, error) {
	var config *rest.Config
	var err error

	// 1. Try In-Cluster
	config, err = rest.InClusterConfig()
	if err != nil {
		// 2. Fallback to Kubeconfig
		kubeconfig := filepath.Join(homedir.HomeDir(), ".kube", "config")
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, fmt.Errorf("failed to build config: %w", err)
		}
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create clientset: %w", err)
	}

	return &Client{clientset: clientset}, nil
}

func (c *Client) GetLiveDigest(ctx context.Context, namespace, ownerName, ownerKind string) (string, error) {
	pods, err := c.clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return "", err
	}

	for _, pod := range pods.Items {
		// Look through the owners of this pod
		for _, ref := range pod.OwnerReferences {
			// Check if this pod belongs to our specific Resource Name and Kind
			// Note: Deployments actually own ReplicaSets, which own Pods.
			// For simplicity in a side-project, we check Name prefix + Kind.
			if ref.Name == ownerName && ref.Kind == ownerKind {
				for _, status := range pod.Status.ContainerStatuses {
					// Returns the 'imageID' which includes the sha256
					return status.ImageID, nil
				}
			}
		}
	}
	return "", fmt.Errorf("no live containers found for %s %s", ownerKind, ownerName)
}

func (c *Client) GetAllWatchableResources(ctx context.Context, ns string) ([]WatchableResource, error) {
	var resources []WatchableResource

	// Fetch Deployments
	deploys, _ := c.clientset.AppsV1().Deployments(ns).List(ctx, metav1.ListOptions{})
	for _, d := range deploys.Items {
		if d.Annotations["buoy.sh/watch"] == "true" {
			resources = append(resources, extractInfo(d.Name, "Deployment", d.Namespace, d.Annotations, d.Spec.Template.Spec.Containers))
		}
	}

	// Fetch StatefulSets
	ss, _ := c.clientset.AppsV1().StatefulSets(ns).List(ctx, metav1.ListOptions{})
	for _, s := range ss.Items {
		if s.Annotations["buoy.sh/watch"] == "true" {
			resources = append(resources, extractInfo(s.Name, "StatefulSet", s.Namespace, s.Annotations, s.Spec.Template.Spec.Containers))
		}
	}

	// Fetch DaemonSets
	ds, _ := c.clientset.AppsV1().DaemonSets(ns).List(ctx, metav1.ListOptions{})
	for _, d := range ds.Items {
		if d.Annotations["buoy.sh/watch"] == "true" {
			resources = append(resources, extractInfo(d.Name, "DaemonSet", d.Namespace, d.Annotations, d.Spec.Template.Spec.Containers))
		}
	}

	return resources, nil
}
