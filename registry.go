package main

import (
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

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
