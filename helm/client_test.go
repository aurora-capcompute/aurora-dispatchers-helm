package helm

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
)

type recordedRun struct {
	stdin []byte
	args  []string
}

type fakeRunner struct {
	runs   []recordedRun
	stdout []byte
	stderr []byte
	err    error
}

func (r *fakeRunner) Run(_ context.Context, stdin []byte, args ...string) ([]byte, []byte, error) {
	r.runs = append(r.runs, recordedRun{
		stdin: append([]byte(nil), stdin...),
		args:  append([]string(nil), args...),
	})
	return r.stdout, r.stderr, r.err
}

func TestInstallBuildsSafeCommandAndStreamsValues(t *testing.T) {
	runner := &fakeRunner{stdout: []byte(`{"name":"api"}`)}
	client := newClient(runner, "/tmp/kube config", "dev")
	values := json.RawMessage(`{"image":{"tag":"1.2.3"}}`)

	_, err := client.Install(context.Background(), InstallRequest{
		Release:         "api",
		Chart:           "internal/api",
		Namespace:       "default",
		Version:         "1.0.0",
		Values:          values,
		CreateNamespace: true,
		Wait:            true,
		Timeout:         "5m",
	})
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	want := []string{
		"install", "api", "internal/api", "--output", "json",
		"--namespace", "default", "--version", "1.0.0", "--values", "-",
		"--create-namespace", "--wait", "--timeout", "5m",
		"--kubeconfig", "/tmp/kube config", "--kube-context", "dev",
	}
	if !reflect.DeepEqual(runner.runs[0].args, want) {
		t.Fatalf("args = %#v, want %#v", runner.runs[0].args, want)
	}
	if !reflect.DeepEqual(runner.runs[0].stdin, []byte(values)) {
		t.Fatalf("stdin = %s, want %s", runner.runs[0].stdin, values)
	}
}

func TestRunnerErrorIncludesHelmStderr(t *testing.T) {
	runner := &fakeRunner{stderr: []byte("release not found\n"), err: errors.New("exit status 1")}
	client := newClient(runner, "", "")

	_, err := client.Status(context.Background(), StatusRequest{Release: "missing"})
	if err == nil || err.Error() != "helm status: release not found" {
		t.Fatalf("error = %v", err)
	}
}

func TestInvalidTimeoutDoesNotRunHelm(t *testing.T) {
	runner := &fakeRunner{stdout: []byte(`{}`)}
	client := newClient(runner, "", "")

	_, err := client.Upgrade(context.Background(), UpgradeRequest{
		Release: "api",
		Chart:   "internal/api",
		Timeout: "eventually",
	})
	if err == nil {
		t.Fatal("expected invalid timeout error")
	}
	if len(runner.runs) != 0 {
		t.Fatalf("helm ran %d times", len(runner.runs))
	}
}
