package main

import (
	"sort"
	"time"
)

type ObservedResource struct {
	Name              string
	Namespace         string
	Kind              string
	Annotations       map[string]string
	Containers        map[string]string
	Schedule          string
	RequiresApproval  bool
	UpdateAvailable   bool
	PendingApproval   bool
	LastChecked       time.Time
	RemoteSHA         map[string]string
	LiveSHA           map[string]string
	Status            string
	LatestPodCreation time.Time
}

func (r *ObservedResource) SortedContainerNames() []string {
	if r == nil || len(r.Containers) == 0 {
		return nil
	}

	names := make([]string, 0, len(r.Containers))
	for name := range r.Containers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
