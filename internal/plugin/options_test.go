// SPDX-License-Identifier: Apache-2.0 OR MIT

package plugin

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseParameters(t *testing.T) {
	o, err := parseParameters(`[
		{"name":"path","string":"chart"},
		{"name":"releaseName","string":"web"},
		{"name":"namespace","string":"prod"},
		{"name":"valueFiles","array":["a.yaml","b.yaml"]},
		{"name":"values","array":["image.tag=1"]},
		{"name":"files","array":["x.yaml"]},
		{"name":"skipCrds","string":"true"},
		{"name":"patches","string":"- target: {kind: Deployment}\n  patch: |\n    - op: add\n      path: /spec/replicas\n      value: 3\n"}
	]`)
	if err != nil {
		t.Fatal(err)
	}
	if o.Path != "chart" || o.ReleaseName != "web" || o.Namespace != "prod" ||
		len(o.ValueFiles) != 2 || o.Values[0] != "image.tag=1" || o.Files[0] != "x.yaml" ||
		!o.SkipCRDs || len(o.Patches) != 1 || o.Patches[0].Target.Kind != "Deployment" {
		t.Errorf("unexpected options: %+v", o)
	}

	if o, err := parseParameters(`[{"name":"skipCrds","string":""}]`); err != nil || o.SkipCRDs {
		t.Errorf("empty skipCrds: got %v, %v; want default false", o, err)
	}

	for raw, want := range map[string]string{
		`{"path":"chart"}`: "parsing ARGOCD_APP_PARAMETERS",
		`[{"name":"skipCrds","string":"yes please"}]`: "want true or false",
		`[{"name":"image","string":"x"}]`:             `unknown parameter "image"`,
		`[{"name":"patches","string":"op: add"}]`:     "want a YAML list",
		`[{"name":"patches","string":"- op: add"}]`:   `unknown field "op"`,
	} {
		if _, err := parseParameters(raw); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("parseParameters(%s): err = %v, want containing %q", raw, err, want)
		}
	}
}

func TestNewOptionsFromEnv(t *testing.T) {
	repo := t.TempDir()
	app := filepath.Join(repo, "charts", "web")
	if err := os.MkdirAll(app, 0o750); err != nil {
		t.Fatal(err)
	}
	t.Chdir(app)
	t.Setenv(envParameters, `[{"name":"releaseName","string":"web"}]`)
	t.Setenv(envSourcePath, "charts/web")
	t.Setenv(envKubeVersion, "1.33.1")
	t.Setenv(envAPIVersions, "apps/v1,example.com/v1")

	o, err := newOptionsFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	// wd can differ from repo through symlinks (/var -> /private/var on macOS).
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Dir(filepath.Dir(wd)); o.Root != want {
		t.Errorf("Root = %q, want %q", o.Root, want)
	}
	if o.ReleaseName != "web" || o.KubeVersion != "1.33.1" ||
		len(o.APIVersions) != 2 || o.APIVersions[1] != "example.com/v1" {
		t.Errorf("unexpected options: %+v", o)
	}

	t.Setenv(envSourcePath, "somewhere/else")
	if _, err := newOptionsFromEnv(); err == nil {
		t.Error("want error when the working directory doesn't match ARGOCD_APP_SOURCE_PATH")
	}
}
