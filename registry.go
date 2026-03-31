package main

import (
	"fmt"
	"log/slog"
	"sync"

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

func (r *Registry) UpdateContainer(namespace, name, kind, containerName, imageID string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := r.getKey(namespace, name, kind)
	res, exists := r.resources[key]
	if !exists {
		return false
	}
	if res.Containers == nil {
		res.Containers = make(map[string]string)
	}
	oldID, hadOld := res.Containers[containerName]
	if hadOld && oldID == imageID {
		return false
	}
	res.Containers[containerName] = imageID
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
		"sha", imageID,
	)
}

func GetRemoteDigest(imageName string) (string, error) {
	imageRef, err := name.ParseReference(imageName)
	if err != nil {
		return "", err
	}
	// remote.Head only fetches the headers (very fast/lightweight)
	// It automatically handles anonymous auth for public images
	image, err := remote.Head(imageRef)
	if err != nil {
		return "", err
	}
	return image.Digest.String(), nil
}
