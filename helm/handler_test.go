package helm

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/aurora-capcompute/aurora-dispatchers/resolution"
	"github.com/aurora-capcompute/capcompute/dispatcher"
)

type mockClient struct {
	listCalls     int
	installCalls  int
	templateCalls int
}

func (m *mockClient) List(context.Context, ListRequest) (json.RawMessage, error) {
	m.listCalls++
	return json.RawMessage(`[{"name":"api"}]`), nil
}
func (*mockClient) Status(context.Context, StatusRequest) (json.RawMessage, error) {
	return json.RawMessage(`{"name":"api"}`), nil
}
func (*mockClient) GetValues(context.Context, GetValuesRequest) (json.RawMessage, error) {
	return json.RawMessage(`{"replicas":2}`), nil
}
func (m *mockClient) Install(context.Context, InstallRequest) (json.RawMessage, error) {
	m.installCalls++
	return json.RawMessage(`{"name":"api"}`), nil
}
func (*mockClient) Upgrade(context.Context, UpgradeRequest) (json.RawMessage, error) {
	return json.RawMessage(`{"name":"api"}`), nil
}
func (*mockClient) Rollback(context.Context, RollbackRequest) (string, error) {
	return "rolled back", nil
}
func (*mockClient) Uninstall(context.Context, UninstallRequest) (string, error) {
	return "uninstalled", nil
}
func (m *mockClient) Template(context.Context, TemplateRequest) (string, error) {
	m.templateCalls++
	return "kind: Deployment", nil
}

func TestReadOperationReturnsImmediately(t *testing.T) {
	client := &mockClient{}
	handler := NewHandler(client)
	handler.AddCapability("helm.list", Settings{})

	outcome, err := handler.DispatchCall(context.Background(), dispatcher.Call{
		Name: "helm.list",
		Args: json.RawMessage(`{}`),
	})
	if err != nil {
		t.Fatalf("dispatch list: %v", err)
	}
	if outcome.Kind() != dispatcher.OutcomeResult {
		t.Fatalf("list outcome = %s, want result", outcome.Kind())
	}
	if client.listCalls != 1 {
		t.Fatalf("list calls = %d, want 1", client.listCalls)
	}
}

func TestMutationYieldsUntilApproved(t *testing.T) {
	client := &mockClient{}
	handler := NewHandler(client)
	handler.AddCapability("helm.install", Settings{
		Namespaces: []string{"default"},
		Charts:     []string{"bitnami/*"},
	})
	call := dispatcher.Call{
		Name: "helm.install",
		Args: json.RawMessage(`{"release":"api","chart":"bitnami/nginx","namespace":"default"}`),
	}

	outcome, err := handler.DispatchCall(context.Background(), call)
	if err != nil {
		t.Fatalf("dispatch install: %v", err)
	}
	if outcome.Kind() != dispatcher.OutcomeYield {
		t.Fatalf("install outcome = %s, want yield", outcome.Kind())
	}
	if client.installCalls != 0 {
		t.Fatalf("install ran before approval")
	}

	ctx := resolution.WithContext(context.Background(), resolution.Resolution{Decision: resolution.Approved})
	outcome, err = handler.DispatchCall(ctx, call)
	if err != nil {
		t.Fatalf("dispatch approved install: %v", err)
	}
	if outcome.Kind() != dispatcher.OutcomeResult {
		t.Fatalf("approved install outcome = %s, want result", outcome.Kind())
	}
	if client.installCalls != 1 {
		t.Fatalf("install calls = %d, want 1", client.installCalls)
	}
}

func TestPoliciesRejectDisallowedScope(t *testing.T) {
	client := &mockClient{}
	disabled := false
	handler := NewHandler(client)
	handler.AddCapability("helm.template", Settings{
		Namespaces:      []string{"default"},
		Charts:          []string{"internal/*"},
		RequireApproval: &disabled,
	})

	for name, args := range map[string]string{
		"namespace": `{"release":"api","chart":"internal/api","namespace":"production"}`,
		"chart":     `{"release":"api","chart":"bitnami/nginx","namespace":"default"}`,
	} {
		t.Run(name, func(t *testing.T) {
			outcome, err := handler.DispatchCall(context.Background(), dispatcher.Call{
				Name: "helm.template",
				Args: json.RawMessage(args),
			})
			if err != nil {
				t.Fatalf("dispatch template: %v", err)
			}
			if outcome.Kind() != dispatcher.OutcomeFailed {
				t.Fatalf("template outcome = %s, want failed", outcome.Kind())
			}
		})
	}
	if client.templateCalls != 0 {
		t.Fatalf("template calls = %d, want 0", client.templateCalls)
	}
}
