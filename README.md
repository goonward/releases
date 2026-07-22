# Go+ releases

This repository builds precompiled distributions for
[`goonward/goplus`](https://github.com/goonward/goplus). The language fork stays
focused on Go source; installers, patched editor tools, and release automation
live here.

## Install

Linux amd64:

```sh
version=v0.2.1
curl -fLO "https://github.com/goonward/releases/releases/download/$version/goplus-install-linux-amd64"
chmod +x goplus-install-linux-amd64
./goplus-install-linux-amd64
```

The CLI verifies the payload against the release's `SHA256SUMS`, installs it
under `~/.local/share/goplus`, and links `go+`, `gofmt+`, `gopls+`, and
`goimports+` into `~/.local/bin`.

macOS (Apple Silicon):

```sh
version=v0.2.1
curl -fLO "https://github.com/goonward/releases/releases/download/$version/goplus-install-darwin-arm64"
chmod +x goplus-install-darwin-arm64
./goplus-install-darwin-arm64
```

Intel Macs use `goplus-install-darwin-amd64` instead. The macOS installers use
the same managed installation directory and command links as Linux. Make sure
`~/.local/bin` is on your `PATH`; with the default macOS shell, add
`export PATH="$HOME/.local/bin:$PATH"` to `~/.zprofile` if needed.

On Windows amd64, download `goplus-windows-amd64.msi` from the same release.
It is a normal WiX MSI, like Go's official installer, and installs the already
compiled toolchain under `Program Files\GoPlus`. It does not compile Go+ during
installation. A portable zip is published beside it.

## Use

Use `go+` anywhere you would use `go`. The editor integrations use `gopls+`.

```go
type Decision enum {
	Allow
	Deny { Reason string }
}

try user := loadUser()

defer {
	cleanup()
}
```

## Build

Pinned source revisions are in [`versions.env`](versions.env). Pull requests
build Linux, Windows, and macOS payloads as validation. To publish, run the
`Build release` workflow with a new `vX.Y.Z` or prerelease version; it creates
the tag and GitHub release only after every platform build succeeds, then
attaches the installers, portable archives, and `SHA256SUMS`.
