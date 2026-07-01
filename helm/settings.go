package helm

import (
	"strings"
)

// Permission is one allowlisted operation: a verb on a chart in a namespace.
// Empty or "*" in any field means "any". `Resource` is matched case-insensitively
// against the chart name for chart-bearing verbs (install/upgrade/template); it is
// ignored for release-only verbs.
type Permission struct {
	Resource  string `json:"resource,omitempty"`
	Verb      string `json:"verb,omitempty"`
	Namespace string `json:"namespace,omitempty"`
}

type Settings struct {
	HelmBinary      string       `json:"helm_binary,omitempty"`
	Kubeconfig      string       `json:"kubeconfig,omitempty"`
	Context         string       `json:"context,omitempty"`
	Permissions     []Permission `json:"permissions"`
	RequireApproval *bool        `json:"require_approval,omitempty"`
}

// knownVerbs are the operations a helm tool can expose, in published order.
var knownVerbs = []string{"list", "status", "get_values", "install", "upgrade", "rollback", "uninstall", "template"}

func isMutatingVerb(verb string) bool {
	switch verb {
	case "install", "upgrade", "rollback", "uninstall":
		return true
	default:
		return false
	}
}

// requiresApproval reports whether a verb needs human approval. An explicit
// per-tool override wins; otherwise mutating verbs require approval.
func requiresApproval(verb string, override *bool) bool {
	if override != nil {
		return *override
	}
	return isMutatingVerb(verb)
}

type permissionPolicy struct {
	perms []Permission
}

func newPermissionPolicy(perms []Permission) permissionPolicy {
	return permissionPolicy{perms: perms}
}

// allowsChart reports whether a chart-bearing verb is granted for (chart, namespace).
func (p permissionPolicy) allowsChart(verb, chart, namespace string) bool {
	for _, perm := range p.perms {
		if matchToken(perm.Verb, verb) && matchToken(perm.Resource, chart) && matchToken(perm.Namespace, namespace) {
			return true
		}
	}
	return false
}

// allowsRelease reports whether a release-only verb is granted in a namespace.
// The resource (chart) dimension is not applicable and is ignored.
func (p permissionPolicy) allowsRelease(verb, namespace string) bool {
	for _, perm := range p.perms {
		if matchToken(perm.Verb, verb) && matchToken(perm.Namespace, namespace) {
			return true
		}
	}
	return false
}

// permittedVerbs returns the known verbs this policy grants, in published order.
// A wildcard (empty or "*") verb grants all known verbs.
func (p permissionPolicy) permittedVerbs() []string {
	wildcard := false
	granted := make(map[string]bool, len(p.perms))
	for _, perm := range p.perms {
		v := strings.ToLower(strings.TrimSpace(perm.Verb))
		if v == "" || v == "*" {
			wildcard = true
			break
		}
		granted[v] = true
	}
	out := make([]string, 0, len(knownVerbs))
	for _, v := range knownVerbs {
		if wildcard || granted[v] {
			out = append(out, v)
		}
	}
	return out
}

// matchToken matches an allowlist token against an actual value. Empty or "*"
// matches anything; a trailing "*" is a case-insensitive prefix glob (e.g.
// "bitnami/*"); otherwise the comparison is exact and case-insensitive.
func matchToken(allowed, actual string) bool {
	allowed = strings.TrimSpace(allowed)
	if allowed == "" || allowed == "*" {
		return true
	}
	if prefix, ok := strings.CutSuffix(allowed, "*"); ok {
		return strings.HasPrefix(strings.ToLower(actual), strings.ToLower(prefix))
	}
	return strings.EqualFold(allowed, actual)
}
