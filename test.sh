#!/bin/sh

assertKptSourceNamesEqual() {
	[ "$(kpt fn source "$1" | grep -E '^    name: ' | grep -Eo '[^ ]+$')" = "$2" ]
}

set -ex

EXAMPLE=examples/hello-world
rm -f examples/hello-world/deploy/generated.yaml
for i in 1 2; do
	kpt fn run $EXAMPLE
	# TODO: preserve kustomize order
	assertKptSourceNamesEqual $EXAMPLE/deploy 'dev-hello-world
dev-hello-config-h7872h2b6h
inventory'
done
