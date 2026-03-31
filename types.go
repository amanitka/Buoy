package main

import "time"

type ObservedResource struct {
	Name             string
	Namespace        string
	Kind             string
	Annotations      map[string]string
	Containers       map[string]string
	Schedule         string
	RequiresApproval bool
	UpdateAvailable  bool
	PendingApproval  bool
	LastChecked      time.Time
	RemoteSHA        map[string]string
	LiveSHA          map[string]string
}
