package helm

import (
	"context"
	"encoding/json"
	"testing"

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

// handlerWithMock builds a Handler holding one helm tool bound to a mock client.
func handlerWithMock(name string, client Client, settings Settings) *Handler {
	return &Handler{
		client: client,
		capabilities: map[string]capabilityConfig{
			name: {
				policy:          newPermissionPolicy(settings.Permissions),
				requireApproval: settings.RequireApproval,
			},
		},
	}
}

var anyVerb = Settings{Permissions: []Permission{{Verb: "*"}}}

func TestReadOperationReturnsImmediately(t *testing.T) {
	client := &mockClient{}
	handler := handlerWithMock("helmTool", client, anyVerb)

	outcome, err := handler.DispatchCall(context.Background(), dispatcher.Call{
		Name: "helmTool",
		Args: json.RawMessage(`{"verb":"list"}`),
	}, dispatcher.Authorization{})
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
	handler := handlerWithMock("helmTool", client, Settings{
		Permissions: []Permission{{Verb: "install", Resource: "bitnami/*", Namespace: "default"}},
	})
	call := dispatcher.Call{
		Name: "helmTool",
		Args: json.RawMessage(`{"verb":"install","release":"api","chart":"bitnami/nginx","namespace":"default"}`),
	}

	outcome, err := handler.DispatchCall(context.Background(), call, dispatcher.Authorization{})
	if err != nil {
		t.Fatalf("dispatch install: %v", err)
	}
	if outcome.Kind() != dispatcher.OutcomeYield {
		t.Fatalf("install outcome = %s, want yield", outcome.Kind())
	}
	if client.installCalls != 0 {
		t.Fatalf("install ran before approval")
	}

	outcome, err = handler.DispatchCall(context.Background(), call, dispatcher.Authorization{Decision: dispatcher.Approved})
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

func TestPermissionsRejectDisallowedScope(t *testing.T) {
	client := &mockClient{}
	disabled := false
	handler := handlerWithMock("helmTool", client, Settings{
		Permissions:     []Permission{{Verb: "template", Resource: "internal/*", Namespace: "default"}},
		RequireApproval: &disabled,
	})

	for name, args := range map[string]string{
		"namespace": `{"verb":"template","release":"api","chart":"internal/api","namespace":"production"}`,
		"chart":     `{"verb":"template","release":"api","chart":"bitnami/nginx","namespace":"default"}`,
	} {
		t.Run(name, func(t *testing.T) {
			outcome, err := handler.DispatchCall(context.Background(), dispatcher.Call{
				Name: "helmTool",
				Args: json.RawMessage(args),
			}, dispatcher.Authorization{})
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
