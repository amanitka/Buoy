package main

import (
	"context"
	"log/slog"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Scheduler struct {
	client   *Client
	registry *Registry
	updater  *Updater
}

func NewScheduler(client *Client, registry *Registry, updater *Updater) *Scheduler {
	return &Scheduler{
		client:   client,
		registry: registry,
		updater:  updater,
	}
}

func (s *Scheduler) Start(ctx context.Context) {
	s.runInitialCheck()
	s.startPeriodicChecks(ctx)
}

func (s *Scheduler) runInitialCheck() {
	slog.Info("🔍 Running initial SHA comparison check...")
	s.checkAllResources()
}

func (s *Scheduler) startPeriodicChecks(ctx context.Context) {
	go s.runScheduleLoop(ctx, "@hourly", func(now time.Time) time.Time {
		return time.Date(now.Year(), now.Month(), now.Day(), now.Hour()+1, 0, 0, 0, now.Location())
	})
	go s.runScheduleLoop(ctx, "@daily", func(now time.Time) time.Time {
		return time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
	})
}

func (s *Scheduler) runScheduleLoop(ctx context.Context, schedule string, nextTimeFunc func(time.Time) time.Time) {
	for {
		now := time.Now()
		next := nextTimeFunc(now)
		timer := time.NewTimer(next.Sub(now))

		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
			s.checkResourcesBySchedule(schedule)
		}
	}
}

func (s *Scheduler) checkResourcesBySchedule(schedule string) {
	slog.Info("⏳ Running scheduled SHA comparison check", "schedule", schedule)
	resources := s.registry.GetAllResources()
	for _, res := range resources {
		if res.Schedule == schedule {
			s.checkResource(res)
		}
	}
}

func (s *Scheduler) checkAllResources() {
	resources := s.registry.GetAllResources()
	for _, res := range resources {
		s.checkResource(res)
	}
}

func (s *Scheduler) checkResource(res *ObservedResource) {
	pods, err := s.client.clientset.CoreV1().Pods(res.Namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		slog.Debug("Failed to list pods", "namespace", res.Namespace, "error", err)
		return
	}
	for _, pod := range pods.Items {
		s.checkPodForResource(&pod, res)
	}
}

func (s *Scheduler) checkPodForResource(pod *corev1.Pod, res *ObservedResource) {
	parentNS, parentName, parentKind := s.client.getGrandparent(pod)
	if parentNS != res.Namespace || parentName != res.Name || parentKind != res.Kind {
		return
	}
	for _, status := range pod.Status.ContainerStatuses {
		if status.ImageID != "" {
			s.compareImageSHA(res, status.Name, status.Image, status.ImageID)
		}
	}
}

func (s *Scheduler) compareImageSHA(res *ObservedResource, containerName, image, liveImageID string) {
	slog.Info("🔍 Comparing image SHA",
		"resource", res.Name,
		"namespace", res.Namespace,
		"container", containerName,
		"image", image,
	)
	remoteSHA, err := GetRemoteDigest(image)
	if err != nil {
		slog.Debug("Failed to fetch remote digest", "image", image, "error", err)
		return
	}
	liveSHA := extractSHA(liveImageID)
	res.LastChecked = time.Now()
	if liveSHA != "" && remoteSHA != "" && liveSHA != remoteSHA {
		s.handleImageMismatch(res, containerName, image, liveSHA, remoteSHA)
	}
}

func extractSHA(imageID string) string {
	parts := strings.Split(imageID, "@")
	if len(parts) == 2 {
		return parts[1]
	}
	if strings.Contains(imageID, "sha256:") {
		idx := strings.Index(imageID, "sha256:")
		return imageID[idx:]
	}
	return ""
}

func (s *Scheduler) handleImageMismatch(res *ObservedResource, containerName, image, liveSHA, remoteSHA string) {
	s.registry.MarkUpdateAvailable(res.Namespace, res.Name, res.Kind, containerName, remoteSHA)
	slog.Info("🔄 Image SHA mismatch detected",
		"resource", res.Name,
		"namespace", res.Namespace,
		"kind", res.Kind,
		"container", containerName,
		"image", image,
		"live_sha", liveSHA,
		"remote_sha", remoteSHA,
		"requires_approval", res.RequiresApproval,
	)
	if !res.RequiresApproval {
		s.triggerAutoUpdate(res)
	}
}

func (s *Scheduler) triggerAutoUpdate(res *ObservedResource) {
	if err := s.updater.TriggerRollingUpdate(res); err != nil {
		slog.Error("Auto-update failed", "resource", res.Name, "error", err)
		return
	}
	s.registry.MarkUpdating(res.Namespace, res.Name, res.Kind)
	slog.Info("🚀 Auto-update triggered",
		"resource", res.Name,
		"namespace", res.Namespace,
		"kind", res.Kind,
	)
}
