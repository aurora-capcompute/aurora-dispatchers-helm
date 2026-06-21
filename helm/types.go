package helm

import "encoding/json"

type ListRequest struct {
	Namespace string `json:"namespace,omitempty"`
	Filter    string `json:"filter,omitempty"`
}

type StatusRequest struct {
	Release   string `json:"release"`
	Namespace string `json:"namespace,omitempty"`
}

type GetValuesRequest struct {
	Release   string `json:"release"`
	Namespace string `json:"namespace,omitempty"`
	All       bool   `json:"all,omitempty"`
}

type InstallRequest struct {
	Release         string          `json:"release"`
	Chart           string          `json:"chart"`
	Namespace       string          `json:"namespace,omitempty"`
	Version         string          `json:"version,omitempty"`
	Values          json.RawMessage `json:"values,omitempty"`
	CreateNamespace bool            `json:"create_namespace,omitempty"`
	Wait            bool            `json:"wait,omitempty"`
	Timeout         string          `json:"timeout,omitempty"`
}

type UpgradeRequest struct {
	Release   string          `json:"release"`
	Chart     string          `json:"chart"`
	Namespace string          `json:"namespace,omitempty"`
	Version   string          `json:"version,omitempty"`
	Values    json.RawMessage `json:"values,omitempty"`
	Install   bool            `json:"install,omitempty"`
	Wait      bool            `json:"wait,omitempty"`
	Timeout   string          `json:"timeout,omitempty"`
}

type RollbackRequest struct {
	Release   string `json:"release"`
	Revision  int    `json:"revision"`
	Namespace string `json:"namespace,omitempty"`
	Wait      bool   `json:"wait,omitempty"`
	Timeout   string `json:"timeout,omitempty"`
}

type UninstallRequest struct {
	Release     string `json:"release"`
	Namespace   string `json:"namespace,omitempty"`
	KeepHistory bool   `json:"keep_history,omitempty"`
	Wait        bool   `json:"wait,omitempty"`
	Timeout     string `json:"timeout,omitempty"`
}

type TemplateRequest struct {
	Release     string          `json:"release"`
	Chart       string          `json:"chart"`
	Namespace   string          `json:"namespace,omitempty"`
	Version     string          `json:"version,omitempty"`
	Values      json.RawMessage `json:"values,omitempty"`
	IncludeCRDs bool            `json:"include_crds,omitempty"`
}

type JSONResponse struct {
	Data json.RawMessage `json:"data"`
}

type TextResponse struct {
	Output string `json:"output"`
}
