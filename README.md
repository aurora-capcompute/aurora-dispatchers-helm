# aurora-dispatchers-helm

Helm dispatcher capabilities for Aurora. The implementation uses the Helm CLI,
so `helm` must be installed on the Aurora host and have access to the target
cluster. Its command surface is compatible with Helm 3 and Helm 4.

## Capabilities

- `helm.list`
- `helm.status`
- `helm.get_values`
- `helm.template`
- `helm.install`
- `helm.upgrade`
- `helm.rollback`
- `helm.uninstall`

Mutating operations require human approval by default. Set
`"require_approval": false` on a capability to disable this.

```json
{
  "capabilities": [
    {
      "name": "helm.list",
      "settings": {
        "namespaces": ["default", "observability"]
      }
    },
    {
      "name": "helm.install",
      "settings": {
        "namespaces": ["default"],
        "charts": ["bitnami/*"]
      }
    },
    {
      "name": "helm.upgrade",
      "settings": {
        "namespaces": ["default"],
        "charts": ["bitnami/*"]
      }
    },
    {
      "name": "helm.uninstall",
      "settings": {
        "namespaces": ["default"]
      }
    }
  ]
}
```

## Settings

- `helm_binary`: Helm executable path or name; defaults to `helm`.
- `kubeconfig`: optional kubeconfig path.
- `context`: optional kubeconfig context.
- `namespaces`: allowed namespaces; empty permits all namespaces.
- `charts`: allowed chart references; empty permits all charts. An entry ending
  in `/*` permits charts below that repository prefix.
- `require_approval`: overrides the default approval behavior.

Settings are defined per capability. A namespace must be supplied when a
namespace allowlist is configured. `helm.list` uses all namespaces only when no
allowlist is configured.

## Integration

Register `helm.Registration{}` with the Aurora dispatcher registry:

```go
registry.New(
    helm.Registration{},
)
```

For local development this module uses sibling `replace` directives for
`aurora-dispatchers` and `capcompute`. Replace those directives with published
module versions when consuming the repository independently.
