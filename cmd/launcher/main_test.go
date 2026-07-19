// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestTargetFor(t *testing.T) {
	tests := []struct {
		name string
		goos string
		want []string
	}{
		{name: "go+", goos: "windows", want: []string{"go", "bin", "go.exe"}},
		{name: "gofmt+", goos: "windows", want: []string{"go", "bin", "gofmt.exe"}},
		{name: "gopls+", goos: "windows", want: []string{"libexec", "gopls.exe"}},
		{name: "goimports+", goos: "windows", want: []string{"libexec", "goimports.exe"}},
		{name: "go+", goos: "darwin", want: []string{"go", "bin", "go"}},
		{name: "gofmt+", goos: "darwin", want: []string{"go", "bin", "gofmt"}},
		{name: "gopls+", goos: "darwin", want: []string{"libexec", "gopls"}},
		{name: "goimports+", goos: "darwin", want: []string{"libexec", "goimports"}},
	}
	for _, test := range tests {
		t.Run(test.goos+"/"+test.name, func(t *testing.T) {
			got, ok := targetFor(test.name, test.goos)
			if !ok || !slices.Equal(got, test.want) {
				t.Errorf("targetFor(%q, %q) = %v, %v; want %v, true", test.name, test.goos, got, ok, test.want)
			}
		})
	}
	if _, ok := targetFor("unknown", "windows"); ok {
		t.Error("targetFor(unknown) succeeded")
	}
}

func TestToolEnv(t *testing.T) {
	t.Setenv("PATH", "old-path")
	got := toolEnv([]string{"GOROOT=old", "Path=old-path", "GOTOOLCHAIN=auto", "KEEP=yes"}, "new-root")
	wantPath := "PATH=" + filepath.Join("new-root", "bin") + string(os.PathListSeparator) + "old-path"
	for _, want := range []string{"KEEP=yes", "GOROOT=new-root", wantPath, "GOTOOLCHAIN=local"} {
		if !slices.Contains(got, want) {
			t.Errorf("toolEnv() = %v, missing %q", got, want)
		}
	}
}
