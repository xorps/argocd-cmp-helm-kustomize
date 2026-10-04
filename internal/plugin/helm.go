package plugin

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"helm.sh/helm/v4/pkg/action"
	"helm.sh/helm/v4/pkg/chart/common"
	"helm.sh/helm/v4/pkg/chart/loader"
	"helm.sh/helm/v4/pkg/cli/values"
	"helm.sh/helm/v4/pkg/getter"
	release "helm.sh/helm/v4/pkg/release/v1"
	"sigs.k8s.io/kustomize/api/types"
)

// helmRenderer renders a chart like `helm template`.
type helmRenderer struct {
	chartPath   string
	releaseName string
	namespace   string
	valueFiles  []string
	values      []string
	patches     []types.Patch
	kubeVersion *common.KubeVersion
	apiVersions []string
	skipCRDs    bool
}

func newHelmRenderer(o *Options) (*helmRenderer, error) {
	switch {
	case o.Path == "":
		return nil, errors.New("path is required")
	case o.ReleaseName == "":
		return nil, errors.New("releaseName is required")
	case o.Namespace == "":
		return nil, errors.New("namespace is required")
	}
	if err := confine(o.Root, "path", o.Path); err != nil {
		return nil, err
	}
	if err := confine(o.Root, "valueFiles", o.ValueFiles...); err != nil {
		return nil, err
	}
	var kubeVersion *common.KubeVersion
	if o.KubeVersion != "" {
		v, err := common.ParseKubeVersion(o.KubeVersion)
		if err != nil {
			return nil, fmt.Errorf("kube version %q: %w", o.KubeVersion, err)
		}
		kubeVersion = v
	}
	return &helmRenderer{
		chartPath:   o.Path,
		releaseName: o.ReleaseName,
		namespace:   o.Namespace,
		valueFiles:  o.ValueFiles,
		values:      o.Values,
		patches:     o.Patches,
		kubeVersion: kubeVersion,
		apiVersions: o.APIVersions,
		skipCRDs:    o.SkipCRDs,
	}, nil
}

func (h *helmRenderer) Render(ctx context.Context) (string, error) {
	chart, err := loader.Load(h.chartPath)
	if err != nil {
		return "", err
	}

	install := action.NewInstall(action.NewConfiguration())
	// Replaces ClientOnly from Helm v3.
	install.DryRunStrategy = action.DryRunClient
	install.Replace = true
	install.ReleaseName = h.releaseName
	install.Namespace = h.namespace
	install.CreateNamespace = false
	install.IncludeCRDs = !h.skipCRDs
	install.KubeVersion = h.kubeVersion
	install.APIVersions = common.VersionSet(h.apiVersions)
	install.PostRenderer = newPatchPostRenderer(h.patches)

	vals, err := h.mergeValues()
	if err != nil {
		return "", err
	}

	r, err := install.RunWithContext(ctx, chart, vals)
	if err != nil {
		return "", err
	}
	rel, ok := r.(*release.Release)
	if !ok {
		return "", fmt.Errorf("unsupported release type %T", r)
	}
	return manifests(rel), nil
}

// manifests returns the templates followed by the hooks, as `helm template`
// prints them.
func manifests(rel *release.Release) string {
	var b strings.Builder
	b.WriteString(rel.Manifest)
	for _, h := range rel.Hooks {
		fmt.Fprintf(&b, "---\n# Source: %s\n%s\n", h.Path, h.Manifest)
	}
	return b.String()
}

// mergeValues merges valueFiles, then values.
func (h *helmRenderer) mergeValues() (map[string]any, error) {
	opts := values.Options{ValueFiles: h.valueFiles, Values: h.values}
	// No getters, so value files are never fetched from URLs.
	return opts.MergeValues(getter.Providers{})
}
