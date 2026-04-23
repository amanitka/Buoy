package main

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"sync"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

type Registry struct {
	mu        sync.RWMutex
	resources map[string]*ObservedResource
}

func NewRegistry() *Registry {
	return &Registry{
		resources: make(map[string]*ObservedResource),
	}
}

func (r *Registry) getKey(namespace, name, kind string) string {
	return fmt.Sprintf("%s/%s/%s", namespace, kind, name)
}

func (r *Registry) Get(namespace, name, kind string) (*ObservedResource, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	res, ok := r.resources[r.getKey(namespace, name, kind)]
	return res, ok
}

func (r *Registry) Set(res *ObservedResource) {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := r.getKey(res.Namespace, res.Name, res.Kind)
	r.resources[key] = res
}

func (r *Registry) Delete(namespace, name, kind string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.resources, r.getKey(namespace, name, kind))
}

func (r *Registry) UpdateContainer(namespace, name, kind, containerName, imageID string, creationTime time.Time) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := r.getKey(namespace, name, kind)
	res, exists := r.resources[key]
	if !exists {
		return false
	}

	// Ignore older pods during a rollout to prevent flip-flopping
	if !res.LatestPodCreation.IsZero() && creationTime.Before(res.LatestPodCreation) {
		return false
	}
	if creationTime.After(res.LatestPodCreation) {
		res.LatestPodCreation = creationTime
	}

	if res.Containers == nil {
		res.Containers = make(map[string]string)
	}
	if res.LiveSHA == nil {
		res.LiveSHA = make(map[string]string)
	}
	oldID, hadOld := res.Containers[containerName]
	if hadOld && oldID == imageID {
		return false
	}
	res.Containers[containerName] = imageID

	newLiveSHA := extractSHAFromImageID(imageID)
	res.LiveSHA[containerName] = newLiveSHA

	// If we are updating, check if the new pod brings us to the desired state or timeout
	if res.Status == "Updating" {
		timeoutReached := !res.UpdatingSince.IsZero() && time.Since(res.UpdatingSince) > 5*time.Minute
		expectedRemoteSHA, ok := res.RemoteSHA[containerName]

		if (ok && newLiveSHA == expectedRemoteSHA) || timeoutReached {
			if timeoutReached {
				res.Status = "Updated (Unverified)"
				slog.Info("⏳ Update timeout reached, marking as unverified", "resource", name, "namespace", namespace, "kind", kind)
			} else {
				res.Status = "UpToDate"
			}
			res.UpdateAvailable = false
			res.PendingApproval = false
			res.UpdatingSince = time.Time{}
		}
	}

	r.logContainerChange(namespace, name, kind, containerName, imageID, oldID, hadOld)
	return true
}

func (r *Registry) logContainerChange(namespace, name, kind, containerName, imageID, oldID string, hadOld bool) {
	if hadOld {
		slog.Info("🔄 Image SHA updated",
			"resource", name,
			"namespace", namespace,
			"kind", kind,
			"container", containerName,
			"old_sha", oldID,
			"new_sha", imageID,
		)
		return
	}
	slog.Info("✨ New container detected",
		"resource", name,
		"namespace", namespace,
		"kind", kind,
		"container", containerName,
	)
}

func (r *Registry) MarkUpdateAvailable(namespace, name, kind, containerName, remoteSHA string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := r.getKey(namespace, name, kind)
	res, exists := r.resources[key]
	if !exists {
		return
	}
	if res.Status == "Updating" {
		return // Do not transition back if we are currently updating
	}
	if res.RemoteSHA == nil {
		res.RemoteSHA = make(map[string]string)
	}
	res.RemoteSHA[containerName] = remoteSHA
	res.UpdateAvailable = true
	if res.RequiresApproval {
		res.PendingApproval = true
		res.Status = "PendingApproval"
	} else {
		res.Status = "UpdateAvailable"
	}
}

func (r *Registry) MarkUpdating(namespace, name, kind string) {
	r.mu.Lock()
	key := r.getKey(namespace, name, kind)
	res, exists := r.resources[key]
	if exists {
		res.Status = "Updating"
		res.UpdateAvailable = false
		res.PendingApproval = false
		updatingSince := time.Now()
		res.UpdatingSince = updatingSince
		r.mu.Unlock()

		go func() {
			time.Sleep(5 * time.Minute)
			r.mu.Lock()
			defer r.mu.Unlock()
			if res, stillExists := r.resources[key]; stillExists {
				if res.Status == "Updating" && res.UpdatingSince.Equal(updatingSince) {
					res.Status = "Updated (Unverified)"
					res.UpdateAvailable = false
					res.PendingApproval = false
					res.UpdatingSince = time.Time{}
					slog.Info("⏳ Update timeout reached (via goroutine), marking as unverified", "resource", name, "namespace", namespace, "kind", kind)
				}
			}
		}()
	} else {
		r.mu.Unlock()
	}
}

func (r *Registry) ApproveUpdate(namespace, name, kind string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := r.getKey(namespace, name, kind)
	res, exists := r.resources[key]
	if !exists {
		return fmt.Errorf("resource not found")
	}
	if !res.RequiresApproval {
		return fmt.Errorf("resource does not require approval")
	}
	res.PendingApproval = false
	return nil
}

func (r *Registry) GetAllResources() []*ObservedResource {
	r.mu.RLock()
	defer r.mu.RUnlock()
	resources := make([]*ObservedResource, 0, len(r.resources))
	for _, res := range r.resources {
		resources = append(resources, res)
	}
	sort.Slice(resources, func(i, j int) bool {
		if resources[i].Namespace != resources[j].Namespace {
			return resources[i].Namespace < resources[j].Namespace
		}
		if resources[i].Kind != resources[j].Kind {
			return resources[i].Kind < resources[j].Kind
		}
		return resources[i].Name < resources[j].Name
	})
	return resources
}

func extractSHAFromImageID(imageID string) string {
	if len(imageID) == 0 {
		return ""
	}
	if atIdx := findChar(imageID, '@'); atIdx >= 0 {
		return imageID[atIdx+1:]
	}
	if shaIdx := findSubstring(imageID, "sha256:"); shaIdx >= 0 {
		return imageID[shaIdx:]
	}
	return ""
}

func findChar(s string, c rune) int {
	for i, ch := range s {
		if ch == c {
			return i
		}
	}
	return -1
}

func findSubstring(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

func GetRemoteDigest(imageName string) (string, error) {
	slog.Debug("Fetching remote digest", "image", imageName)
	imageRef, err := name.ParseReference(imageName)
	if err != nil {
		slog.Debug("Failed to parse reference", "image", imageName, "error", err)
		return "", err
	}

	// Create context with timeout to prevent blocking
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// remote.Head only fetches the headers (very fast/lightweight)
	// It automatically handles anonymous auth for public images
	slog.Debug("Calling remote.Head", "imageRef", imageRef.String())
	image, err := remote.Head(imageRef, remote.WithContext(ctx))
	if err != nil {
		slog.Debug("remote.Head failed", "imageRef", imageRef.String(), "error", err)
		return "", err
	}
	slog.Debug("Successfully fetched remote digest", "image", imageName, "digest", image.Digest.String())
	return image.Digest.String(), nil
}
