package helm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/aurora-capcompute/aurora-dispatchers/builtin"
	"github.com/aurora-capcompute/aurora-dispatchers/registry"
	"github.com/aurora-capcompute/capcompute/dispatcher"
)

// ToolType is the manifest `type` for a Helm tool.
const ToolType = "core.helm"

type Registration struct{}

func (Registration) Matches(toolType string) bool { return toolType == ToolType }

func (Registration) Normalize(_ string, raw json.RawMessage) (json.RawMessage, error) {
	var settings Settings
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &settings); err != nil {
			return nil, err
		}
	}
	settings.HelmBinary = strings.TrimSpace(settings.HelmBinary)
	if settings.HelmBinary == "" {
		settings.HelmBinary = "helm"
	}
	settings.Kubeconfig = strings.TrimSpace(settings.Kubeconfig)
	settings.Context = strings.TrimSpace(settings.Context)
	if len(settings.Permissions) == 0 {
		return nil, fmt.Errorf("permissions must contain at least one {resource, verb, namespace}")
	}
	for i := range settings.Permissions {
		settings.Permissions[i].Resource = strings.TrimSpace(settings.Permissions[i].Resource)
		settings.Permissions[i].Verb = strings.ToLower(strings.TrimSpace(settings.Permissions[i].Verb))
		settings.Permissions[i].Namespace = strings.TrimSpace(settings.Permissions[i].Namespace)
	}
	return json.Marshal(settings)
}

func (Registration) Configure(
	_ context.Context,
	name string,
	raw json.RawMessage,
	_ registry.Services,
	config *builtin.Config,
) error {
	normalized, err := (Registration{}).Normalize(name, raw)
	if err != nil {
		return err
	}
	var settings Settings
	if err := json.Unmarshal(normalized, &settings); err != nil {
		return err
	}
	handler, err := findOrCreateHandler(config, settings)
	if err != nil {
		return err
	}
	handler.AddCapability(name, settings)
	config.Capabilities = append(config.Capabilities, capabilityFor(name, settings))
	return nil
}

func findOrCreateHandler(config *builtin.Config, settings Settings) (*Handler, error) {
	connection := connectionSettings{
		binary:     settings.HelmBinary,
		kubeconfig: settings.Kubeconfig,
		context:    settings.Context,
	}
	for _, existing := range config.Handlers {
		if handler, ok := existing.(*Handler); ok {
			if handler.connection != connection {
				return nil, fmt.Errorf("helm capabilities must use the same helm_binary, kubeconfig, and context")
			}
			return handler, nil
		}
	}
	client, err := NewClient(settings.HelmBinary, settings.Kubeconfig, settings.Context)
	if err != nil {
		client = failedClient{err: err}
	}
	handler := NewHandler(client)
	handler.connection = connection
	config.Handlers = append(config.Handlers, handler)
	return handler, nil
}

type failedClient struct{ err error }

func (c failedClient) List(context.Context, ListRequest) (json.RawMessage, error) {
	return nil, c.err
}
func (c failedClient) Status(context.Context, StatusRequest) (json.RawMessage, error) {
	return nil, c.err
}
func (c failedClient) GetValues(context.Context, GetValuesRequest) (json.RawMessage, error) {
	return nil, c.err
}
func (c failedClient) Install(context.Context, InstallRequest) (json.RawMessage, error) {
	return nil, c.err
}
func (c failedClient) Upgrade(context.Context, UpgradeRequest) (json.RawMessage, error) {
	return nil, c.err
}
func (c failedClient) Rollback(context.Context, RollbackRequest) (string, error) {
	return "", c.err
}
func (c failedClient) Uninstall(context.Context, UninstallRequest) (string, error) {
	return "", c.err
}
func (c failedClient) Template(context.Context, TemplateRequest) (string, error) {
	return "", c.err
}

// capabilityFor publishes one tool capability named by the local tool name. The
// input schema is a discriminated union over the verbs the permissions grant.
func capabilityFor(name string, settings Settings) dispatcher.Capability {
	verbs := newPermissionPolicy(settings.Permissions).permittedVerbs()
	branches := make([]json.RawMessage, 0, len(verbs))
	for _, v := range verbs {
		branches = append(branches, verbSchemas[v])
	}
	schema := json.RawMessage(`{"type":"object"}`)
	if len(branches) > 0 {
		oneOf, _ := json.Marshal(map[string]any{"oneOf": branches})
		schema = oneOf
	}
	approval := ""
	if settings.RequireApproval != nil && *settings.RequireApproval {
		approval = " All operations require human approval."
	}
	return dispatcher.Capability{
		Name:        name,
		Description: fmt.Sprintf("Helm operations selected by `verb` (%s). Allowed: %s.%s", strings.Join(verbs, ", "), describePermissions(settings.Permissions), approval),
		InputSchema: schema,
	}
}

func describePermissions(perms []Permission) string {
	parts := make([]string, 0, len(perms))
	for _, p := range perms {
		resource, verb, ns := p.Resource, p.Verb, p.Namespace
		if resource == "" {
			resource = "*"
		}
		if verb == "" {
			verb = "*"
		}
		if ns == "" {
			ns = "*"
		}
		parts = append(parts, fmt.Sprintf("%s %s in %s", verb, resource, ns))
	}
	return strings.Join(parts, "; ")
}

// verbSchemas are the per-verb branches of the union input schema. Each carries
// a `verb` discriminator const plus that verb's operation fields.
var verbSchemas = map[string]json.RawMessage{
	"list":       json.RawMessage(`{"type":"object","properties":{"verb":{"const":"list"},"namespace":{"type":"string"},"filter":{"type":"string"}},"required":["verb"],"additionalProperties":false}`),
	"status":     json.RawMessage(`{"type":"object","properties":{"verb":{"const":"status"},"release":{"type":"string"},"namespace":{"type":"string"}},"required":["verb","release"],"additionalProperties":false}`),
	"get_values": json.RawMessage(`{"type":"object","properties":{"verb":{"const":"get_values"},"release":{"type":"string"},"namespace":{"type":"string"},"all":{"type":"boolean"}},"required":["verb","release"],"additionalProperties":false}`),
	"install":    json.RawMessage(`{"type":"object","properties":{"verb":{"const":"install"},"release":{"type":"string"},"chart":{"type":"string"},"namespace":{"type":"string"},"version":{"type":"string"},"values":{"type":"object"},"create_namespace":{"type":"boolean"},"wait":{"type":"boolean"},"timeout":{"type":"string"}},"required":["verb","release","chart"],"additionalProperties":false}`),
	"upgrade":    json.RawMessage(`{"type":"object","properties":{"verb":{"const":"upgrade"},"release":{"type":"string"},"chart":{"type":"string"},"namespace":{"type":"string"},"version":{"type":"string"},"values":{"type":"object"},"install":{"type":"boolean"},"wait":{"type":"boolean"},"timeout":{"type":"string"}},"required":["verb","release","chart"],"additionalProperties":false}`),
	"rollback":   json.RawMessage(`{"type":"object","properties":{"verb":{"const":"rollback"},"release":{"type":"string"},"revision":{"type":"integer","minimum":1},"namespace":{"type":"string"},"wait":{"type":"boolean"},"timeout":{"type":"string"}},"required":["verb","release","revision"],"additionalProperties":false}`),
	"uninstall":  json.RawMessage(`{"type":"object","properties":{"verb":{"const":"uninstall"},"release":{"type":"string"},"namespace":{"type":"string"},"keep_history":{"type":"boolean"},"wait":{"type":"boolean"},"timeout":{"type":"string"}},"required":["verb","release"],"additionalProperties":false}`),
	"template":   json.RawMessage(`{"type":"object","properties":{"verb":{"const":"template"},"release":{"type":"string"},"chart":{"type":"string"},"namespace":{"type":"string"},"version":{"type":"string"},"values":{"type":"object"},"include_crds":{"type":"boolean"}},"required":["verb","release","chart"],"additionalProperties":false}`),
}
