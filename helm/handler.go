package helm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/aurora-capcompute/aurora-dispatchers/builtin"
	"github.com/aurora-capcompute/capcompute/dispatcher"
)

var _ builtin.Handler = (*Handler)(nil)

type capabilityConfig struct {
	policy          permissionPolicy
	requireApproval *bool
}

type Handler struct {
	client       Client
	capabilities map[string]capabilityConfig
	connection   connectionSettings
}

func NewHandler(client Client) *Handler {
	return &Handler{client: client, capabilities: make(map[string]capabilityConfig)}
}

type connectionSettings struct {
	binary     string
	kubeconfig string
	context    string
}

func (h *Handler) AddCapability(name string, settings Settings) {
	h.capabilities[name] = capabilityConfig{
		policy:          newPermissionPolicy(settings.Permissions),
		requireApproval: settings.RequireApproval,
	}
}

func (h *Handler) Handles(name string) bool {
	_, ok := h.capabilities[name]
	return ok
}

func (h *Handler) DispatchCall(ctx context.Context, call dispatcher.Call, auth dispatcher.Authorization) (dispatcher.Outcome, error) {
	capability, ok := h.capabilities[call.Name]
	if !ok {
		return dispatcher.Fail("unknown helm tool: " + call.Name), nil
	}
	var disc struct {
		Verb string `json:"verb"`
	}
	if err := json.Unmarshal(call.Args, &disc); err != nil {
		return dispatcher.Fail(fmt.Sprintf("decode verb: %v", err)), nil
	}
	verb := strings.ToLower(strings.TrimSpace(disc.Verb))
	switch verb {
	case "list":
		return h.dispatchList(ctx, call, capability)
	case "status":
		return h.dispatchStatus(ctx, call, capability, auth)
	case "get_values":
		return h.dispatchGetValues(ctx, call, capability, auth)
	case "install":
		return h.dispatchInstall(ctx, call, capability, auth)
	case "upgrade":
		return h.dispatchUpgrade(ctx, call, capability, auth)
	case "rollback":
		return h.dispatchRollback(ctx, call, capability, auth)
	case "uninstall":
		return h.dispatchUninstall(ctx, call, capability, auth)
	case "template":
		return h.dispatchTemplate(ctx, call, capability, auth)
	default:
		return dispatcher.Fail("unsupported verb: " + disc.Verb), nil
	}
}

func (h *Handler) dispatchList(ctx context.Context, call dispatcher.Call, cap capabilityConfig) (dispatcher.Outcome, error) {
	var req ListRequest
	if outcome := decode(call, &req); outcome != nil {
		return *outcome, nil
	}
	if denied := permitRelease(cap, "list", req.Namespace); denied != nil {
		return *denied, nil
	}
	data, err := h.client.List(ctx, req)
	return jsonClientResult(ctx, data, err)
}

func (h *Handler) dispatchStatus(ctx context.Context, call dispatcher.Call, cap capabilityConfig, auth dispatcher.Authorization) (dispatcher.Outcome, error) {
	var req StatusRequest
	if outcome := decode(call, &req); outcome != nil {
		return *outcome, nil
	}
	if req.Release == "" {
		return dispatcher.Fail("release is required"), nil
	}
	if denied := permitRelease(cap, "status", req.Namespace); denied != nil {
		return *denied, nil
	}
	if outcome := approval(auth, cap, "status", fmt.Sprintf("status %s in %s", req.Release, req.Namespace)); outcome != nil {
		return *outcome, nil
	}
	data, err := h.client.Status(ctx, req)
	return jsonClientResult(ctx, data, err)
}

func (h *Handler) dispatchGetValues(ctx context.Context, call dispatcher.Call, cap capabilityConfig, auth dispatcher.Authorization) (dispatcher.Outcome, error) {
	var req GetValuesRequest
	if outcome := decode(call, &req); outcome != nil {
		return *outcome, nil
	}
	if req.Release == "" {
		return dispatcher.Fail("release is required"), nil
	}
	if denied := permitRelease(cap, "get_values", req.Namespace); denied != nil {
		return *denied, nil
	}
	if outcome := approval(auth, cap, "get_values", fmt.Sprintf("get_values %s in %s", req.Release, req.Namespace)); outcome != nil {
		return *outcome, nil
	}
	data, err := h.client.GetValues(ctx, req)
	return jsonClientResult(ctx, data, err)
}

func (h *Handler) dispatchInstall(ctx context.Context, call dispatcher.Call, cap capabilityConfig, auth dispatcher.Authorization) (dispatcher.Outcome, error) {
	var req InstallRequest
	if outcome := decode(call, &req); outcome != nil {
		return *outcome, nil
	}
	if outcome := validateReleaseChart(cap, "install", req.Release, req.Chart, req.Namespace); outcome != nil {
		return *outcome, nil
	}
	if outcome := approval(auth, cap, "install", fmt.Sprintf("install %s from %s in %s", req.Release, req.Chart, req.Namespace)); outcome != nil {
		return *outcome, nil
	}
	data, err := h.client.Install(ctx, req)
	return jsonClientResult(ctx, data, err)
}

func (h *Handler) dispatchUpgrade(ctx context.Context, call dispatcher.Call, cap capabilityConfig, auth dispatcher.Authorization) (dispatcher.Outcome, error) {
	var req UpgradeRequest
	if outcome := decode(call, &req); outcome != nil {
		return *outcome, nil
	}
	if outcome := validateReleaseChart(cap, "upgrade", req.Release, req.Chart, req.Namespace); outcome != nil {
		return *outcome, nil
	}
	if outcome := approval(auth, cap, "upgrade", fmt.Sprintf("upgrade %s from %s in %s", req.Release, req.Chart, req.Namespace)); outcome != nil {
		return *outcome, nil
	}
	data, err := h.client.Upgrade(ctx, req)
	return jsonClientResult(ctx, data, err)
}

func (h *Handler) dispatchRollback(ctx context.Context, call dispatcher.Call, cap capabilityConfig, auth dispatcher.Authorization) (dispatcher.Outcome, error) {
	var req RollbackRequest
	if outcome := decode(call, &req); outcome != nil {
		return *outcome, nil
	}
	if req.Release == "" || req.Revision < 1 {
		return dispatcher.Fail("release and a positive revision are required"), nil
	}
	if denied := permitRelease(cap, "rollback", req.Namespace); denied != nil {
		return *denied, nil
	}
	if outcome := approval(auth, cap, "rollback", fmt.Sprintf("rollback %s to revision %d in %s", req.Release, req.Revision, req.Namespace)); outcome != nil {
		return *outcome, nil
	}
	output, err := h.client.Rollback(ctx, req)
	return textClientResult(ctx, output, err)
}

func (h *Handler) dispatchUninstall(ctx context.Context, call dispatcher.Call, cap capabilityConfig, auth dispatcher.Authorization) (dispatcher.Outcome, error) {
	var req UninstallRequest
	if outcome := decode(call, &req); outcome != nil {
		return *outcome, nil
	}
	if req.Release == "" {
		return dispatcher.Fail("release is required"), nil
	}
	if denied := permitRelease(cap, "uninstall", req.Namespace); denied != nil {
		return *denied, nil
	}
	if outcome := approval(auth, cap, "uninstall", fmt.Sprintf("uninstall %s in %s", req.Release, req.Namespace)); outcome != nil {
		return *outcome, nil
	}
	output, err := h.client.Uninstall(ctx, req)
	return textClientResult(ctx, output, err)
}

func (h *Handler) dispatchTemplate(ctx context.Context, call dispatcher.Call, cap capabilityConfig, auth dispatcher.Authorization) (dispatcher.Outcome, error) {
	var req TemplateRequest
	if outcome := decode(call, &req); outcome != nil {
		return *outcome, nil
	}
	if outcome := validateReleaseChart(cap, "template", req.Release, req.Chart, req.Namespace); outcome != nil {
		return *outcome, nil
	}
	if outcome := approval(auth, cap, "template", fmt.Sprintf("template %s from %s in %s", req.Release, req.Chart, req.Namespace)); outcome != nil {
		return *outcome, nil
	}
	output, err := h.client.Template(ctx, req)
	return textClientResult(ctx, output, err)
}

func decode(call dispatcher.Call, target any) *dispatcher.Outcome {
	if err := json.Unmarshal(call.Args, target); err != nil {
		outcome := dispatcher.Fail(fmt.Sprintf("decode %s: %v", call.Name, err))
		return &outcome
	}
	return nil
}

// permitRelease gates a release-only verb (resource/chart not applicable).
func permitRelease(cap capabilityConfig, verb, namespace string) *dispatcher.Outcome {
	if cap.policy.allowsRelease(verb, namespace) {
		return nil
	}
	out := dispatcher.Fail(fmt.Sprintf("not permitted: %s in namespace %q", verb, namespace))
	return &out
}

// validateReleaseChart requires release+chart and gates a chart-bearing verb.
func validateReleaseChart(cap capabilityConfig, verb, release, chart, namespace string) *dispatcher.Outcome {
	if release == "" || chart == "" {
		outcome := dispatcher.Fail("release and chart are required")
		return &outcome
	}
	if !cap.policy.allowsChart(verb, chart, namespace) {
		outcome := dispatcher.Fail(fmt.Sprintf("not permitted: %s chart %q in namespace %q", verb, chart, namespace))
		return &outcome
	}
	return nil
}

func approval(auth dispatcher.Authorization, cap capabilityConfig, verb, summary string) *dispatcher.Outcome {
	if !requiresApproval(verb, cap.requireApproval) {
		return nil
	}
	if auth.Decision == dispatcher.Approved {
		return nil
	}
	outcome := dispatcher.Yield("Approve: " + strings.TrimSpace(summary))
	return &outcome
}

func jsonClientResult(ctx context.Context, data json.RawMessage, err error) (dispatcher.Outcome, error) {
	if err != nil {
		return clientError(ctx, err)
	}
	return marshalResult(JSONResponse{Data: data})
}

func textClientResult(ctx context.Context, output string, err error) (dispatcher.Outcome, error) {
	if err != nil {
		return clientError(ctx, err)
	}
	return marshalResult(TextResponse{Output: output})
}

func clientError(ctx context.Context, err error) (dispatcher.Outcome, error) {
	if ctx.Err() != nil {
		return dispatcher.Outcome{}, ctx.Err()
	}
	return dispatcher.Fail(err.Error()), nil
}

func marshalResult(value any) (dispatcher.Outcome, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return dispatcher.Outcome{}, err
	}
	return dispatcher.Result(raw), nil
}
