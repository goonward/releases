#!/usr/bin/env bash

set -euo pipefail

if (( $# != 5 )); then
	echo "usage: $0 SOURCE_ROOT OUTPUT_DIR VERSION TOOLS_REF GOPLS_REF" >&2
	exit 2
fi

canonical_directory() {
	(CDPATH= cd -- "$1" && pwd -P)
}

source_root=$(canonical_directory "$1")
output_dir=$2
version=$3
tools_ref=$4
gopls_ref=$5
release_root=$(canonical_directory "$(dirname -- "${BASH_SOURCE[0]}")/..")

case $(uname -s) in
Linux) goos=linux ;;
Darwin) goos=darwin ;;
*)
	echo "unsupported operating system: $(uname -s)" >&2
	exit 1
	;;
esac

case $(uname -m) in
x86_64) goarch=amd64 ;;
arm64 | aarch64) goarch=arm64 ;;
*)
	echo "unsupported architecture: $(uname -m)" >&2
	exit 1
	;;
esac

case $goos/$goarch in
linux/amd64 | darwin/amd64 | darwin/arm64) ;;
*)
	echo "unsupported platform: $goos/$goarch" >&2
	exit 1
	;;
esac

for command in bash cc git tar uname; do
	command -v "$command" >/dev/null 2>&1 || {
		echo "missing required command: $command" >&2
		exit 1
	}
done

mkdir -p "$output_dir"
output_dir=$(canonical_directory "$output_dir")
work=$(mktemp -d)
trap 'rm -rf -- "$work"' EXIT
stage=$work/payload
mkdir -p "$stage/go" "$stage/bin" "$stage/libexec"
printf 'Go+ %s managed installation\n' "$version" >"$stage/.goplus-managed"

echo "Building Go+"
(cd "$source_root/src" && ./make.bash)
test -x "$source_root/bin/go"
test -x "$source_root/bin/gofmt"
version_file=$source_root/VERSION.cache
if [[ ! -f $version_file ]]; then
	version_file=$source_root/VERSION
fi
test -f "$version_file"

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
cp -a "$version_file" "$stage/go/VERSION"

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
test "$("$private_go" env GOHOSTOS)" = "$goos"
test "$("$private_go" env GOHOSTARCH)" = "$goarch"
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

archive=$output_dir/goplus-$goos-$goarch.tar.gz
archive_tool=tar
if [[ $goos == darwin ]] && command -v gtar >/dev/null 2>&1; then
	archive_tool=gtar
fi
if "$archive_tool" --version 2>/dev/null | grep -q 'GNU tar'; then
	COPYFILE_DISABLE=1 "$archive_tool" --sort=name --mtime="@${SOURCE_DATE_EPOCH:-0}" --owner=0 --group=0 --numeric-owner \
		-C "$stage" -czf "$archive" .goplus-managed bin go libexec
else
	COPYFILE_DISABLE=1 "$archive_tool" -C "$stage" -czf "$archive" .goplus-managed bin go libexec
fi
(cd "$release_root" && "$private_go" build -trimpath -ldflags="-s -w -X main.releaseTag=$version" \
	-o "$output_dir/goplus-install-$goos-$goarch" ./cmd/install)
installer=$output_dir/goplus-install-$goos-$goarch
"$installer" -version

echo "Testing the packaged installation"
"$installer" -archive "$archive" -prefix "$work/installed" -bin-dir "$work/bin"
"$work/bin/go+" version
