// Command goplus-msi packages a precompiled Windows Go+ payload as an MSI.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/goonward/releases/internal/windowsmsi"
)

type options struct {
	payload string
	output  string
	work    string
	version string
	arch    string
}

func main() {
	log.SetFlags(0)
	options, err := parseOptions(os.Args[1:])
	if err != nil {
		log.Fatal("goplus-msi: ", err)
	}
	if err := windowsmsi.Build(context.Background(), options.work, options.payload, options.output, windowsmsi.Options{
		Arch:    options.arch,
		Version: options.version,
	}); err != nil {
		log.Fatal("goplus-msi: ", err)
	}
}

func parseOptions(arguments []string) (options, error) {
	var result options
	flags := flag.NewFlagSet("goplus-msi", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.StringVar(&result.payload, "payload", "", "precompiled Windows payload directory")
	flags.StringVar(&result.output, "out", "", "output MSI path")
	flags.StringVar(&result.work, "work", filepath.Join(os.TempDir(), "goplus-msi"), "MSI work directory")
	flags.StringVar(&result.version, "version", "", "Go+ release version")
	flags.StringVar(&result.arch, "arch", "amd64", "Windows architecture")
	if err := flags.Parse(arguments); err != nil {
		return options{}, err
	}
	var missing []error
	if result.payload == "" {
		missing = append(missing, errors.New("-payload is required"))
	}
	if result.output == "" {
		missing = append(missing, errors.New("-out is required"))
	}
	if result.version == "" {
		missing = append(missing, errors.New("-version is required"))
	}
	if err := errors.Join(missing...); err != nil {
		return options{}, fmt.Errorf("invalid options: %w", err)
	}
	return result, nil
}
