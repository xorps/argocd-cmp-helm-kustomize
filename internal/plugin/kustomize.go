package plugin

import (
	"bytes"
	"cmp"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"helm.sh/helm/v4/pkg/postrenderer"
	"sigs.k8s.io/kustomize/api/konfig"
	"sigs.k8s.io/kustomize/api/krusty"
	"sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/kustomize/kyaml/filesys"
	"sigs.k8s.io/yaml"
)

// filesRenderer kustomizes files under dir. It writes kustomization.yaml into
// dir, which is safe because Argo CD runs plugins in a throwaway checkout.
// Kustomize itself keeps files within dir.
type filesRenderer struct {
	dir     string
	files   []string
	patches []types.Patch
}

func newFilesRenderer(o *Options) (*filesRenderer, error) {
	switch {
	case o.ReleaseName != "":
		return nil, errors.New("releaseName can't be used with files")
	case o.Namespace != "":
		return nil, errors.New("namespace can't be used with files")
	case len(o.ValueFiles) > 0:
		return nil, errors.New("valueFiles can't be used with files")
	case len(o.Values) > 0:
		return nil, errors.New("values can't be used with files")
	case o.SkipCRDs:
		return nil, errors.New("skipCrds can't be used with files")
	}
	dir := cmp.Or(o.Path, ".")
	if err := confine(o.Root, "path", dir); err != nil {
		return nil, err
	}
	// Don't overwrite an existing kustomization.
	for _, name := range konfig.RecognizedKustomizationFileNames() {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			return nil, fmt.Errorf("path %s already has a %s; files can't be used there", dir, name)
		}
	}
	return &filesRenderer{dir: dir, files: o.Files, patches: o.Patches}, nil
}

func (f *filesRenderer) Render(context.Context) (string, error) {
	out, err := kustomize(filesys.MakeFsOnDisk(), f.dir, f.files, f.patches)
	return string(out), err
}

// patchPostRenderer is a Helm post-renderer that applies kustomize patches.
type patchPostRenderer []types.Patch

// newPatchPostRenderer returns nil if there are no patches. It must return a
// nil interface rather than a nil patchPostRenderer for Helm to skip
// post-rendering.
func newPatchPostRenderer(patches []types.Patch) postrenderer.PostRenderer {
	if len(patches) == 0 {
		return nil
	}
	return patchPostRenderer(patches)
}

func (p patchPostRenderer) Run(rendered *bytes.Buffer) (*bytes.Buffer, error) {
	fs := filesys.MakeFsInMemory()
	if err := fs.WriteFile("/rendered.yaml", rendered.Bytes()); err != nil {
		return nil, err
	}
	out, err := kustomize(fs, "/", []string{"rendered.yaml"}, p)
	if err != nil {
		return nil, err
	}
	return bytes.NewBuffer(out), nil
}

func kustomize(fs filesys.FileSystem, dir string, resources []string, patches []types.Patch) ([]byte, error) {
	k, err := yaml.Marshal(types.Kustomization{Resources: resources, Patches: patches})
	if err != nil {
		return nil, err
	}
	if err := fs.WriteFile(filepath.Join(dir, "kustomization.yaml"), k); err != nil {
		return nil, err
	}
	m, err := krusty.MakeKustomizer(krusty.MakeDefaultOptions()).Run(fs, dir)
	if err != nil {
		return nil, fmt.Errorf("kustomize build: %w", err)
	}
	return m.AsYaml()
}
