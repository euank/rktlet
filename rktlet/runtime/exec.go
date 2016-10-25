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
	"bytes"
	"errors"
	"io"
	"io/ioutil"
	"os/exec"

	"github.com/kubernetes-incubator/rktlet/rktlet/cli"
	"golang.org/x/net/context"

	runtimeapi "k8s.io/kubernetes/pkg/kubelet/api/v1alpha1/runtime"
	"k8s.io/kubernetes/pkg/kubelet/server/streaming"
	"k8s.io/kubernetes/pkg/util/term"
)

func (r *RktRuntime) Attach(ctx context.Context, req *runtimeapi.AttachRequest) (*runtimeapi.AttachResponse, error) {
	// TODO, the second parameter here needs to be retrieved from the
	// `ContainerConfig` associated with the req.ContainerID
	return r.streamServer.GetAttach(req, true)
}

func (r *RktRuntime) Exec(ctx context.Context, req *runtimeapi.ExecRequest) (*runtimeapi.ExecResponse, error) {
	return r.streamServer.GetExec(req)
}

type nopWriteCloser bytes.Buffer

func (n nopWriteCloser) Bytes() []byte {
	return n.Bytes()
}

func (n nopWriteCloser) Write(p []byte) (int, error) {
	return n.Write(p)
}

func (nopWriteCloser) Close() error {
	return nil
}

func (r *RktRuntime) ExecSync(ctx context.Context, req *runtimeapi.ExecSyncRequest) (*runtimeapi.ExecSyncResponse, error) {
	nopStdin := ioutil.NopCloser(bytes.NewReader([]byte{}))
	var stdout, stderr nopWriteCloser
	err := r.execShim.Exec(req.GetContainerId(), req.GetCmd(), nopStdin, stdout, stderr, false, make(chan term.Size))
	if err != nil {
		return nil, err
	}

	var exitCode int32 = 0 // TODO
	return &runtimeapi.ExecSyncResponse{
		ExitCode: &exitCode,
		Stderr:   stderr.Bytes(),
		Stdout:   stdout.Bytes(),
	}, nil
}

func (r *RktRuntime) PortForward(ctx context.Context, req *runtimeapi.PortForwardRequest) (*runtimeapi.PortForwardResponse, error) {
	return r.streamServer.GetPortForward(req)
}

type execShim struct {
	cli cli.CLI
}

var _ streaming.Runtime = &execShim{}

func NewExecShim(cli cli.CLI) *execShim {
	return &execShim{cli: cli}
}

func (es *execShim) Attach(containerID string, in io.Reader, out, err io.WriteCloser, resize <-chan term.Size) error {
	return errors.New("TODO")
}

func (es *execShim) Exec(containerID string, cmd []string, in io.Reader, out, errOut io.WriteCloser, tty bool, resize <-chan term.Size) error {
	uuid, appName, err := parseContainerID(containerID)
	if err != nil {
		return err
	}

	// Since "k8s.io/kubernetes/pkg/util/exec.Cmd" doesn't include
	// StdinPipe(), StdoutPipe() and StderrPipe() in the interface,
	// so we have to use the "Cmd" under "os/exec" package.
	// TODO(yifan): Patch upstream to include SetStderr() in the interface.
	cmdList := []string{"app", "exec", "--app=" + appName, uuid}
	cmdList = append(cmdList, cmd...)
	rktCommand := es.cli.Command(cmdList[0], cmdList[1:]...)
	execCmd := exec.Command(rktCommand[0], rktCommand[1:]...)

	// At most one error will happen in each of the following goroutines.
	errCh := make(chan error, 4)
	done := make(chan struct{})

	go streamStdin(execCmd, in, errCh)
	go streamStdout(execCmd, out, errCh)
	go streamStderr(execCmd, errOut, errCh)
	go run(execCmd, errCh, done)

	select {
	case err := <-errCh:
		return err
	case <-done:
		return nil
	}
}

func (es *execShim) PortForward(sandboxID string, port int32, stream io.ReadWriteCloser) error {
	return errors.New("TODO")
}

func streamStdin(cmd *exec.Cmd, in io.Reader, errCh chan error) {
	stdin, err := cmd.StdinPipe()
	if err != nil {
		errCh <- err
		return
	}
	_, err = io.Copy(stdin, in)
	if err != nil {
		errCh <- err
	}
}

func streamStdout(cmd *exec.Cmd, out io.WriteCloser, errCh chan error) {
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		errCh <- err
		return
	}
	_, err = io.Copy(out, stdout)
	if err != nil {
		errCh <- err
	}
}

func streamStderr(cmd *exec.Cmd, out io.WriteCloser, errCh chan error) {
	stderr, err := cmd.StderrPipe()
	if err != nil {
		errCh <- err
		return
	}

	_, err = io.Copy(out, stderr)
	if err != nil {
		errCh <- err
	}
}

func run(cmd *exec.Cmd, errCh chan error, done chan struct{}) {
	if err := cmd.Start(); err != nil {
		errCh <- err
		return
	}
	if err := cmd.Wait(); err != nil {
		errCh <- err
		return
	}
	close(done)
	return
}
