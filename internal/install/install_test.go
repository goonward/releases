package install

import (
	"archive/tar"
	"compress/gzip"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestArchive(t *testing.T) {
	tests := []struct {
		name    string
		prepare func(*testing.T, string)
		wantErr bool
	}{
		{name: "fresh install"},
		{
			name: "managed upgrade",
			prepare: func(t *testing.T, prefix string) {
				t.Helper()
				writeFile(t, filepath.Join(prefix, ManagedMarker), "managed")
				writeFile(t, filepath.Join(prefix, "old"), "old")
			},
		},
		{
			name: "unmanaged directory",
			prepare: func(t *testing.T, prefix string) {
				t.Helper()
				writeFile(t, filepath.Join(prefix, "keep"), "user data")
			},
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			prefix := filepath.Join(root, "goplus")
			if test.prepare != nil {
				test.prepare(t, prefix)
			}
			archive := filepath.Join(root, "goplus.tar.gz")
			writeArchive(t, archive, validPayload())

			err := Archive(archive, prefix)
			if (err != nil) != test.wantErr {
				t.Fatalf("Archive() error = %v, wantErr %v", err, test.wantErr)
			}
			if test.wantErr {
				return
			}
			if got := readFile(t, filepath.Join(prefix, "bin", "go+")); got != "go+" {
				t.Errorf("installed go+ = %q, want %q", got, "go+")
			}
			if _, err := os.Stat(filepath.Join(prefix, "old")); !os.IsNotExist(err) {
				t.Errorf("old installation file still exists")
			}
		})
	}
}

func TestArchiveRejectsUnsafeEntries(t *testing.T) {
	tests := []struct {
		name string
		file archiveFile
	}{
		{name: "parent traversal", file: archiveFile{name: "../escape", body: "bad"}},
		{name: "nested traversal", file: archiveFile{name: "root/../../escape", body: "bad"}},
		{name: "absolute path", file: archiveFile{name: "/escape", body: "bad"}},
		{name: "symbolic link", file: archiveFile{name: "bin/go+", mode: 0o777, typeflag: tar.TypeSymlink, linkname: "/tmp/escape"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			archive := filepath.Join(root, "unsafe.tar.gz")
			writeArchive(t, archive, []archiveFile{test.file})
			if err := Archive(archive, filepath.Join(root, "install")); err == nil {
				t.Fatal("Archive() succeeded for unsafe entry")
			}
		})
	}
}

func TestLinks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symbolic link behavior is platform-specific")
	}
	prefix := filepath.Join(t.TempDir(), "goplus")
	for _, name := range Commands {
		writeFile(t, filepath.Join(prefix, "bin", name), name)
	}
	binDir := filepath.Join(t.TempDir(), "bin")
	if err := Links(prefix, binDir); err != nil {
		t.Fatalf("Links() error = %v", err)
	}
	for _, name := range Commands {
		target, err := os.Readlink(filepath.Join(binDir, name))
		if err != nil {
			t.Errorf("Readlink(%q): %v", name, err)
			continue
		}
		want := filepath.Join(prefix, "bin", name)
		if target != want {
			t.Errorf("link %q target = %q, want %q", name, target, want)
		}
	}
}

func TestLinksRefusesRegularFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symbolic link behavior is platform-specific")
	}
	prefix := filepath.Join(t.TempDir(), "goplus")
	for _, name := range Commands {
		writeFile(t, filepath.Join(prefix, "bin", name), name)
	}
	binDir := filepath.Join(t.TempDir(), "bin")
	writeFile(t, filepath.Join(binDir, "go+"), "keep")
	if err := Links(prefix, binDir); err == nil {
		t.Fatal("Links() replaced a regular file")
	}
	if got := readFile(t, filepath.Join(binDir, "go+")); got != "keep" {
		t.Errorf("regular file = %q, want %q", got, "keep")
	}
}

type archiveFile struct {
	name     string
	body     string
	mode     int64
	typeflag byte
	linkname string
}

func validPayload() []archiveFile {
	files := []archiveFile{{name: ManagedMarker, body: "managed", mode: 0o644}}
	for _, name := range Commands {
		files = append(files, archiveFile{name: filepath.ToSlash(filepath.Join("bin", name)), body: name, mode: 0o755})
	}
	files = append(files, archiveFile{name: "go/bin/go", body: "go", mode: 0o755})
	return files
}

func writeArchive(t *testing.T, path string, files []archiveFile) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(): %v", err)
	}
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("Create(): %v", err)
	}
	gz := gzip.NewWriter(file)
	tw := tar.NewWriter(gz)
	for _, entry := range files {
		mode := entry.mode
		if mode == 0 {
			mode = 0o644
		}
		typeflag := entry.typeflag
		if typeflag == 0 {
			typeflag = tar.TypeReg
		}
		header := &tar.Header{Name: entry.name, Mode: mode, Size: int64(len(entry.body)), Typeflag: typeflag, Linkname: entry.linkname}
		if typeflag != tar.TypeReg {
			header.Size = 0
		}
		if err := tw.WriteHeader(header); err != nil {
			t.Fatalf("WriteHeader(): %v", err)
		}
		if header.Size > 0 {
			if _, err := tw.Write([]byte(entry.body)); err != nil {
				t.Fatalf("Write(): %v", err)
			}
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("tar Close(): %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("gzip Close(): %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("file Close(): %v", err)
	}
}

func writeFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(): %v", err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o755); err != nil {
		t.Fatalf("WriteFile(): %v", err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(): %v", err)
	}
	return string(contents)
}
