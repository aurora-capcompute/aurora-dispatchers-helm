package helm

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"aurora-dispatchers/builtin"
	"aurora-dispatchers/registry"
	"capcompute/dispatcher"
)

var validOperations = map[string]struct{}{
	"helm.list":       {},
	"helm.status":     {},
	"helm.get_values": {},
	"helm.install":    {},
	"helm.upgrade":    {},
	"helm.rollback":   {},
	"helm.uninstall":  {},
	"helm.template":   {},
}

type Registration struct{}

func (Registration) Matches(name string) bool {
	_, ok := validOperations[name]
	return ok
}

func (Registration) Normalize(name string, raw json.RawMessage) (json.RawMessage, error) {
	if _, ok := validOperations[name]; !ok {
		return nil, fmt.Errorf("unsupported helm operation %q", name)
	}
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
	settings.Namespaces = cleanList(settings.Namespaces)
	settings.Charts = cleanList(settings.Charts)
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

func (Registration) IsSubset(name string, parent, child json.RawMessage) error {
	var parentSettings, childSettings Settings
	if err := json.Unmarshal(parent, &parentSettings); err != nil {
		return fmt.Errorf("decode parent settings: %w", err)
	}
	if err := json.Unmarshal(child, &childSettings); err != nil {
		return fmt.Errorf("decode child settings: %w", err)
	}
	if len(parentSettings.Namespaces) > 0 {
		allowed := make(map[string]struct{}, len(parentSettings.Namespaces))
		for _, ns := range parentSettings.Namespaces {
			allowed[ns] = struct{}{}
		}
		for _, ns := range childSettings.Namespaces {
			if _, ok := allowed[ns]; !ok {
				return fmt.Errorf("child namespace %q is not in parent's allowed namespaces", ns)
			}
		}
		if len(childSettings.Namespaces) == 0 {
			return fmt.Errorf("child must specify namespaces when parent restricts them")
		}
	}
	if len(parentSettings.Charts) > 0 {
		for _, childChart := range childSettings.Charts {
			if !chartAllowed(childChart, parentSettings.Charts) {
				return fmt.Errorf("child chart %q is not in parent's allowed charts", childChart)
			}
		}
		if len(childSettings.Charts) == 0 {
			return fmt.Errorf("child must specify charts when parent restricts them")
		}
	}
	return nil
}

func chartAllowed(chart string, allowed []string) bool {
	for _, a := range allowed {
		if a == chart {
			return true
		}
		if strings.HasSuffix(a, "/*") && strings.HasPrefix(chart, strings.TrimSuffix(a, "*")) {
			return true
		}
	}
	return false
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

func capabilityFor(name string, settings Settings) dispatcher.Capability {
	scope := "all namespaces and charts"
	if len(settings.Namespaces) > 0 || len(settings.Charts) > 0 {
		scope = fmt.Sprintf("namespaces: %s; charts: %s", displayList(settings.Namespaces), displayList(settings.Charts))
	}
	approval := ""
	if requiresApproval(name, settings) {
		approval = " Requires human approval."
	}
	descriptions := map[string]string{
		"helm.list":       "List Helm releases.",
		"helm.status":     "Get the status of a Helm release.",
		"helm.get_values": "Get values for a Helm release.",
		"helm.install":    "Install a Helm release.",
		"helm.upgrade":    "Upgrade a Helm release.",
		"helm.rollback":   "Roll back a Helm release to a prior revision.",
		"helm.uninstall":  "Uninstall a Helm release.",
		"helm.template":   "Render a Helm chart locally.",
	}
	return dispatcher.Capability{
		Name:        name,
		Description: fmt.Sprintf("%s Scope: %s.%s", descriptions[name], scope, approval),
		InputSchema: schemas[name],
	}
}

var schemas = map[string]json.RawMessage{
	"helm.list":       json.RawMessage(`{"type":"object","properties":{"namespace":{"type":"string"},"filter":{"type":"string"}},"additionalProperties":false}`),
	"helm.status":     json.RawMessage(`{"type":"object","properties":{"release":{"type":"string"},"namespace":{"type":"string"}},"required":["release"],"additionalProperties":false}`),
	"helm.get_values": json.RawMessage(`{"type":"object","properties":{"release":{"type":"string"},"namespace":{"type":"string"},"all":{"type":"boolean"}},"required":["release"],"additionalProperties":false}`),
	"helm.install":    json.RawMessage(`{"type":"object","properties":{"release":{"type":"string"},"chart":{"type":"string"},"namespace":{"type":"string"},"version":{"type":"string"},"values":{"type":"object"},"create_namespace":{"type":"boolean"},"wait":{"type":"boolean"},"timeout":{"type":"string"}},"required":["release","chart"],"additionalProperties":false}`),
	"helm.upgrade":    json.RawMessage(`{"type":"object","properties":{"release":{"type":"string"},"chart":{"type":"string"},"namespace":{"type":"string"},"version":{"type":"string"},"values":{"type":"object"},"install":{"type":"boolean"},"wait":{"type":"boolean"},"timeout":{"type":"string"}},"required":["release","chart"],"additionalProperties":false}`),
	"helm.rollback":   json.RawMessage(`{"type":"object","properties":{"release":{"type":"string"},"revision":{"type":"integer","minimum":1},"namespace":{"type":"string"},"wait":{"type":"boolean"},"timeout":{"type":"string"}},"required":["release","revision"],"additionalProperties":false}`),
	"helm.uninstall":  json.RawMessage(`{"type":"object","properties":{"release":{"type":"string"},"namespace":{"type":"string"},"keep_history":{"type":"boolean"},"wait":{"type":"boolean"},"timeout":{"type":"string"}},"required":["release"],"additionalProperties":false}`),
	"helm.template":   json.RawMessage(`{"type":"object","properties":{"release":{"type":"string"},"chart":{"type":"string"},"namespace":{"type":"string"},"version":{"type":"string"},"values":{"type":"object"},"include_crds":{"type":"boolean"}},"required":["release","chart"],"additionalProperties":false}`),
}

func cleanList(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	cleaned := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		cleaned = append(cleaned, value)
	}
	sort.Strings(cleaned)
	return cleaned
}

func displayList(values []string) string {
	if len(values) == 0 {
		return "all"
	}
	return strings.Join(values, ", ")
}
