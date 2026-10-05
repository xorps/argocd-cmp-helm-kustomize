// SPDX-License-Identifier: Apache-2.0 OR MIT

package plugin

import (
	"cmp"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// confine returns an error if any of paths, relative to the working
// directory, resolves outside root, including through symlinks. field is
// used in error messages.
func confine(root, field string, paths ...string) error {
	rootDir, err := filepath.Abs(cmp.Or(root, "."))
	if err != nil {
		return err
	}
	r, err := os.OpenRoot(rootDir)
	if err != nil {
		return err
	}
	defer r.Close()

	wd, err := os.Getwd()
	if err != nil {
		return err
	}
	for _, p := range paths {
		if !filepath.IsAbs(p) {
			p = filepath.Join(wd, p)
		}
		rel, err := filepath.Rel(rootDir, p)
		if err == nil {
			_, err = r.Stat(rel)
		}
		if err != nil {
			return fmt.Errorf("%s: %w", field, err)
		}
	}
	return nil
}

// repoRoot returns wd with the ARGOCD_APP_SOURCE_PATH suffix removed.
func repoRoot(wd, sourcePath string) (string, error) {
	src := filepath.Clean(sourcePath)
	if src == "." || src == string(filepath.Separator) {
		return wd, nil
	}
	root, ok := strings.CutSuffix(filepath.Clean(wd), string(filepath.Separator)+src)
	if !ok {
		return "", fmt.Errorf("working directory %s does not end in source path %s", wd, sourcePath)
	}
	return root, nil
}
