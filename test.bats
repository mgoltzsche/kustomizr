#!/usr/bin/env bats

# ARGS: MANIFEST_DIR EXPECTED_NAME...
assertKptSourceNamesEqual() {
	RESOURCE_NAMES="$(cat $1 | grep -E '^  name: ' | grep -Eo '[^ ]+$')" || true
	([ "$RESOURCE_NAMES" ] || (echo "FAIL: no resources found in generated manifest $1" >&2; false)) &&
	shift || return 1
	if echo "$1" | grep -Eq '^[0-9]+$'; then
		RESOURCE_COUNT="$(echo "$RESOURCE_NAMES" | wc -w)" &&
		[ "$RESOURCE_COUNT" -eq "$1" ] || (echo "FAIL: generated manifest contains $RESOURCE_COUNT resources but expected $1" >&2; false)
	else
		EXPECTED_NAMES="$(echo "$@" | xargs -n1 echo)" &&
		NAMEDIFF="$(comm -3 --nocheck-order <(echo "$EXPECTED_NAMES") <(echo "$RESOURCE_NAMES"))" &&
		([ ! "$NAMEDIFF" ] || (printf 'FAIL: Unexpected resources names appeared within generated output:\n%s\n\nActual names:\n%s\n\nExpected names:\n%s\n' "$NAMEDIFF" "$RESOURCE_NAMES" "$EXPECTED_NAMES" >&2; false))
	fi
}

# ARGS: EXAMPLE_NAME FN_OPTS EXPECTED_RESOURCE_NAME...
buildExample() {
	EXAMPLE=examples/$1
	FN_OPTS="$2"
	shift 2
	rm -f $EXAMPLE/static/generated-manifest.yaml || return 1
	for i in 1st 2nd; do
		echo Run function for the $i time
		kpt fn run $FN_OPTS $EXAMPLE &&
		assertKptSourceNamesEqual $EXAMPLE/static/generated-manifest.yaml "$@" || return 1
	done
}

@test "build hello-world example" {
	buildExample hello-world --network=false \
		dev-hello-world \
		dev-hello-config-d2842mm7gh
}

@test "build ingress-nginx example" {
	buildExample ingress-nginx --network=true 27
}

@test "build list-resource example" {
	buildExample list-resource --network=false myconfig
}
