package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
)

type Updater struct {
	clientset *kubernetes.Clientset
}

func NewUpdater(clientset *kubernetes.Clientset) *Updater {
	return &Updater{clientset: clientset}
}

func (u *Updater) TriggerRollingUpdate(res *ObservedResource) error {
	switch res.Kind {
	case "Deployment":
		return u.restartDeployment(res.Namespace, res.Name)
	case "StatefulSet":
		return u.restartStatefulSet(res.Namespace, res.Name)
	case "DaemonSet":
		return u.restartDaemonSet(res.Namespace, res.Name)
	default:
		return fmt.Errorf("unsupported resource kind: %s", res.Kind)
	}
}

func (u *Updater) restartDeployment(namespace, name string) error {
	patch := createRestartPatch()
	_, err := u.clientset.AppsV1().Deployments(namespace).Patch(
		context.Background(),
		name,
		types.StrategicMergePatchType,
		patch,
		metav1.PatchOptions{},
	)
	if err != nil {
		return fmt.Errorf("failed to restart deployment: %w", err)
	}
	slog.Info("🚀 Rolling update triggered",
		"kind", "Deployment",
		"namespace", namespace,
		"name", name,
	)
	return nil
}

func (u *Updater) restartStatefulSet(namespace, name string) error {
	patch := createRestartPatch()
	_, err := u.clientset.AppsV1().StatefulSets(namespace).Patch(
		context.Background(),
		name,
		types.StrategicMergePatchType,
		patch,
		metav1.PatchOptions{},
	)
	if err != nil {
		return fmt.Errorf("failed to restart statefulset: %w", err)
	}
	slog.Info("🚀 Rolling update triggered",
		"kind", "StatefulSet",
		"namespace", namespace,
		"name", name,
	)
	return nil
}

func (u *Updater) restartDaemonSet(namespace, name string) error {
	patch := createRestartPatch()
	_, err := u.clientset.AppsV1().DaemonSets(namespace).Patch(
		context.Background(),
		name,
		types.StrategicMergePatchType,
		patch,
		metav1.PatchOptions{},
	)
	if err != nil {
		return fmt.Errorf("failed to restart daemonset: %w", err)
	}
	slog.Info("🚀 Rolling update triggered",
		"kind", "DaemonSet",
		"namespace", namespace,
		"name", name,
	)
	return nil
}

func createRestartPatch() []byte {
	patch := map[string]interface{}{
		"spec": map[string]interface{}{
			"template": map[string]interface{}{
				"metadata": map[string]interface{}{
					"annotations": map[string]string{
						"kubectl.kubernetes.io/restartedAt": time.Now().Format(time.RFC3339),
					},
				},
			},
		},
	}
	patchBytes, _ := json.Marshal(patch)
	return patchBytes
}
