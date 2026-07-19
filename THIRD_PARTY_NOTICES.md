# Third-party notices

The Windows MSI builder follows the WiX packaging design used by the Go
project's `golang.org/x/build/internal/installer/windowsmsi` package. The
adapted implementation and the Go+ fork are distributed under the BSD license
in [`LICENSE`](LICENSE).

The patched `golang.org/x/tools` and `gopls` sources included in release
artifacts retain their upstream copyright and BSD license notices. WiX binaries
are downloaded from the checksummed archives used by Go's release builder and
retain their own upstream license.
