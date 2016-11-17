/*
Copyright 2016 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package image

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/golang/glog"
	"github.com/kubernetes-incubator/rktlet/rktlet/cli"
	"github.com/kubernetes-incubator/rktlet/rktlet/util"

	appcschema "github.com/appc/spec/schema"
	context "golang.org/x/net/context"
	"k8s.io/kubernetes/pkg/kubelet/api/v1alpha1/runtime"
)

// TODO(tmrts): Move these errors to the container API for code re-use.
var (
	ErrImageNotFound = errors.New("rkt: image not found")
)

// var _ kubeletApi.ImageManagerService = (*ImageStore)(nil)

// ImageStore supports CRUD operations for images.
type ImageStore struct {
	cli.CLI
	requestTimeout time.Duration
}

// TODO(tmrts): fill the image store configuration fields.
type ImageStoreConfig struct {
	CLI            cli.CLI
	RequestTimeout time.Duration
}

// NewImageStore creates an image storage that allows CRUD operations for images.
func NewImageStore(cfg ImageStoreConfig) runtime.ImageServiceServer {
	return &ImageStore{cfg.CLI, cfg.RequestTimeout}
}

// Remove removes the image from the image store.
func (s *ImageStore) RemoveImage(ctx context.Context, req *runtime.RemoveImageRequest) (*runtime.RemoveImageResponse, error) {
	img, err := s.ImageStatus(ctx, &runtime.ImageStatusRequest{Image: req.Image})
	if err != nil {
		return nil, err
	}

	if _, err := s.RunCommand("image", "rm", *img.Image.Id); err != nil {
		return nil, fmt.Errorf("failed to remove the image: %v", err)
	}

	return &runtime.RemoveImageResponse{}, nil
}

// ImageStatus returns the status of the image.
// TODO(euank): rkt should support listing a single image so this is more
// efficient
func (s *ImageStore) ImageStatus(ctx context.Context, req *runtime.ImageStatusRequest) (*runtime.ImageStatusResponse, error) {
	images, err := s.ListImages(ctx, &runtime.ListImagesRequest{})
	if err != nil {
		return nil, err
	}

	reqImg := req.GetImage().GetImage()
	// TODO this should be done in kubelet (see comment on ApplyDefaultImageTag)
	reqImg, err = util.ApplyDefaultImageTag(reqImg)
	if err != nil {
		return nil, err
	}

	for _, img := range images.Images {
		for _, name := range img.RepoTags {
			if name == reqImg {
				return &runtime.ImageStatusResponse{Image: img}, nil
			}
		}
	}

	return nil, fmt.Errorf("couldn't find image %q", *req.Image.Image)
}

// TODO this should be exported by rkt upstream. This is a copy of https://github.com/coreos/rkt/blob/v1.19.0/rkt/image_list.go#L81-L87
// After https://github.com/coreos/rkt/pull/3383, the exported type can be used.
type ImageListEntry struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	ImportTime string `json:"importtime"`
	LastUsed   string `json:"lastused"`
	Size       string `json:"size"`
}

// ListImages lists images in the store
func (s *ImageStore) ListImages(ctx context.Context, req *runtime.ListImagesRequest) (*runtime.ListImagesResponse, error) {
	list, err := s.RunCommand("image", "list",
		"--full",
		"--format=json",
		"--sort=importtime",
	)
	if err != nil {
		return nil, fmt.Errorf("couldn't list images: %v", err)
	}

	listEntries := []ImageListEntry{}

	err = json.Unmarshal([]byte(list[0]), &listEntries)
	if err != nil {
		return nil, fmt.Errorf("could not unmarshal images into expected format: %v", err)
	}

	images := make([]*runtime.Image, 0, len(list))
	for _, img := range listEntries {
		img := img

		realName := s.getImageRealName(img.ID)
		if realName == "" {
			realName = img.Name
		}
		sz, err := strconv.ParseUint(img.Size, 10, 64)
		if err != nil {
			glog.Warningf("could not parse image size: %v", err)
			sz = 0
		}

		image := &runtime.Image{
			Id:          &img.ID,
			RepoTags:    []string{img.Name},
			RepoDigests: []string{img.ID},
			Size_:       &sz,
		}

		if passFilter(image, req.Filter) {
			images = append(images, image)
		}
	}

	return &runtime.ListImagesResponse{Images: images}, nil
}

func (s *ImageStore) getImageRealName(id string) string {
	imgManifest, err := s.RunCommand("image", "cat-manifest", id)
	var manifest appcschema.ImageManifest

	err = json.Unmarshal([]byte(strings.Join(imgManifest, "")), &manifest)
	if err != nil {
		glog.Warningf("unable to unmarshal image %q manifest into appc: %v", id, err)
		return ""
	}

	originalName, ok := manifest.GetAnnotation("appc.io/docker/originalname")
	if !ok {
		glog.Warningf("image %q does not have originalname annotation", id)
		return ""
	}
	return originalName
}

// PullImage pulls an image into the store
func (s *ImageStore) PullImage(ctx context.Context, req *runtime.PullImageRequest) (*runtime.PullImageResponse, error) {
	// TODO auth
	output, err := s.RunCommand("image", "fetch", "--no-store=true", "--insecure-options=image,ondisk", "--full=true", "docker://"+*req.Image.Image)

	if err != nil {
		return nil, fmt.Errorf("unable to fetch image: %v", err)
	}
	if len(output) < 1 {
		return nil, fmt.Errorf("malformed fetch image response; must include image id: %v", output)
	}

	return &runtime.PullImageResponse{}, nil
}

// passFilter returns whether the target image satisfies the filter.
func passFilter(image *runtime.Image, filter *runtime.ImageFilter) bool {
	if filter == nil {
		return true
	}

	if filter.Image == nil {
		return true
	}

	imageName := filter.Image.GetImage()
	for _, name := range image.RepoTags {
		if imageName == name {
			return true
		}
	}
	return false
}
