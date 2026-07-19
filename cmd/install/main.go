// Command goplus-install downloads, verifies, and installs a precompiled Go+
// release on Linux.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/goonward/releases/internal/install"
	projectrelease "github.com/goonward/releases/internal/release"
)

var releaseTag = "dev"

func main() {
	log.SetFlags(0)
	if err := run(context.Background()); err != nil {
		log.Fatal("goplus-install: ", err)
	}
}

func run(ctx context.Context) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("find home directory: %w", err)
	}
	prefix := flag.String("prefix", envOr("GOPLUS_PREFIX", filepath.Join(home, ".local", "share", "goplus")), "installation directory")
	binDir := flag.String("bin-dir", envOr("GOPLUS_BIN_DIR", filepath.Join(home, ".local", "bin")), "directory for Go+ command links")
	localArchive := flag.String("archive", "", "install a local payload archive instead of downloading")
	digest := flag.String("sha256", "", "expected SHA-256 for a local payload archive")
	noLinks := flag.Bool("no-links", false, "do not create command links")
	showVersion := flag.Bool("version", false, "print the installer version")
	flag.Parse()
	if *showVersion {
		fmt.Println(releaseTag)
		return nil
	}

	asset, err := assetName(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return err
	}
	archivePath := *localArchive
	if archivePath != "" {
		if *digest != "" {
			if err := projectrelease.VerifyFile(archivePath, *digest); err != nil {
				return err
			}
		}
	} else {
		archivePath, err = fetchRelease(ctx, asset)
		if err != nil {
			return err
		}
		defer os.RemoveAll(filepath.Dir(archivePath))
	}

	fmt.Printf("Installing Go+ %s in %s\n", releaseTag, *prefix)
	if err := install.Archive(archivePath, *prefix); err != nil {
		return err
	}
	if !*noLinks {
		if err := install.Links(*prefix, *binDir); err != nil {
			return err
		}
	}
	command := exec.Command(filepath.Join(*prefix, "bin", "go+"), "version")
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	if err := command.Run(); err != nil {
		return fmt.Errorf("verify installed go+: %w", err)
	}
	fmt.Printf("Go+ %s installed; commands are in %s\n", releaseTag, *binDir)
	return nil
}

func fetchRelease(ctx context.Context, asset string) (string, error) {
	archiveURL, err := projectrelease.AssetURL(releaseTag, asset)
	if err != nil {
		return "", err
	}
	checksumsURL, err := projectrelease.AssetURL(releaseTag, "SHA256SUMS")
	if err != nil {
		return "", err
	}
	work, err := os.MkdirTemp("", "goplus-download-")
	if err != nil {
		return "", fmt.Errorf("create download directory: %w", err)
	}
	keep := false
	defer func() {
		if !keep {
			_ = os.RemoveAll(work)
		}
	}()
	client := &http.Client{Timeout: 10 * time.Minute}
	checksumsPath := filepath.Join(work, "SHA256SUMS")
	if err := download(ctx, client, checksumsURL, checksumsPath); err != nil {
		return "", err
	}
	contents, err := os.ReadFile(checksumsPath)
	if err != nil {
		return "", fmt.Errorf("read release checksums: %w", err)
	}
	checksums, err := projectrelease.ParseChecksums(contents)
	if err != nil {
		return "", err
	}
	want, ok := checksums[asset]
	if !ok {
		return "", fmt.Errorf("release checksums do not contain %s", asset)
	}
	archivePath := filepath.Join(work, asset)
	if err := download(ctx, client, archiveURL, archivePath); err != nil {
		return "", err
	}
	if err := projectrelease.VerifyFile(archivePath, want); err != nil {
		return "", err
	}
	keep = true
	return archivePath, nil
}

func assetName(goos, goarch string) (string, error) {
	if goos != "linux" || goarch != "amd64" {
		return "", fmt.Errorf("unsupported platform %s/%s", goos, goarch)
	}
	return "goplus-linux-amd64.tar.gz", nil
}

func download(ctx context.Context, client *http.Client, sourceURL, destination string) (resultErr error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, sourceURL, nil)
	if err != nil {
		return fmt.Errorf("create download request: %w", err)
	}
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("download %s: %w", sourceURL, err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: %s", sourceURL, response.Status)
	}
	file, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("create download: %w", err)
	}
	defer func() {
		if resultErr != nil {
			_ = os.Remove(destination)
		}
	}()
	_, copyErr := io.Copy(file, response.Body)
	closeErr := file.Close()
	if copyErr != nil {
		return fmt.Errorf("save download: %w", copyErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close download: %w", closeErr)
	}
	return nil
}

func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
