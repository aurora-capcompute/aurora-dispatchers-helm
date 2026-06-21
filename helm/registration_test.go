package helm

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestNormalizeCleansAndSortsPolicyLists(t *testing.T) {
	raw, err := (Registration{}).Normalize("helm.install", json.RawMessage(`{
		"helm_binary":"  helm ",
		"namespaces":[" production ","default","default",""],
		"charts":[" internal/* ","bitnami/nginx","internal/*"]
	}`))
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	var settings Settings
	if err := json.Unmarshal(raw, &settings); err != nil {
		t.Fatalf("decode normalized settings: %v", err)
	}
	if settings.HelmBinary != "helm" {
		t.Fatalf("helm binary = %q", settings.HelmBinary)
	}
	if !reflect.DeepEqual(settings.Namespaces, []string{"default", "production"}) {
		t.Fatalf("namespaces = %#v", settings.Namespaces)
	}
	if !reflect.DeepEqual(settings.Charts, []string{"bitnami/nginx", "internal/*"}) {
		t.Fatalf("charts = %#v", settings.Charts)
	}
}
