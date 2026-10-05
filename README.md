# argocd-cmp-helm-kustomize

An Argo CD Config Management Plugin. It renders a Helm chart, or a set of plain manifest files, and then applies kustomize patches, so no overlay directories need to live in git.

## Configuration

Configure the plugin with the Application's plugin parameters:

```yaml
source:
  repoURL: https://github.com/acme/deploy
  path: charts/web
  plugin:
    name: helm-kustomize
    parameters:
      - name: path
        string: .
      - name: releaseName
        string: web
      - name: namespace
        string: prod
      - name: valueFiles
        array: [../../envs/prod/values.yaml]
      - name: values
        array: [image.tag=1.4.2]
      - name: patches
        string: |
          - target: {kind: Deployment, name: web}
            patch: |
              - op: add
                path: /spec/template/spec/priorityClassName
                value: high
```

| Parameter | Type | Description |
|---|---|---|
| `path` | string | Chart directory, or the kustomization root when `files` is set |
| `releaseName` | string | Helm release name. Required unless `files` is set |
| `namespace` | string | Helm release namespace. Required unless `files` is set |
| `valueFiles` | array | Helm values files |
| `values` | array | Helm `--set` entries (`key=value`) |
| `skipCrds` | string | `"true"` to leave out the chart's `crds/` directory, for example when another Application manages the CRDs. Default `"false"` |
| `files` | array | Manifest files relative to `path`. When set, the plugin runs kustomize over these files instead of rendering a chart |
| `patches` | string | YAML list of [kustomize patches](https://kubectl.docs.kubernetes.io/references/kustomize/kustomization/patches/) |

`path` and `valueFiles` are relative to the Application's source path. They can point anywhere in the repository but nowhere outside it, including through symlinks. Value files are always read locally and never fetched from URLs. Unknown parameter names are rejected.

See [examples/applications](examples/applications) for complete Applications.

## How it works

- **Helm:** the chart at `path` is rendered client-side with the Helm v4 SDK, the same way `helm template` does it. The output includes the chart's `crds/` and its hooks, and Argo CD turns `helm.sh/hook` annotations into sync hooks. `.Capabilities` reflects the target cluster through the `KUBE_VERSION` and `KUBE_API_VERSIONS` values Argo CD provides. If any `patches` are set, a Helm post-renderer applies them to the whole chart, hooks included, through a kustomization in an in-memory filesystem.
- **Files:** a `kustomization.yaml` listing `files` as resources, plus the `patches`, is written into `path` and built with kustomize. Argo CD gives plugins a throwaway copy of the repo, so this doesn't touch git. If `path` already contains a kustomization file, the plugin returns an error instead of replacing it.

Everything runs in a single static Go binary. It doesn't call the `helm` or `kustomize` binaries.

## Install

Add the sidecar to `argocd-repo-server` in one of these ways:

- **Kustomize:** use [deploy/repo-server-patch.yaml](deploy/repo-server-patch.yaml), or the complete example in [deploy/kustomization.yaml](deploy/kustomization.yaml).
- **argo-cd Helm chart:** use [deploy/argo-cd-helm-values.yaml](deploy/argo-cd-helm-values.yaml).

Images are published to `ghcr.io/xorps/argocd-cmp-helm-kustomize` for each `vX.Y.Z` tag as `X.Y.Z` and `X.Y`, and for `main` as `latest`. Applications must set `spec.source.plugin.name: helm-kustomize`.

## Development

Tool versions are pinned in [.tool-versions](.tool-versions):

```sh
asdf plugin add golang
asdf plugin add golangci-lint
asdf install

make check   # gofmt, go vet, golangci-lint, go mod tidy, tests (same as the PR checks)
make build   # bin/cmp-helm-kustomize
make image   # docker build
```

An end-to-end test, also run in CI, starts a kind cluster, installs Argo CD with the sidecar, and syncs [examples/applications](examples/applications). Argo CD clones the repository, so the revision under test must be pushed:

```sh
REVISION=$(git rev-parse HEAD) test/e2e/run.sh   # needs docker, kind and kubectl
```

Example run:

```sh
cd examples/helm
ARGOCD_APP_PARAMETERS='[{"name":"path","string":"."},{"name":"releaseName","string":"demo"},
  {"name":"namespace","string":"demo"},{"name":"values","array":["replicaCount=3"]}]' \
  ../../bin/cmp-helm-kustomize
```

## License

Licensed under either of

- Apache License, Version 2.0 ([LICENSE-APACHE](LICENSE-APACHE))
- MIT License ([LICENSE-MIT](LICENSE-MIT))

at your option.

Unless you explicitly state otherwise, any contribution you intentionally submit for inclusion in this project, as defined in the Apache-2.0 license, is dual licensed as above, without any additional terms or conditions.
