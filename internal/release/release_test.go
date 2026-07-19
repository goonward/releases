package release

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestAssetURL(t *testing.T) {
	tests := []struct {
		name    string
		tag     string
		asset   string
		want    string
		wantErr bool
	}{
		{name: "release", tag: "v0.1.0-alpha.1", asset: "goplus-linux-amd64.tar.gz", want: "https://github.com/goonward/releases/releases/download/v0.1.0-alpha.1/goplus-linux-amd64.tar.gz"},
		{name: "development", tag: "dev", asset: "archive", wantErr: true},
		{name: "tag traversal", tag: "v0.1/escape", asset: "archive", wantErr: true},
		{name: "asset traversal", tag: "v0.1.0", asset: "../archive", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := AssetURL(test.tag, test.asset)
			if (err != nil) != test.wantErr {
				t.Fatalf("AssetURL() error = %v, wantErr %v", err, test.wantErr)
			}
			if got != test.want {
				t.Errorf("AssetURL() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestParseChecksums(t *testing.T) {
	const first = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	const second = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	checksums, err := ParseChecksums([]byte(first + "  first.tar.gz\n" + second + " *second.msi\n"))
	if err != nil {
		t.Fatalf("ParseChecksums() error = %v", err)
	}
	if checksums["first.tar.gz"] != first || checksums["second.msi"] != second {
		t.Errorf("ParseChecksums() = %v", checksums)
	}
	if _, err := ParseChecksums([]byte("not-a-checksum  archive\n")); err == nil {
		t.Fatal("ParseChecksums() accepted an invalid checksum")
	}
}

func TestVerifyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "archive")
	if err := os.WriteFile(path, []byte("payload"), 0o644); err != nil {
		t.Fatalf("WriteFile(): %v", err)
	}
	want := fmt.Sprintf("%x", sha256.Sum256([]byte("payload")))
	if err := VerifyFile(path, want); err != nil {
		t.Fatalf("VerifyFile() error = %v", err)
	}
	if err := VerifyFile(path, fmt.Sprintf("%064x", 0)); err == nil {
		t.Fatal("VerifyFile() accepted the wrong digest")
	}
}
