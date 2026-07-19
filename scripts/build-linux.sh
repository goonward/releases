#!/usr/bin/env bash

set -euo pipefail

if (( $# != 5 )); then
	echo "usage: $0 SOURCE_ROOT OUTPUT_DIR VERSION TOOLS_REF GOPLS_REF" >&2
	exit 2
fi

source_root=$(realpath -- "$1")
output_dir=$2
version=$3
tools_ref=$4
gopls_ref=$5
release_root=$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd -P)

for command in bash cc git tar; do
	command -v "$command" >/dev/null 2>&1 || {
		echo "missing required command: $command" >&2
		exit 1
	}
done

mkdir -p "$output_dir"
output_dir=$(realpath -- "$output_dir")
work=$(mktemp -d)
trap 'rm -rf -- "$work"' EXIT
stage=$work/payload
mkdir -p "$stage/go" "$stage/bin" "$stage/libexec"
printf 'Go+ %s managed installation\n' "$version" >"$stage/.goplus-managed"

echo "Building Go+"
(cd "$source_root/src" && ./make.bash)
test -x "$source_root/bin/go"
test -x "$source_root/bin/gofmt"
test -f "$source_root/VERSION.cache"

for directory in api bin doc lib misc src test; do
	cp -a "$source_root/$directory" "$stage/go/"
done
mkdir -p "$stage/go/pkg"
cp -a "$source_root/pkg/include" "$source_root/pkg/tool" "$stage/go/pkg/"
for file in CONTRIBUTING.md LICENSE PATENTS README.md SECURITY.md codereview.cfg go.env; do
	if [[ -f $source_root/$file ]]; then
		cp -a "$source_root/$file" "$stage/go/$file"
	fi
done
cp -a "$source_root/VERSION.cache" "$stage/go/VERSION"

echo "Building patched goimports and gopls"
git clone --quiet --filter=blob:none --no-checkout https://go.googlesource.com/tools "$work/tools"
git -C "$work/tools" checkout --quiet "$tools_ref"
git -C "$work/tools" apply "$release_root/patches/x-tools.patch"

git clone --quiet --filter=blob:none --no-checkout https://go.googlesource.com/tools "$work/gopls-repo"
git -C "$work/gopls-repo" checkout --quiet "$gopls_ref"
git -C "$work/gopls-repo" apply --directory=gopls "$release_root/patches/gopls.patch"

private_go=$stage/go/bin/go
export GOROOT=$stage/go
export GOTOOLCHAIN=local
export GOWORK=off
(cd "$work/tools" && "$private_go" build -trimpath -o "$stage/libexec/goimports" ./cmd/goimports)
(cd "$work/gopls-repo/gopls" && "$private_go" mod edit -replace="golang.org/x/tools=$work/tools")
(cd "$work/gopls-repo/gopls" && "$private_go" build -trimpath -o "$stage/libexec/gopls" .)

echo "Building Go+ launchers"
(cd "$release_root" && "$private_go" build -trimpath -o "$stage/bin/goplus-launcher" ./cmd/launcher)
for name in go+ gofmt+ gopls+ goimports+; do
	cp "$stage/bin/goplus-launcher" "$stage/bin/$name"
done
rm "$stage/bin/goplus-launcher"

"$stage/bin/go+" version
"$stage/bin/gopls+" version

archive=$output_dir/goplus-linux-amd64.tar.gz
tar --sort=name --mtime="@${SOURCE_DATE_EPOCH:-0}" --owner=0 --group=0 --numeric-owner \
	-C "$stage" -czf "$archive" .goplus-managed bin go libexec
(cd "$release_root" && "$private_go" build -trimpath -ldflags="-s -w -X main.releaseTag=$version" \
	-o "$output_dir/goplus-install-linux-amd64" ./cmd/install)
"$output_dir/goplus-install-linux-amd64" -version
