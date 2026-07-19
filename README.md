# Go+ releases

This repository builds precompiled distributions for
[`goonward/goplus`](https://github.com/goonward/goplus). The language fork stays
focused on Go source; installers, patched editor tools, and release automation
live here.

## Install

Linux amd64:

```sh
version=v0.1.0-alpha.1
curl -fLO "https://github.com/goonward/releases/releases/download/$version/goplus-install-linux-amd64"
chmod +x goplus-install-linux-amd64
./goplus-install-linux-amd64
```

The CLI verifies the payload against the release's `SHA256SUMS`, installs it
under `~/.local/share/goplus`, and links `go+`, `gofmt+`, `gopls+`, and
`goimports+` into `~/.local/bin`.

On Windows amd64, download `goplus-windows-amd64.msi` from the same release.
It is a normal WiX MSI, like Go's official installer, and installs the already
compiled toolchain under `Program Files\GoPlus`. It does not compile Go+ during
installation. A portable zip is published beside it.

## Use

Use `go+` anywhere you would use `go`. The editor integrations use `gopls+`.

```go
type Decision enum {
	Allow
	Deny(reason string)
}

try user := loadUser()

defer {
	cleanup()
}
```

## Build

Pinned source revisions are in [`versions.env`](versions.env). The release
workflow builds Linux and Windows payloads independently, tests the packaging
tools, and only publishes assets for a pushed version tag.
