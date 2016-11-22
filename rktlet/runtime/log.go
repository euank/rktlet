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

package runtime

import (
	"fmt"
	"path/filepath"
	"strings"

	"golang.org/x/net/context"

	runtimeapi "k8s.io/kubernetes/pkg/kubelet/api/v1alpha1/runtime"
)

const loggingHelperImage = "quay.io/coreos/rktlet-journal2cri:0.0.1"
const loggingAppName = "journal2cri-rktletinternal"

func (r *RktRuntime) initializeLoggingAppImage(ctx context.Context) error {
	imageName := loggingHelperImage
	_, err := r.imageStore.PullImage(ctx, &runtimeapi.PullImageRequest{
		Image: &runtimeapi.ImageSpec{
			Image: &imageName,
		},
	})
	return err
}

// addInternalLoggingApp adds the helper app for converting journald logs for this pod to cri logs
func (r *RktRuntime) addInternalLoggingApp(rktUUID string, criLogDir string) error {
	if criLogDir == "" {
		return fmt.Errorf("unable to start logging: no cri log directory provided")
	}

	imageHash, err := r.getImageHash(loggingHelperImage)
	if err != nil {
		return err
	}

	rktJournalDir := filepath.Join("var", "log", "journal", strings.Replace(rktUUID, "-", "", -1))

	cmd := []string{"app", "add", rktUUID, imageHash}

	cmd = append(cmd, "--name=journal2cri-"+loggingAppName)
	cmd = append(cmd, fmt.Sprintf("--mnt-volume=name=journal,kind=host,source=%s,target=/journal,readOnly=true"), rktJournalDir)
	cmd = append(cmd, fmt.Sprintf("--mnt-volume=name=cri,kind=host,source=%s,target=/cri,readOnly=false"), criLogDir)

	if _, err := r.RunCommand(cmd[0], cmd[1:]...); err != nil {
		return err
	}

	if _, err := r.RunCommand("app", "start", rktUUID, "--app="+loggingAppName); err != nil {
		return err
	}
	return nil
}
