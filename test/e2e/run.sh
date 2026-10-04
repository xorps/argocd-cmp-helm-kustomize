#!/usr/bin/env bash
# Syncs the example Applications through the plugin in a kind cluster.
# REVISION must be pushed to REPO, since Argo CD clones it.
set -euo pipefail

REPO=${REPO:-https://github.com/xorps/argocd-cmp-helm-kustomize}
REVISION=${REVISION:-$(git rev-parse HEAD)}
CLUSTER=${CLUSTER:-cmp-e2e}
APPS=(web-prod web-files)

cd "$(dirname "$0")/../.."

if ! kind get clusters | grep -qx "$CLUSTER"; then
  kind create cluster --name "$CLUSTER" --wait 2m
fi
kubectl config use-context "kind-$CLUSTER"

make image TAG=e2e
kind load docker-image ghcr.io/xorps/argocd-cmp-helm-kustomize:e2e --name "$CLUSTER"

kubectl create namespace argocd --dry-run=client -o yaml | kubectl apply -f -
kubectl apply --server-side --force-conflicts -k test/e2e
kubectl -n argocd rollout status deploy/argocd-repo-server --timeout=5m
kubectl -n argocd rollout status statefulset/argocd-application-controller --timeout=5m

for f in examples/applications/*.yaml; do
  sed -e "s|repoURL: .*|repoURL: $REPO|" -e "s|targetRevision: .*|targetRevision: $REVISION|" "$f" |
    kubectl apply -f -
done
for app in "${APPS[@]}"; do
  kubectl -n argocd patch application "$app" --type merge -p '{"spec":{"syncPolicy":{"automated":{}}}}'
done

dump() {
  kubectl -n argocd get applications -o yaml
  kubectl -n argocd logs deploy/argocd-repo-server -c cmp-helm-kustomize --tail=200 || true
}
for app in "${APPS[@]}"; do
  if ! kubectl -n argocd wait "application/$app" --for=jsonpath='{.status.health.status}'=Healthy --timeout=5m ||
    ! kubectl -n argocd wait "application/$app" --for=jsonpath='{.status.sync.status}'=Synced --timeout=2m; then
    dump
    exit 1
  fi
done

failed=0
check() { # name actual expected
  if [[ "$2" == "$3" ]]; then
    echo "ok   $1"
  else
    echo "FAIL $1: got '$2', want '$3'"
    failed=1
  fi
}
check "helm: CRD from crds/" \
  "$(kubectl get crd widgets.example.com -o jsonpath='{.metadata.name}')" "widgets.example.com"
check "helm: valueFiles" \
  "$(kubectl -n web-prod get deploy web-web -o jsonpath='{.spec.replicas}')" "2"
check "helm: values" \
  "$(kubectl -n web-prod get deploy web-web -o jsonpath='{.spec.template.spec.containers[0].image}')" "nginx:1.27.3"
check "helm: patches" \
  "$(kubectl -n web-prod get deploy web-web -o jsonpath='{.spec.template.metadata.annotations.example\.com/patched}')" "true"
check "helm: hook ran as a sync hook" \
  "$(kubectl -n argocd get application web-prod -o jsonpath='{.status.operationState.syncResult.resources[?(@.kind=="Job")].hookPhase}')" "Succeeded"
check "files: patches" \
  "$(kubectl -n web-files get deploy web -o jsonpath='{.spec.replicas}')" "3"

if [[ $failed -ne 0 ]]; then
  dump
  exit 1
fi
echo "e2e passed (delete the cluster with: kind delete cluster --name $CLUSTER)"
