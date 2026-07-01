package helm

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/aurora-capcompute/aurora-dispatchers/builtin"
	"github.com/aurora-capcompute/aurora-dispatchers/registry"
)

func TestHelmMatchesType(t *testing.T) {
	reg := Registration{}
	if !reg.Matches("core.helm") {
		t.Fatal("should match core.helm")
	}
	if reg.Matches("helm.install") {
		t.Fatal("must match by type, not an operation name")
	}
}

func TestHelmNormalizeDefaultsBinaryAndRequiresPermissions(t *testing.T) {
	if _, err := (Registration{}).Normalize("core.helm", json.RawMessage(`{}`)); err == nil {
		t.Fatal("expected error when permissions is empty")
	}
	raw, err := (Registration{}).Normalize("core.helm", json.RawMessage(`{"permissions":[{"verb":"list"}]}`))
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	var settings Settings
	if err := json.Unmarshal(raw, &settings); err != nil {
		t.Fatalf("decode normalized settings: %v", err)
	}
	if settings.HelmBinary != "helm" {
		t.Fatalf("helm binary = %q, want default helm", settings.HelmBinary)
	}
}

// One helm tool publishes one capability named by its local name, with a union
// input schema over exactly the verbs the permissions grant.
func TestHelmConfigurePublishesUnionOfPermittedVerbs(t *testing.T) {
	raw := json.RawMessage(`{"permissions":[{"verb":"list"},{"verb":"status"}]}`)
	var config builtin.Config
	if err := (Registration{}).Configure(context.Background(), "ops", raw, registry.Services{}, &config); err != nil {
		t.Fatalf("configure: %v", err)
	}
	if len(config.Capabilities) != 1 || config.Capabilities[0].Name != "ops" {
		t.Fatalf("capabilities = %+v, want one named ops", config.Capabilities)
	}
	var schema struct {
		OneOf []map[string]any `json:"oneOf"`
	}
	if err := json.Unmarshal(config.Capabilities[0].InputSchema, &schema); err != nil {
		t.Fatalf("schema not a oneOf union: %v", err)
	}
	if len(schema.OneOf) != 2 {
		t.Fatalf("oneOf branches = %d, want 2 (list, status only)", len(schema.OneOf))
	}
	if len(config.Handlers) != 1 || !config.Handlers[0].Handles("ops") {
		t.Fatal("handler must route by the local name ops")
	}
}
