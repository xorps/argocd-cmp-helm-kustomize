package plugin

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/yaml"
)

// Options configures a Renderer.
type Options struct {
	Path        string
	ReleaseName string
	Namespace   string
	Patches     []types.Patch
	ValueFiles  []string
	Values      []string
	Files       []string
	SkipCRDs    bool

	// Root is the directory Path and ValueFiles must stay within.
	// Defaults to the working directory.
	Root string

	KubeVersion string
	APIVersions []string
}

// Environment variables Argo CD sets for plugins.
//
// see https://argo-cd.readthedocs.io/en/stable/user-guide/build-environment/
const (
	envParameters  = "ARGOCD_APP_PARAMETERS"
	envSourcePath  = "ARGOCD_APP_SOURCE_PATH"
	envKubeVersion = "KUBE_VERSION"
	envAPIVersions = "KUBE_API_VERSIONS"
)

func newOptionsFromEnv() (*Options, error) {
	o, err := parseParameters(os.Getenv(envParameters))
	if err != nil {
		return nil, err
	}

	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	if o.Root, err = repoRoot(wd, os.Getenv(envSourcePath)); err != nil {
		return nil, err
	}

	o.KubeVersion = os.Getenv(envKubeVersion)
	if v := os.Getenv(envAPIVersions); v != "" {
		o.APIVersions = strings.Split(v, ",")
	}
	return o, nil
}

// parameter is an entry in ARGOCD_APP_PARAMETERS.
type parameter struct {
	Name   string   `json:"name"`
	String string   `json:"string"`
	Array  []string `json:"array"`
}

// parses ARGOCD_APP_PARAMETERS.
// Unknown parameters are an error.
func parseParameters(raw string) (*Options, error) {
	o := &Options{}
	if strings.TrimSpace(raw) == "" {
		return o, nil
	}
	var params []parameter
	if err := json.Unmarshal([]byte(raw), &params); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", envParameters, err)
	}
	for _, p := range params {
		switch p.Name {
		case "path":
			o.Path = p.String
		case "releaseName":
			o.ReleaseName = p.String
		case "namespace":
			o.Namespace = p.String
		case "valueFiles":
			o.ValueFiles = p.Array
		case "values":
			o.Values = p.Array
		case "files":
			o.Files = p.Array
		case "skipCrds":
			if p.String == "" { // unset in the UI
				continue
			}
			b, err := strconv.ParseBool(p.String)
			if err != nil {
				return nil, fmt.Errorf("parameter skipCrds: want true or false, got %q", p.String)
			}
			o.SkipCRDs = b
		case "patches":
			if err := yaml.UnmarshalStrict([]byte(p.String), &o.Patches); err != nil {
				return nil, fmt.Errorf("parameter patches: want a YAML list of kustomize patches: %w", err)
			}
		default:
			return nil, fmt.Errorf("unknown parameter %q", p.Name)
		}
	}
	return o, nil
}
