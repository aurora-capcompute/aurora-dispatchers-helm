package helm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type Client interface {
	List(context.Context, ListRequest) (json.RawMessage, error)
	Status(context.Context, StatusRequest) (json.RawMessage, error)
	GetValues(context.Context, GetValuesRequest) (json.RawMessage, error)
	Install(context.Context, InstallRequest) (json.RawMessage, error)
	Upgrade(context.Context, UpgradeRequest) (json.RawMessage, error)
	Rollback(context.Context, RollbackRequest) (string, error)
	Uninstall(context.Context, UninstallRequest) (string, error)
	Template(context.Context, TemplateRequest) (string, error)
}

type commandRunner interface {
	Run(context.Context, []byte, ...string) ([]byte, []byte, error)
}

type execRunner struct {
	binary string
}

func (r execRunner) Run(ctx context.Context, stdin []byte, args ...string) ([]byte, []byte, error) {
	cmd := exec.CommandContext(ctx, r.binary, args...)
	if len(stdin) > 0 {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.Bytes(), stderr.Bytes(), err
}

type client struct {
	runner     commandRunner
	kubeconfig string
	context    string
}

func NewClient(binary, kubeconfig, kubeContext string) (Client, error) {
	if binary = strings.TrimSpace(binary); binary == "" {
		binary = "helm"
	}
	path, err := exec.LookPath(binary)
	if err != nil {
		return nil, fmt.Errorf("find helm binary %q: %w", binary, err)
	}
	return newClient(execRunner{binary: path}, kubeconfig, kubeContext), nil
}

func newClient(runner commandRunner, kubeconfig, kubeContext string) *client {
	return &client{runner: runner, kubeconfig: kubeconfig, context: kubeContext}
}

func (c *client) List(ctx context.Context, req ListRequest) (json.RawMessage, error) {
	args := []string{"list", "--output", "json"}
	if req.Namespace != "" {
		args = append(args, "--namespace", req.Namespace)
	} else {
		args = append(args, "--all-namespaces")
	}
	if req.Filter != "" {
		args = append(args, "--filter", req.Filter)
	}
	return c.runJSON(ctx, nil, args...)
}

func (c *client) Status(ctx context.Context, req StatusRequest) (json.RawMessage, error) {
	args := []string{"status", req.Release, "--output", "json"}
	args = appendNamespace(args, req.Namespace)
	return c.runJSON(ctx, nil, args...)
}

func (c *client) GetValues(ctx context.Context, req GetValuesRequest) (json.RawMessage, error) {
	args := []string{"get", "values", req.Release, "--output", "json"}
	args = appendNamespace(args, req.Namespace)
	if req.All {
		args = append(args, "--all")
	}
	return c.runJSON(ctx, nil, args...)
}

func (c *client) Install(ctx context.Context, req InstallRequest) (json.RawMessage, error) {
	args := []string{"install", req.Release, req.Chart, "--output", "json"}
	args = appendNamespace(args, req.Namespace)
	args = appendVersion(args, req.Version)
	args = appendValues(args, req.Values)
	if req.CreateNamespace {
		args = append(args, "--create-namespace")
	}
	args, err := appendExecutionFlags(args, req.Wait, req.Timeout)
	if err != nil {
		return nil, err
	}
	return c.runJSON(ctx, req.Values, args...)
}

func (c *client) Upgrade(ctx context.Context, req UpgradeRequest) (json.RawMessage, error) {
	args := []string{"upgrade", req.Release, req.Chart, "--output", "json"}
	args = appendNamespace(args, req.Namespace)
	args = appendVersion(args, req.Version)
	args = appendValues(args, req.Values)
	if req.Install {
		args = append(args, "--install")
	}
	args, err := appendExecutionFlags(args, req.Wait, req.Timeout)
	if err != nil {
		return nil, err
	}
	return c.runJSON(ctx, req.Values, args...)
}

func (c *client) Rollback(ctx context.Context, req RollbackRequest) (string, error) {
	args := []string{"rollback", req.Release, strconv.Itoa(req.Revision)}
	args = appendNamespace(args, req.Namespace)
	var err error
	args, err = appendExecutionFlags(args, req.Wait, req.Timeout)
	if err != nil {
		return "", err
	}
	return c.runText(ctx, nil, args...)
}

func (c *client) Uninstall(ctx context.Context, req UninstallRequest) (string, error) {
	args := []string{"uninstall", req.Release}
	args = appendNamespace(args, req.Namespace)
	if req.KeepHistory {
		args = append(args, "--keep-history")
	}
	var err error
	args, err = appendExecutionFlags(args, req.Wait, req.Timeout)
	if err != nil {
		return "", err
	}
	return c.runText(ctx, nil, args...)
}

func (c *client) Template(ctx context.Context, req TemplateRequest) (string, error) {
	args := []string{"template", req.Release, req.Chart}
	args = appendNamespace(args, req.Namespace)
	args = appendVersion(args, req.Version)
	args = appendValues(args, req.Values)
	if req.IncludeCRDs {
		args = append(args, "--include-crds")
	}
	return c.runText(ctx, req.Values, args...)
}

func (c *client) runJSON(ctx context.Context, stdin []byte, args ...string) (json.RawMessage, error) {
	stdout, err := c.run(ctx, stdin, args...)
	if err != nil {
		return nil, err
	}
	if !json.Valid(stdout) {
		return nil, fmt.Errorf("helm returned invalid JSON")
	}
	return json.RawMessage(append([]byte(nil), stdout...)), nil
}

func (c *client) runText(ctx context.Context, stdin []byte, args ...string) (string, error) {
	stdout, err := c.run(ctx, stdin, args...)
	return string(stdout), err
}

func (c *client) run(ctx context.Context, stdin []byte, args ...string) ([]byte, error) {
	args = append(args, c.connectionArgs()...)
	stdout, stderr, err := c.runner.Run(ctx, stdin, args...)
	if err == nil {
		return stdout, nil
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	message := strings.TrimSpace(string(stderr))
	if message == "" {
		message = err.Error()
	}
	return nil, fmt.Errorf("helm %s: %s", args[0], message)
}

func (c *client) connectionArgs() []string {
	var args []string
	if c.kubeconfig != "" {
		args = append(args, "--kubeconfig", c.kubeconfig)
	}
	if c.context != "" {
		args = append(args, "--kube-context", c.context)
	}
	return args
}

func appendNamespace(args []string, namespace string) []string {
	if namespace != "" {
		args = append(args, "--namespace", namespace)
	}
	return args
}

func appendVersion(args []string, version string) []string {
	if version != "" {
		args = append(args, "--version", version)
	}
	return args
}

func appendValues(args []string, values json.RawMessage) []string {
	if len(values) > 0 {
		args = append(args, "--values", "-")
	}
	return args
}

func appendExecutionFlags(args []string, wait bool, timeout string) ([]string, error) {
	if wait {
		args = append(args, "--wait")
	}
	if timeout != "" {
		if _, err := time.ParseDuration(timeout); err != nil {
			return nil, fmt.Errorf("invalid timeout %q: %w", timeout, err)
		}
		args = append(args, "--timeout", timeout)
	}
	return args, nil
}
