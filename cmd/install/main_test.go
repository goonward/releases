package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestAssetName(t *testing.T) {
	tests := []struct {
		goos    string
		goarch  string
		want    string
		wantErr bool
	}{
		{goos: "linux", goarch: "amd64", want: "goplus-linux-amd64.tar.gz"},
		{goos: "linux", goarch: "arm64", wantErr: true},
		{goos: "windows", goarch: "amd64", wantErr: true},
		{goos: "linux", goarch: "386", wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.goos+"/"+test.goarch, func(t *testing.T) {
			got, err := assetName(test.goos, test.goarch)
			if (err != nil) != test.wantErr {
				t.Fatalf("assetName() error = %v, wantErr %v", err, test.wantErr)
			}
			if got != test.want {
				t.Errorf("assetName() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestDownload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.WriteHeader(http.StatusOK)
		_, _ = response.Write([]byte("release"))
	}))
	defer server.Close()
	destination := filepath.Join(t.TempDir(), "asset")
	if err := download(context.Background(), server.Client(), server.URL, destination); err != nil {
		t.Fatalf("download() error = %v", err)
	}
	contents, err := os.ReadFile(destination)
	if err != nil {
		t.Fatalf("ReadFile(): %v", err)
	}
	if string(contents) != "release" {
		t.Errorf("downloaded contents = %q, want %q", contents, "release")
	}
}

func TestDownloadRejectsHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		http.Error(response, "no release", http.StatusNotFound)
	}))
	defer server.Close()
	if err := download(context.Background(), server.Client(), server.URL, filepath.Join(t.TempDir(), "asset")); err == nil {
		t.Fatal("download() accepted an HTTP error")
	}
}
