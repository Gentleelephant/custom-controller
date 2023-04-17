#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

# corresponding to go mod init <module>
MODULE=github.com/Gentleelephant/custom-controller
# api package
APIS_PKG=pkg/apis
# generated output package
OUTPUT_PKG=pkg/client
# group-version such as foo:v1alpha1
GROUP_VERSION="foo:v1alpha1 policy:v1 work:v1"

SCRIPT_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
#CODEGEN_PKG=${CODEGEN_PKG:-$(cd "${SCRIPT_ROOT}"; ls -d -1 ./vendor/k8s.io/code-generator 2>/dev/null || echo ../code-generator)}


echo  "update codegen==>MODULE:${MODULE}"
echo "update codegen==>APIS_PKG:${APIS_PKG}"
echo "update codegen==>OUTPUT_PKG:${OUTPUT_PKG}"
echo "update codegen==>GROUP_VERSION:${GROUP_VERSION}"

# generate the code with:
# --output-base    because this script should also be able to run inside the vendor dir of
#                  k8s.io/kubernetes. The output-base is needed for the generators to output into the vendor dir
#                  instead of the $GOPATH directly. For normal projects this can be dropped.
#bash "${CODEGEN_PKG}"/generate-groups.sh all \
bash hack/generate-internal-groups.sh client,informer,lister \
${MODULE}/${OUTPUT_PKG} \
${MODULE}/${APIS_PKG} \
${MODULE}/${APIS_PKG} \
${GROUP_VERSION} \
--go-header-file "${SCRIPT_ROOT}"/hack/boilerplate.go.txt \
--output-base "$(dirname "${BASH_SOURCE[0]}")/../../../.."
#  --output-base "./"
# To use your own boilerplate text append:
#   --go-header-file "${SCRIPT_ROOT}"/hack/custom-boilerplate.go.txt