package helm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"aurora-dispatchers/builtin"
	"aurora-dispatchers/resolution"
	"capcompute/dispatcher"
)

var _ builtin.Handler = (*Handler)(nil)

type capabilityConfig struct {
	policy          policy
	requireApproval bool
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
		policy:          newPolicy(settings),
		requireApproval: requiresApproval(name, settings),
	}
}

func (h *Handler) Handles(name string) bool {
	_, ok := h.capabilities[name]
	return ok
}

func (h *Handler) DispatchCall(ctx context.Context, call dispatcher.Call) (dispatcher.Outcome, error) {
	capability, ok := h.capabilities[call.Name]
	if !ok {
		return dispatcher.Failed("unknown helm call: " + call.Name), nil
	}
	switch call.Name {
	case "helm.list":
		return h.dispatchList(ctx, call, capability)
	case "helm.status":
		return h.dispatchStatus(ctx, call, capability)
	case "helm.get_values":
		return h.dispatchGetValues(ctx, call, capability)
	case "helm.install":
		return h.dispatchInstall(ctx, call, capability)
	case "helm.upgrade":
		return h.dispatchUpgrade(ctx, call, capability)
	case "helm.rollback":
		return h.dispatchRollback(ctx, call, capability)
	case "helm.uninstall":
		return h.dispatchUninstall(ctx, call, capability)
	case "helm.template":
		return h.dispatchTemplate(ctx, call, capability)
	default:
		return dispatcher.Failed("unsupported helm operation: " + call.Name), nil
	}
}

func (h *Handler) dispatchList(ctx context.Context, call dispatcher.Call, cap capabilityConfig) (dispatcher.Outcome, error) {
	var req ListRequest
	if outcome := decode(call, &req); outcome != nil {
		return *outcome, nil
	}
	if err := cap.policy.checkNamespace(req.Namespace); err != nil {
		return dispatcher.Failed(err.Error()), nil
	}
	data, err := h.client.List(ctx, req)
	return jsonClientResult(ctx, data, err)
}

func (h *Handler) dispatchStatus(ctx context.Context, call dispatcher.Call, cap capabilityConfig) (dispatcher.Outcome, error) {
	var req StatusRequest
	if outcome := decode(call, &req); outcome != nil {
		return *outcome, nil
	}
	if req.Release == "" {
		return dispatcher.Failed("release is required"), nil
	}
	if err := cap.policy.checkNamespace(req.Namespace); err != nil {
		return dispatcher.Failed(err.Error()), nil
	}
	if outcome := approval(ctx, cap, fmt.Sprintf("helm.status %s in %s", req.Release, req.Namespace)); outcome != nil {
		return *outcome, nil
	}
	data, err := h.client.Status(ctx, req)
	return jsonClientResult(ctx, data, err)
}

func (h *Handler) dispatchGetValues(ctx context.Context, call dispatcher.Call, cap capabilityConfig) (dispatcher.Outcome, error) {
	var req GetValuesRequest
	if outcome := decode(call, &req); outcome != nil {
		return *outcome, nil
	}
	if req.Release == "" {
		return dispatcher.Failed("release is required"), nil
	}
	if err := cap.policy.checkNamespace(req.Namespace); err != nil {
		return dispatcher.Failed(err.Error()), nil
	}
	if outcome := approval(ctx, cap, fmt.Sprintf("helm.get_values %s in %s", req.Release, req.Namespace)); outcome != nil {
		return *outcome, nil
	}
	data, err := h.client.GetValues(ctx, req)
	return jsonClientResult(ctx, data, err)
}

func (h *Handler) dispatchInstall(ctx context.Context, call dispatcher.Call, cap capabilityConfig) (dispatcher.Outcome, error) {
	var req InstallRequest
	if outcome := decode(call, &req); outcome != nil {
		return *outcome, nil
	}
	if outcome := validateReleaseChart(cap, req.Release, req.Chart, req.Namespace); outcome != nil {
		return *outcome, nil
	}
	if outcome := approval(ctx, cap, fmt.Sprintf("helm.install %s from %s in %s", req.Release, req.Chart, req.Namespace)); outcome != nil {
		return *outcome, nil
	}
	data, err := h.client.Install(ctx, req)
	return jsonClientResult(ctx, data, err)
}

func (h *Handler) dispatchUpgrade(ctx context.Context, call dispatcher.Call, cap capabilityConfig) (dispatcher.Outcome, error) {
	var req UpgradeRequest
	if outcome := decode(call, &req); outcome != nil {
		return *outcome, nil
	}
	if outcome := validateReleaseChart(cap, req.Release, req.Chart, req.Namespace); outcome != nil {
		return *outcome, nil
	}
	if outcome := approval(ctx, cap, fmt.Sprintf("helm.upgrade %s from %s in %s", req.Release, req.Chart, req.Namespace)); outcome != nil {
		return *outcome, nil
	}
	data, err := h.client.Upgrade(ctx, req)
	return jsonClientResult(ctx, data, err)
}

func (h *Handler) dispatchRollback(ctx context.Context, call dispatcher.Call, cap capabilityConfig) (dispatcher.Outcome, error) {
	var req RollbackRequest
	if outcome := decode(call, &req); outcome != nil {
		return *outcome, nil
	}
	if req.Release == "" || req.Revision < 1 {
		return dispatcher.Failed("release and a positive revision are required"), nil
	}
	if err := cap.policy.checkNamespace(req.Namespace); err != nil {
		return dispatcher.Failed(err.Error()), nil
	}
	if outcome := approval(ctx, cap, fmt.Sprintf("helm.rollback %s to revision %d in %s", req.Release, req.Revision, req.Namespace)); outcome != nil {
		return *outcome, nil
	}
	output, err := h.client.Rollback(ctx, req)
	return textClientResult(ctx, output, err)
}

func (h *Handler) dispatchUninstall(ctx context.Context, call dispatcher.Call, cap capabilityConfig) (dispatcher.Outcome, error) {
	var req UninstallRequest
	if outcome := decode(call, &req); outcome != nil {
		return *outcome, nil
	}
	if req.Release == "" {
		return dispatcher.Failed("release is required"), nil
	}
	if err := cap.policy.checkNamespace(req.Namespace); err != nil {
		return dispatcher.Failed(err.Error()), nil
	}
	if outcome := approval(ctx, cap, fmt.Sprintf("helm.uninstall %s in %s", req.Release, req.Namespace)); outcome != nil {
		return *outcome, nil
	}
	output, err := h.client.Uninstall(ctx, req)
	return textClientResult(ctx, output, err)
}

func (h *Handler) dispatchTemplate(ctx context.Context, call dispatcher.Call, cap capabilityConfig) (dispatcher.Outcome, error) {
	var req TemplateRequest
	if outcome := decode(call, &req); outcome != nil {
		return *outcome, nil
	}
	if outcome := validateReleaseChart(cap, req.Release, req.Chart, req.Namespace); outcome != nil {
		return *outcome, nil
	}
	if outcome := approval(ctx, cap, fmt.Sprintf("helm.template %s from %s in %s", req.Release, req.Chart, req.Namespace)); outcome != nil {
		return *outcome, nil
	}
	output, err := h.client.Template(ctx, req)
	return textClientResult(ctx, output, err)
}

func decode(call dispatcher.Call, target any) *dispatcher.Outcome {
	if err := json.Unmarshal(call.Args, target); err != nil {
		outcome := dispatcher.Failed(fmt.Sprintf("decode %s: %v", call.Name, err))
		return &outcome
	}
	return nil
}

func validateReleaseChart(cap capabilityConfig, release, chart, namespace string) *dispatcher.Outcome {
	if release == "" || chart == "" {
		outcome := dispatcher.Failed("release and chart are required")
		return &outcome
	}
	if err := cap.policy.checkNamespace(namespace); err != nil {
		outcome := dispatcher.Failed(err.Error())
		return &outcome
	}
	if err := cap.policy.checkChart(chart); err != nil {
		outcome := dispatcher.Failed(err.Error())
		return &outcome
	}
	return nil
}

func approval(ctx context.Context, cap capabilityConfig, summary string) *dispatcher.Outcome {
	if !cap.requireApproval {
		return nil
	}
	if resolved, ok := resolution.FromContext(ctx); ok && resolved.Decision == resolution.Approved {
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
	return dispatcher.Failed(err.Error()), nil
}

func marshalResult(value any) (dispatcher.Outcome, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return dispatcher.Outcome{}, err
	}
	return dispatcher.Result(raw), nil
}
