package main

type ObservedResource struct {
	Name        string
	Namespace   string
	Kind        string
	Annotations map[string]string
	Containers  map[string]string
}
