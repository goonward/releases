// Package release locates and verifies Go+ release assets.
package release

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"strings"
)

const repository = "goonward/releases"

func AssetURL(tag, asset string) (string, error) {
	if tag == "" || tag == "dev" || url.PathEscape(tag) != tag {
		return "", fmt.Errorf("invalid release tag %q", tag)
	}
	if asset == "" || path.Base(asset) != asset || url.PathEscape(asset) != asset {
		return "", fmt.Errorf("invalid release asset %q", asset)
	}
	return fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", repository, tag, asset), nil
}

func ParseChecksums(contents []byte) (map[string]string, error) {
	checksums := make(map[string]string)
	scanner := bufio.NewScanner(strings.NewReader(string(contents)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 2 {
			return nil, fmt.Errorf("invalid checksum line %q", line)
		}
		digest := strings.ToLower(fields[0])
		decoded, err := hex.DecodeString(digest)
		if err != nil || len(decoded) != sha256.Size {
			return nil, fmt.Errorf("invalid checksum %q", fields[0])
		}
		name := strings.TrimPrefix(fields[1], "*")
		if name == "" || path.Base(name) != name {
			return nil, fmt.Errorf("invalid checksum filename %q", name)
		}
		if _, exists := checksums[name]; exists {
			return nil, fmt.Errorf("duplicate checksum for %s", name)
		}
		checksums[name] = digest
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read checksums: %w", err)
	}
	return checksums, nil
}

func VerifyFile(filename, want string) error {
	want = strings.ToLower(want)
	decoded, err := hex.DecodeString(want)
	if err != nil || len(decoded) != sha256.Size {
		return errors.New("invalid expected SHA-256 digest")
	}
	file, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("open %s: %w", filename, err)
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return fmt.Errorf("hash %s: %w", filename, err)
	}
	got := fmt.Sprintf("%x", hash.Sum(nil))
	if got != want {
		return fmt.Errorf("SHA-256 mismatch for %s: got %s, want %s", filename, got, want)
	}
	return nil
}
