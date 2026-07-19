package main

import "testing"

func TestParseOptions(t *testing.T) {
	options, err := parseOptions([]string{"-payload", "payload", "-out", "goplus.msi", "-version", "v0.1.0", "-arch", "amd64"})
	if err != nil {
		t.Fatalf("parseOptions() error = %v", err)
	}
	if options.payload != "payload" || options.output != "goplus.msi" || options.version != "v0.1.0" || options.arch != "amd64" {
		t.Errorf("parseOptions() = %+v", options)
	}
}

func TestParseOptionsRequiresInputs(t *testing.T) {
	for _, args := range [][]string{
		nil,
		{"-out", "goplus.msi", "-version", "v0.1.0"},
		{"-payload", "payload", "-version", "v0.1.0"},
		{"-payload", "payload", "-out", "goplus.msi"},
	} {
		if _, err := parseOptions(args); err == nil {
			t.Errorf("parseOptions(%v) succeeded", args)
		}
	}
}
