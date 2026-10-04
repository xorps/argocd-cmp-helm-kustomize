package plugin

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/kustomize/kyaml/kio"
	"sigs.k8s.io/kustomize/kyaml/resid"
	kyaml "sigs.k8s.io/kustomize/kyaml/yaml"
)

// example copies examples/<name> into a temp dir, changes into it, and
// returns name.
func example(t *testing.T, name string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.CopyFS(filepath.Join(dir, name), os.DirFS(filepath.Join("..", "..", "examples", name))); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	return name
}

func tryRender(o *Options) (string, error) {
	r, err := New(o)
	if err != nil {
		return "", err
	}
	return r.Render(context.Background())
}

func render(t *testing.T, o *Options) []*kyaml.RNode {
	t.Helper()
	out, err := tryRender(o)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	nodes, err := kio.FromBytes([]byte(out))
	if err != nil {
		t.Fatalf("parsing output: %v\n%s", err, out)
	}
	return nodes
}

func find(t *testing.T, nodes []*kyaml.RNode, kind, name string) *kyaml.RNode {
	t.Helper()
	var have []string
	for _, n := range nodes {
		if n.GetKind() == kind && n.GetName() == name {
			return n
		}
		have = append(have, n.GetKind()+"/"+n.GetName())
	}
	t.Fatalf("%s/%s not found in output; have %v", kind, name, have)
	return nil
}

func field(t *testing.T, n *kyaml.RNode, path ...string) string {
	t.Helper()
	v, err := n.Pipe(kyaml.Lookup(path...))
	if err != nil {
		t.Fatal(err)
	}
	if v == nil {
		return ""
	}
	return v.YNode().Value
}

var priorityPatch = types.Patch{
	Target: &types.Selector{ResId: resid.NewResIdKindOnly("Deployment", "")},
	Patch:  "- op: add\n  path: /spec/template/spec/priorityClassName\n  value: high\n",
}

func TestRenderHelm(t *testing.T) {
	dir := example(t, "helm")
	nodes := render(t, &Options{
		Path:        dir,
		ReleaseName: "demo",
		Namespace:   "demo-ns",
		ValueFiles:  []string{filepath.Join(dir, "values-prod.yaml")},
		Values:      []string{"image.tag=1.27.3"},
		Patches:     []types.Patch{priorityPatch},
	})

	dep := find(t, nodes, "Deployment", "demo-web")
	for _, c := range []struct {
		path []string
		want string
	}{
		{[]string{"spec", "replicas"}, "2"},
		{[]string{"spec", "template", "spec", "containers", "[name=web]", "image"}, "nginx:1.27.3"},
		{[]string{"spec", "template", "spec", "priorityClassName"}, "high"},
	} {
		if got := field(t, dep, c.path...); got != c.want {
			t.Errorf("%s = %q, want %q", strings.Join(c.path, "."), got, c.want)
		}
	}
	if got := find(t, nodes, "Service", "demo-web").GetNamespace(); got != "demo-ns" {
		t.Errorf("service namespace = %q, want demo-ns", got)
	}
}

func TestRenderHelmWithoutPatches(t *testing.T) {
	nodes := render(t, &Options{Path: example(t, "helm"), ReleaseName: "demo", Namespace: "demo-ns"})
	if got := field(t, find(t, nodes, "Deployment", "demo-web"), "spec", "template", "spec", "priorityClassName"); got != "" {
		t.Errorf("unexpected priorityClassName %q", got)
	}
}

func TestRenderFiles(t *testing.T) {
	nodes := render(t, &Options{
		Path:    example(t, "manifests"),
		Files:   []string{"deployment.yaml", "service.yaml"},
		Patches: []types.Patch{priorityPatch},
	})
	if len(nodes) != 2 {
		t.Fatalf("got %d objects, want 2", len(nodes))
	}
	if got := field(t, find(t, nodes, "Deployment", "web"), "spec", "template", "spec", "priorityClassName"); got != "high" {
		t.Errorf("priorityClassName = %q, want high", got)
	}
	find(t, nodes, "Service", "web")
}

func TestRenderFilesOutsidePathRejected(t *testing.T) {
	_, err := tryRender(&Options{
		Path:  example(t, "manifests"),
		Files: []string{"../../etc/passwd"},
	})
	if err == nil {
		t.Fatal("want error for a file outside path")
	}
}

func TestNew(t *testing.T) {
	cases := map[string]*Options{
		"releaseName can't be used": {Files: []string{"a.yaml"}, ReleaseName: "x"},
		"namespace can't be used":   {Files: []string{"a.yaml"}, Namespace: "x"},
		"valueFiles can't be used":  {Files: []string{"a.yaml"}, ValueFiles: []string{"v.yaml"}},
		"values can't be used":      {Files: []string{"a.yaml"}, Values: []string{"a=b"}},
		"skipCrds can't be used":    {Files: []string{"a.yaml"}, SkipCRDs: true},
		"path is required":          {ReleaseName: "x", Namespace: "y"},
		"releaseName is required":   {Path: "p", Namespace: "y"},
		"namespace is required":     {Path: "p", ReleaseName: "x"},
	}
	for want, o := range cases {
		if _, err := New(o); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("New(%+v) = %v, want containing %q", o, err, want)
		}
	}

	dir := example(t, "helm")
	if r, err := New(&Options{Path: dir, ReleaseName: "x", Namespace: "y"}); err != nil {
		t.Errorf("helm: %v", err)
	} else if _, ok := r.(*helmRenderer); !ok {
		t.Errorf("helm: got %T, want *helmRenderer", r)
	}
	if r, err := New(&Options{Files: []string{"a.yaml"}}); err != nil {
		t.Errorf("files: %v", err)
	} else if _, ok := r.(*filesRenderer); !ok {
		t.Errorf("files: got %T, want *filesRenderer", r)
	}
}

func TestRenderValueFilesNotFetched(t *testing.T) {
	_, err := tryRender(&Options{
		Path:        example(t, "helm"),
		ReleaseName: "demo",
		Namespace:   "demo-ns",
		ValueFiles:  []string{"https://example.com/values.yaml"},
	})
	if err == nil || !strings.Contains(err.Error(), "no such file or directory") {
		t.Errorf("err = %v, want a missing local file", err)
	}
}

func TestRenderConfinedToWorkingDir(t *testing.T) {
	dir := example(t, "helm")
	outside := filepath.Join(t.TempDir(), "values.yaml")
	if err := os.WriteFile(outside, []byte("replicaCount: 9\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, "link.yaml")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Dir(outside), "linkdir"); err != nil {
		t.Fatal(err)
	}

	chart := func(valueFiles ...string) *Options {
		return &Options{Path: dir, ReleaseName: "demo", Namespace: "demo-ns", ValueFiles: valueFiles}
	}
	for name, o := range map[string]*Options{
		"absolute path":        {Path: filepath.Dir(outside), ReleaseName: "demo", Namespace: "demo-ns"},
		"path traversal":       {Path: "..", ReleaseName: "demo", Namespace: "demo-ns"},
		"symlinked path":       {Path: "linkdir", Files: []string{"values.yaml"}},
		"absolute value file":  chart(outside),
		"value file traversal": chart("../" + filepath.Base(outside)),
		"symlinked value file": chart(filepath.Join(dir, "link.yaml")),
	} {
		if _, err := tryRender(o); err == nil || !strings.Contains(err.Error(), "escapes") {
			t.Errorf("%s: err = %v, want path escapes error", name, err)
		}
	}
}

func TestRenderValueFileElsewhereInRepo(t *testing.T) {
	// charts/helm is the source path.
	repo := t.TempDir()
	if err := os.CopyFS(filepath.Join(repo, "charts", "helm"), os.DirFS(filepath.Join("..", "..", "examples", "helm"))); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(repo, "envs", "prod"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "envs", "prod", "values.yaml"), []byte("replicaCount: 5\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(filepath.Join(repo, "charts", "helm"))

	o := &Options{Path: ".", ReleaseName: "demo", Namespace: "demo-ns", ValueFiles: []string{"../../envs/prod/values.yaml"}}
	if _, err := tryRender(o); err == nil || !strings.Contains(err.Error(), "escapes") {
		t.Errorf("without Root: err = %v, want path escapes error", err)
	}

	o.Root = repo
	nodes := render(t, o)
	if got := field(t, find(t, nodes, "Deployment", "demo-web"), "spec", "replicas"); got != "5" {
		t.Errorf("replicas = %q, want 5", got)
	}

	o.ValueFiles = []string{"../../../outside.yaml"}
	if _, err := tryRender(o); err == nil || !strings.Contains(err.Error(), "escapes") {
		t.Errorf("outside repo: err = %v, want path escapes error", err)
	}
}

func TestRepoRoot(t *testing.T) {
	for _, c := range []struct{ wd, src, want string }{
		{"/tmp/x/charts/web", "charts/web", "/tmp/x"},
		{"/tmp/x/charts/web", "./charts/web/", "/tmp/x"},
		{"/tmp/x", "", "/tmp/x"},
		{"/tmp/x", ".", "/tmp/x"},
	} {
		if got, err := repoRoot(c.wd, c.src); err != nil || got != c.want {
			t.Errorf("repoRoot(%q, %q) = %q, %v; want %q", c.wd, c.src, got, err, c.want)
		}
	}
	if _, err := repoRoot("/tmp/x/other", "charts/web"); err == nil {
		t.Error("want error when the working directory doesn't match the source path")
	}
}

func TestNewPatchPostRenderer(t *testing.T) {
	if r := newPatchPostRenderer(nil); r != nil {
		t.Errorf("no patches: got %#v, want nil so Helm skips post-rendering", r)
	}
	if r := newPatchPostRenderer([]types.Patch{priorityPatch}); r == nil {
		t.Error("with patches: got nil")
	}
}

// An empty render is valid, with or without patches.
func TestRenderHelmEmptyChart(t *testing.T) {
	t.Chdir(t.TempDir())
	writeFiles(t, map[string]string{
		"chart/Chart.yaml":        "apiVersion: v2\nname: empty\nversion: 0.1.0\n",
		"chart/templates/cm.yaml": "{{- if .Values.enabled }}\napiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: cm\n{{- end }}\n",
	})

	for name, patches := range map[string][]types.Patch{
		"no patches":   nil,
		"with patches": {priorityPatch},
	} {
		out, err := tryRender(&Options{Path: "chart", ReleaseName: "demo", Namespace: "demo-ns", Patches: patches})
		if err != nil {
			t.Errorf("%s: %v", name, err)
		}
		if strings.TrimSpace(out) != "" {
			t.Errorf("%s: got output %q, want none", name, out)
		}
	}
}

// writeFiles writes files, keyed by path, under the working directory.
func writeFiles(t *testing.T, files map[string]string) {
	t.Helper()
	for name, content := range files {
		if err := os.MkdirAll(filepath.Dir(name), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(name, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestRenderHelmCRDsAndHooks(t *testing.T) {
	nodes := render(t, &Options{
		Path:        example(t, "helm"),
		ReleaseName: "demo",
		Namespace:   "demo-ns",
		Values:      []string{"migrations.enabled=true"},
		Patches:     []types.Patch{priorityPatch},
	})
	find(t, nodes, "CustomResourceDefinition", "widgets.example.com")
	job := find(t, nodes, "Job", "demo-migrate")
	if got := job.GetAnnotations()["helm.sh/hook"]; got != "pre-install,pre-upgrade" {
		t.Errorf("hook annotation = %q", got)
	}
	if got := field(t, find(t, nodes, "Deployment", "demo-web"), "spec", "template", "spec", "priorityClassName"); got != "high" {
		t.Errorf("priorityClassName = %q, want high", got)
	}
}

func TestRenderHelmCapabilities(t *testing.T) {
	t.Chdir(t.TempDir())
	writeFiles(t, map[string]string{
		"chart/Chart.yaml": "apiVersion: v2\nname: caps\nversion: 0.1.0\nkubeVersion: \">=1.40.0-0\"\n",
		"chart/templates/cm.yaml": `apiVersion: v1
kind: ConfigMap
metadata:
  name: caps
data:
  kube: {{ .Capabilities.KubeVersion.Version | quote }}
  widgets: {{ .Capabilities.APIVersions.Has "example.com/v1" | quote }}
`,
	})
	o := &Options{Path: "chart", ReleaseName: "demo", Namespace: "demo-ns"}
	if _, err := tryRender(o); err == nil || !strings.Contains(err.Error(), "kubeVersion") {
		t.Errorf("default capabilities: err = %v, want kubeVersion constraint failure", err)
	}

	o.KubeVersion = "1.40.2"
	o.APIVersions = []string{"example.com/v1", "apps/v1"}
	cm := find(t, render(t, o), "ConfigMap", "caps")
	if got := field(t, cm, "data", "kube"); got != "v1.40.2" {
		t.Errorf("kube = %q, want v1.40.2", got)
	}
	if got := field(t, cm, "data", "widgets"); got != "true" {
		t.Errorf("widgets = %q, want true", got)
	}

	o.KubeVersion = "not-a-version"
	if _, err := New(o); err == nil || !strings.Contains(err.Error(), "kube version") {
		t.Errorf("invalid kube version: err = %v", err)
	}
}

func TestRenderFilesExistingKustomization(t *testing.T) {
	dir := example(t, "manifests")
	writeFiles(t, map[string]string{filepath.Join(dir, "kustomization.yaml"): "resources: [deployment.yaml]\n"})
	_, err := New(&Options{Path: dir, Files: []string{"deployment.yaml"}})
	if err == nil || !strings.Contains(err.Error(), "already has a kustomization.yaml") {
		t.Errorf("err = %v, want existing kustomization error", err)
	}
}

func TestRenderHelmSkipCRDs(t *testing.T) {
	nodes := render(t, &Options{Path: example(t, "helm"), ReleaseName: "demo", Namespace: "demo-ns", SkipCRDs: true})
	for _, n := range nodes {
		if n.GetKind() == "CustomResourceDefinition" {
			t.Errorf("got CRD %s with skipCrds", n.GetName())
		}
	}
	find(t, nodes, "Deployment", "demo-web")
}
