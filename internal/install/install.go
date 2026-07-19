// Package install installs a precompiled Go+ payload without replacing
// directories that are not managed by Go+.
package install

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

const ManagedMarker = ".goplus-managed"

var Commands = []string{"go+", "gofmt+", "gopls+", "goimports+"}

// Archive atomically installs a Go+ payload archive at prefix.
func Archive(archivePath, prefix string) error {
	prefix, err := safePrefix(prefix)
	if err != nil {
		return err
	}
	if err := validateExisting(prefix); err != nil {
		return err
	}
	parent := filepath.Dir(prefix)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return fmt.Errorf("create install parent: %w", err)
	}
	stage, err := os.MkdirTemp(parent, ".goplus-install-")
	if err != nil {
		return fmt.Errorf("create staging directory: %w", err)
	}
	defer os.RemoveAll(stage)

	if err := extract(archivePath, stage); err != nil {
		return err
	}
	if err := validatePayload(stage); err != nil {
		return err
	}

	backup := fmt.Sprintf("%s.backup.%d", prefix, os.Getpid())
	if _, err := os.Lstat(backup); err == nil {
		return fmt.Errorf("backup path already exists: %s", backup)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect backup path: %w", err)
	}

	hadExisting := false
	if _, err := os.Lstat(prefix); err == nil {
		if err := os.Rename(prefix, backup); err != nil {
			return fmt.Errorf("back up existing installation: %w", err)
		}
		hadExisting = true
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect existing installation: %w", err)
	}
	if err := os.Rename(stage, prefix); err != nil {
		if hadExisting {
			_ = os.Rename(backup, prefix)
		}
		return fmt.Errorf("activate installation: %w", err)
	}
	if hadExisting {
		if err := os.RemoveAll(backup); err != nil {
			return fmt.Errorf("remove previous installation: %w", err)
		}
	}
	return nil
}

// Links exposes the Go+ commands from prefix in binDir.
func Links(prefix, binDir string) error {
	prefix, err := filepath.Abs(prefix)
	if err != nil {
		return fmt.Errorf("resolve install prefix: %w", err)
	}
	binDir, err = filepath.Abs(binDir)
	if err != nil {
		return fmt.Errorf("resolve command directory: %w", err)
	}
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return fmt.Errorf("create command directory: %w", err)
	}

	for _, name := range Commands {
		source := filepath.Join(prefix, "bin", name)
		if info, err := os.Stat(source); err != nil || !info.Mode().IsRegular() {
			if err == nil {
				err = fmt.Errorf("not a regular file")
			}
			return fmt.Errorf("inspect %s: %w", source, err)
		}
		destination := filepath.Join(binDir, name)
		info, err := os.Lstat(destination)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return fmt.Errorf("inspect %s: %w", destination, err)
		}
		if info.Mode()&os.ModeSymlink == 0 {
			return fmt.Errorf("refusing to replace non-symbolic link %s", destination)
		}
		target, err := os.Readlink(destination)
		if err != nil {
			return fmt.Errorf("read %s: %w", destination, err)
		}
		if target != source {
			return fmt.Errorf("refusing to replace symbolic link %s to %s", destination, target)
		}
	}

	for _, name := range Commands {
		source := filepath.Join(prefix, "bin", name)
		destination := filepath.Join(binDir, name)
		temporary := fmt.Sprintf("%s.goplus.%d", destination, os.Getpid())
		if err := os.Symlink(source, temporary); err != nil {
			return fmt.Errorf("create command link: %w", err)
		}
		if err := os.Rename(temporary, destination); err != nil {
			_ = os.Remove(temporary)
			return fmt.Errorf("activate command link: %w", err)
		}
	}
	return nil
}

func safePrefix(prefix string) (string, error) {
	if prefix == "" {
		return "", errors.New("install prefix is empty")
	}
	absolute, err := filepath.Abs(prefix)
	if err != nil {
		return "", fmt.Errorf("resolve install prefix: %w", err)
	}
	absolute = filepath.Clean(absolute)
	if absolute == filepath.VolumeName(absolute)+string(filepath.Separator) {
		return "", fmt.Errorf("refusing unsafe install prefix %q", absolute)
	}
	if home, err := os.UserHomeDir(); err == nil && absolute == filepath.Clean(home) {
		return "", fmt.Errorf("refusing unsafe install prefix %q", absolute)
	}
	return absolute, nil
}

func validateExisting(prefix string) error {
	info, err := os.Lstat(prefix)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect install prefix: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("install prefix is not a regular directory: %s", prefix)
	}
	entries, err := os.ReadDir(prefix)
	if err != nil {
		return fmt.Errorf("read install prefix: %w", err)
	}
	if len(entries) == 0 {
		return nil
	}
	if info, err := os.Stat(filepath.Join(prefix, ManagedMarker)); err != nil || !info.Mode().IsRegular() {
		return fmt.Errorf("refusing to replace unmanaged non-empty directory %s", prefix)
	}
	return nil
}

func validatePayload(root string) error {
	required := []string{ManagedMarker, filepath.Join("go", "bin", "go")}
	for _, name := range Commands {
		required = append(required, filepath.Join("bin", name))
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

func extract(archivePath, destination string) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("open payload: %w", err)
	}
	defer file.Close()
	gz, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("open compressed payload: %w", err)
	}
	defer gz.Close()

	reader := tar.NewReader(gz)
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("read payload: %w", err)
		}
		if err := extractEntry(reader, header, destination); err != nil {
			return err
		}
	}
	return nil
}

func extractEntry(reader io.Reader, header *tar.Header, destination string) error {
	if strings.Contains(header.Name, `\`) {
		return fmt.Errorf("unsafe payload path %q", header.Name)
	}
	name := path.Clean(header.Name)
	if name == "." || path.IsAbs(name) || name == ".." || strings.HasPrefix(name, "../") {
		return fmt.Errorf("unsafe payload path %q", header.Name)
	}
	target := filepath.Join(destination, filepath.FromSlash(name))
	relative, err := filepath.Rel(destination, target)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return fmt.Errorf("unsafe payload path %q", header.Name)
	}

	switch header.Typeflag {
	case tar.TypeDir:
		if err := os.MkdirAll(target, 0o755); err != nil {
			return fmt.Errorf("create payload directory: %w", err)
		}
		return nil
	case tar.TypeReg, tar.TypeRegA:
	default:
		return fmt.Errorf("unsupported payload entry %q", header.Name)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return fmt.Errorf("create payload parent: %w", err)
	}
	mode := os.FileMode(header.Mode).Perm()
	mode &^= 0o022
	file, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if err != nil {
		return fmt.Errorf("create payload entry: %w", err)
	}
	_, copyErr := io.Copy(file, reader)
	closeErr := file.Close()
	if copyErr != nil {
		return fmt.Errorf("extract payload entry: %w", copyErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close payload entry: %w", closeErr)
	}
	return nil
}
