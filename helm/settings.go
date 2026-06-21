package helm

import (
	"fmt"
	"sort"
	"strings"
)

type Settings struct {
	HelmBinary      string   `json:"helm_binary,omitempty"`
	Kubeconfig      string   `json:"kubeconfig,omitempty"`
	Context         string   `json:"context,omitempty"`
	Namespaces      []string `json:"namespaces,omitempty"`
	Charts          []string `json:"charts,omitempty"`
	RequireApproval *bool    `json:"require_approval,omitempty"`
}

func isMutating(name string) bool {
	switch name {
	case "helm.install", "helm.upgrade", "helm.rollback", "helm.uninstall":
		return true
	default:
		return false
	}
}

func requiresApproval(name string, settings Settings) bool {
	if settings.RequireApproval != nil {
		return *settings.RequireApproval
	}
	return isMutating(name)
}

type policy struct {
	namespaces map[string]struct{}
	charts     []string
}

func newPolicy(settings Settings) policy {
	namespaces := make(map[string]struct{}, len(settings.Namespaces))
	for _, namespace := range settings.Namespaces {
		if namespace = strings.TrimSpace(namespace); namespace != "" {
			namespaces[namespace] = struct{}{}
		}
	}
	return policy{namespaces: namespaces, charts: append([]string(nil), settings.Charts...)}
}

func (p policy) checkNamespace(namespace string) error {
	if len(p.namespaces) == 0 {
		return nil
	}
	if namespace == "" {
		return fmt.Errorf("namespace is required (allowed: %s)", strings.Join(p.namespaceList(), ", "))
	}
	if _, ok := p.namespaces[namespace]; !ok {
		return fmt.Errorf("namespace %q is not allowed (allowed: %s)", namespace, strings.Join(p.namespaceList(), ", "))
	}
	return nil
}

func (p policy) checkChart(chart string) error {
	if len(p.charts) == 0 {
		return nil
	}
	for _, allowed := range p.charts {
		if allowed == chart {
			return nil
		}
		if strings.HasSuffix(allowed, "/*") && strings.HasPrefix(chart, strings.TrimSuffix(allowed, "*")) {
			return nil
		}
	}
	return fmt.Errorf("chart %q is not allowed (allowed: %s)", chart, strings.Join(p.charts, ", "))
}

func (p policy) namespaceList() []string {
	names := make([]string, 0, len(p.namespaces))
	for namespace := range p.namespaces {
		names = append(names, namespace)
	}
	sort.Strings(names)
	return names
}
