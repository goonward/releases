package windowsmsi

import (
	"encoding/xml"
	"strings"
	"testing"
)

func TestPackageVersion(t *testing.T) {
	tests := []struct {
		version string
		want    string
		wantErr bool
	}{
		{version: "v0.1.0-alpha.1", want: "0.1.0"},
		{version: "v1.2.345", want: "1.2.345"},
		{version: "1.4.2", want: "1.4.2"},
		{version: "dev", wantErr: true},
		{version: "v256.0.0", wantErr: true},
		{version: "v1.256.0", wantErr: true},
		{version: "v1.2.65536", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.version, func(t *testing.T) {
			got, err := packageVersion(test.version)
			if (err != nil) != test.wantErr {
				t.Fatalf("packageVersion() error = %v, wantErr %v", err, test.wantErr)
			}
			if got != test.want {
				t.Errorf("packageVersion() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestInstallerSource(t *testing.T) {
	source, err := installerSource(Options{Arch: "amd64", Version: "v0.1.0-alpha.1"})
	if err != nil {
		t.Fatalf("installerSource() error = %v", err)
	}
	var document struct {
		XMLName xml.Name `xml:"Wix"`
	}
	if err := xml.Unmarshal([]byte(source), &document); err != nil {
		t.Fatalf("installer source is not valid XML: %v", err)
	}
	for _, want := range []string{
		`Name="Go+ amd64 v0.1.0-alpha.1"`,
		`Version="0.1.0"`,
		`Name="GoPlus"`,
		`Value="[INSTALLDIR]bin"`,
		`Key="Software\goonward\GoPlus"`,
	} {
		if !strings.Contains(source, want) {
			t.Errorf("installer source missing %q", want)
		}
	}
	for _, unwanted := range []string{"GoProgrammingLanguage", `Name="Go"`} {
		if strings.Contains(source, unwanted) {
			t.Errorf("installer source contains official Go identity %q", unwanted)
		}
	}
}

func TestArchitecture(t *testing.T) {
	tests := []struct {
		arch    string
		wantWix string
		wantErr bool
	}{
		{arch: "amd64", wantWix: "x64"},
		{arch: "arm64", wantWix: "arm64"},
		{arch: "386", wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.arch, func(t *testing.T) {
			got, _, err := architecture(test.arch)
			if (err != nil) != test.wantErr {
				t.Fatalf("architecture() error = %v, wantErr %v", err, test.wantErr)
			}
			if got != test.wantWix {
				t.Errorf("architecture() = %q, want %q", got, test.wantWix)
			}
		})
	}
}
