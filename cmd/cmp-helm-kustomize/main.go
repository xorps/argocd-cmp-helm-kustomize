// Command cmp-helm-kustomize is an Argo CD config management plugin.
package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/xorps/argocd-cmp-helm-kustomize/internal/plugin"
)

func main() {
	if err := run(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, "cmp-helm-kustomize:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	r, err := plugin.NewFromEnv()
	if err != nil {
		return err
	}
	out, err := r.Render(ctx)
	if err != nil {
		return err
	}
	_, err = io.WriteString(os.Stdout, out)
	return err
}
