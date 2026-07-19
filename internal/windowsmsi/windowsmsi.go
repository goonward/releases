// Package windowsmsi builds a Go+-specific MSI with the WiX packaging flow
// used by the official Go release builder.
package windowsmsi

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"text/template"
	"time"
)

type Options struct {
	Arch    string
	Version string
}

// Build constructs an MSI that embeds the already-compiled payload directory.
func Build(ctx context.Context, workDir, payloadDir, outputPath string, options Options) error {
	source, err := installerSource(options)
	if err != nil {
		return err
	}
	if err := validatePayload(payloadDir); err != nil {
		return err
	}
	wixArch, _, err := architecture(options.Arch)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		return fmt.Errorf("create MSI work directory: %w", err)
	}
	wixDir := filepath.Join(workDir, "wix")
	if err := installWix(wixFor(options.Arch), wixDir); err != nil {
		return err
	}
	installerPath := filepath.Join(workDir, "installer.wxs")
	if err := os.WriteFile(installerPath, []byte(source), 0o644); err != nil {
		return fmt.Errorf("write installer source: %w", err)
	}
	if err := os.WriteFile(filepath.Join(workDir, "LICENSE.rtf"), []byte(licenseRTF), 0o644); err != nil {
		return fmt.Errorf("write installer license: %w", err)
	}

	appFiles := filepath.Join(workDir, "AppFiles.wxs")
	if err := run(ctx, workDir, filepath.Join(wixDir, "heat.exe"),
		"dir", payloadDir,
		"-nologo", "-gg", "-g1", "-srd", "-sfrag", "-sreg",
		"-cg", "AppFiles", "-template", "fragment", "-dr", "INSTALLDIR",
		"-var", "var.SourceDir", "-out", appFiles,
	); err != nil {
		return fmt.Errorf("gather payload files: %w", err)
	}
	if err := run(ctx, workDir, filepath.Join(wixDir, "candle.exe"),
		"-nologo", "-arch", wixArch, "-dSourceDir="+payloadDir, installerPath, appFiles,
	); err != nil {
		return fmt.Errorf("compile installer: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return fmt.Errorf("create MSI output directory: %w", err)
	}
	if err := run(ctx, workDir, filepath.Join(wixDir, "light.exe"),
		"-nologo", "-dcl:high", "-ext", "WixUIExtension", "-ext", "WixUtilExtension",
		filepath.Join(workDir, "AppFiles.wixobj"), filepath.Join(workDir, "installer.wixobj"),
		"-o", outputPath,
	); err != nil {
		return fmt.Errorf("link installer: %w", err)
	}
	return nil
}

func validatePayload(root string) error {
	required := []string{
		".goplus-managed",
		filepath.Join("go", "bin", "go.exe"),
		filepath.Join("bin", "go+.exe"),
		filepath.Join("bin", "gofmt+.exe"),
		filepath.Join("bin", "gopls+.exe"),
		filepath.Join("bin", "goimports+.exe"),
	}
	for _, name := range required {
		info, err := os.Stat(filepath.Join(root, name))
		if err != nil {
			return fmt.Errorf("payload missing %s: %w", name, err)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("payload entry %s is not a regular file", name)
		}
	}
	return nil
}

var versionPattern = regexp.MustCompile(`^v?(\d+)\.(\d+)\.(\d+)(?:[-+][0-9A-Za-z.-]+)?$`)

func packageVersion(version string) (string, error) {
	match := versionPattern.FindStringSubmatch(version)
	if match == nil {
		return "", fmt.Errorf("invalid release version %q", version)
	}
	parts := make([]int, 3)
	for index := range parts {
		value, err := strconv.Atoi(match[index+1])
		if err != nil {
			return "", fmt.Errorf("parse release version: %w", err)
		}
		parts[index] = value
	}
	if parts[0] > 255 || parts[1] > 255 || parts[2] > 65535 {
		return "", fmt.Errorf("release version %q exceeds MSI limits", version)
	}
	return fmt.Sprintf("%d.%d.%d", parts[0], parts[1], parts[2]), nil
}

func architecture(arch string) (wixArch, upgradeCode string, err error) {
	switch arch {
	case "amd64":
		return "x64", "{9D6193D2-A216-4D2E-8E1A-94268BB80DF1}", nil
	case "arm64":
		return "arm64", "{A15B02DF-3F9F-43A1-9C15-2E0DCA8E1027}", nil
	default:
		return "", "", fmt.Errorf("unsupported Windows architecture %q", arch)
	}
}

func installerSource(options Options) (string, error) {
	packageVersion, err := packageVersion(options.Version)
	if err != nil {
		return "", err
	}
	_, upgradeCode, err := architecture(options.Arch)
	if err != nil {
		return "", err
	}
	componentCode := "{277BCA0C-AF73-4FC7-A5D4-FAD8E0E32770}"
	if options.Arch == "arm64" {
		componentCode = "{E879884C-A771-4A48-B6C6-377105344905}"
	}
	data := struct {
		Arch          string
		Display       string
		Package       string
		UpgradeCode   string
		ComponentCode string
	}{
		Arch:          html.EscapeString(options.Arch),
		Display:       html.EscapeString(options.Version),
		Package:       packageVersion,
		UpgradeCode:   upgradeCode,
		ComponentCode: componentCode,
	}
	var output strings.Builder
	if err := installerTemplate.Execute(&output, data); err != nil {
		return "", fmt.Errorf("render installer source: %w", err)
	}
	return output.String(), nil
}

var installerTemplate = template.Must(template.New("installer").Parse(`<?xml version="1.0" encoding="UTF-8"?>
<Wix xmlns="http://schemas.microsoft.com/wix/2006/wi">
  <Product Id="*"
      Name="Go+ {{.Arch}} {{.Display}}"
      Language="1033"
      Version="{{.Package}}"
      Manufacturer="goonward"
      UpgradeCode="{{.UpgradeCode}}">
    <Package Id="*"
        Description="Go+ programming language installer"
        InstallerVersion="500"
        Compressed="yes"
        InstallScope="perMachine"
        Languages="1033" />
    <MajorUpgrade AllowDowngrades="yes" />
    <MediaTemplate EmbedCab="yes" CompressionLevel="high" />

    <Property Id="ARPHELPLINK" Value="https://github.com/goonward/goplus" />
    <Property Id="ARPURLINFOABOUT" Value="https://github.com/goonward" />
    <Property Id="WIXUI_INSTALLDIR" Value="INSTALLDIR" />

    <Directory Id="TARGETDIR" Name="SourceDir">
      <Directory Id="ProgramFiles64Folder">
        <Directory Id="INSTALLDIR" Name="GoPlus" />
      </Directory>
    </Directory>

    <DirectoryRef Id="INSTALLDIR">
      <Component Id="GoPlusEnvironment" Guid="{{.ComponentCode}}" Win64="yes">
        <RegistryKey Root="HKLM" Key="Software\goonward\GoPlus">
          <RegistryValue Name="InstallLocation" Type="string" Value="[INSTALLDIR]" KeyPath="yes" />
        </RegistryKey>
        <Environment Id="GoPlusPath"
            Action="set"
            Part="last"
            Name="PATH"
            Permanent="no"
            System="yes"
            Value="[INSTALLDIR]bin" />
      </Component>
    </DirectoryRef>

    <Feature Id="GoPlusTools" Title="Go+" Level="1">
      <ComponentGroupRef Id="AppFiles" />
      <ComponentRef Id="GoPlusEnvironment" />
    </Feature>

    <CustomActionRef Id="WixBroadcastEnvironmentChange" />
    <WixVariable Id="WixUILicenseRtf" Value="LICENSE.rtf" />
    <UIRef Id="WixUI_InstallDir" />
  </Product>
</Wix>
`))

type wixRelease struct {
	url    string
	sha256 string
}

var (
	wix311 = wixRelease{
		url:    "https://storage.googleapis.com/go-builder-data/wix311-binaries.zip",
		sha256: "da034c489bd1dd6d8e1623675bf5e899f32d74d6d8312f8dd125a084543193de",
	}
	wix314 = wixRelease{
		url:    "https://storage.googleapis.com/go-builder-data/wix314-binaries.zip",
		sha256: "34dcbba9952902bfb710161bd45ee2e721ffa878db99f738285a21c9b09c6edb",
	}
)

func wixFor(arch string) wixRelease {
	if arch == "arm64" {
		return wix314
	}
	return wix311
}

func installWix(release wixRelease, destination string) error {
	if _, err := os.Stat(filepath.Join(destination, "heat.exe")); err == nil {
		return nil
	}
	client := &http.Client{Timeout: 5 * time.Minute}
	response, err := client.Get(release.url)
	if err != nil {
		return fmt.Errorf("download WiX: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("download WiX: %s", response.Status)
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return fmt.Errorf("read WiX archive: %w", err)
	}
	if got := fmt.Sprintf("%x", sha256.Sum256(body)); got != release.sha256 {
		return fmt.Errorf("WiX SHA-256 mismatch: got %s", got)
	}
	archive, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		return fmt.Errorf("open WiX archive: %w", err)
	}
	for _, entry := range archive.File {
		name := filepath.Clean(filepath.FromSlash(entry.Name))
		if filepath.IsAbs(name) || name == ".." || strings.HasPrefix(name, ".."+string(filepath.Separator)) {
			return fmt.Errorf("unsafe WiX archive path %q", entry.Name)
		}
		target := filepath.Join(destination, name)
		if entry.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return fmt.Errorf("create WiX directory: %w", err)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return fmt.Errorf("create WiX parent: %w", err)
		}
		reader, err := entry.Open()
		if err != nil {
			return fmt.Errorf("open WiX archive entry: %w", err)
		}
		file, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
		if err != nil {
			reader.Close()
			return fmt.Errorf("create WiX file: %w", err)
		}
		_, copyErr := io.Copy(file, reader)
		readerCloseErr := reader.Close()
		fileCloseErr := file.Close()
		if copyErr != nil {
			return fmt.Errorf("extract WiX file: %w", copyErr)
		}
		if readerCloseErr != nil || fileCloseErr != nil {
			return errors.Join(readerCloseErr, fileCloseErr)
		}
	}
	return nil
}

func run(ctx context.Context, directory, name string, args ...string) error {
	command := exec.CommandContext(ctx, name, args...)
	command.Dir = directory
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	fmt.Printf("$ %s %s\n", name, strings.Join(args, " "))
	return command.Run()
}

const licenseRTF = `{\rtf1\ansi\deff0
{\fonttbl{\f0 Consolas;}}
\f0\fs18
Copyright (c) 2009 The Go Authors. All rights reserved.\par
\par
Redistribution and use in source and binary forms, with or without modification, are permitted provided that the following conditions are met:\par
\par
* Redistributions of source code must retain the above copyright notice, this list of conditions and the following disclaimer.\par
* Redistributions in binary form must reproduce the above copyright notice, this list of conditions and the following disclaimer in the documentation and/or other materials provided with the distribution.\par
* Neither the name of Google LLC nor the names of its contributors may be used to endorse or promote products derived from this software without specific prior written permission.\par
\par
THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE DISCLAIMED.
}`
