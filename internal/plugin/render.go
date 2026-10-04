// Package plugin renders Helm charts or plain manifests with kustomize patches.
package plugin

import "context"

// Renderer renders an Application's manifests.
type Renderer interface {
	Render(ctx context.Context) (string, error)
}

// New returns a Renderer for o: kustomize if Files is set, otherwise Helm.
func New(o *Options) (Renderer, error) {
	if len(o.Files) > 0 {
		return newFilesRenderer(o)
	}
	return newHelmRenderer(o)
}

// NewFromEnv returns a Renderer configured from the Argo CD environment.
func NewFromEnv() (Renderer, error) {
	o, err := newOptionsFromEnv()
	if err != nil {
		return nil, err
	}
	return New(o)
}
