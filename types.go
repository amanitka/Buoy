package main

type WatchableResource struct {
	Name          string            `json:"name"`
	Namespace     string            `json:"namespace"`
	Kind          string            `json:"kind"`
	Annotations   map[string]string `json:"annotations"`
	ContainerData []ContainerInfo   `json:"containers"`
}

type ContainerInfo struct {
	Name        string `json:"name"`
	Image       string `json:"image"`
	ImageDigest string `json:"image_digest"`
}
