package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/superposition/kotoba-line/internal/importer"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "kotoba-import: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	var opts importer.Options
	var documentOut string
	var campaignOut string

	fs := flag.NewFlagSet("kotoba-import", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.StringVar(&opts.InputPath, "input", "", "local .md or .txt document to import")
	fs.StringVar(&documentOut, "document-out", "", "path to write source-preserving document JSON")
	fs.StringVar(&campaignOut, "campaign-out", "", "path to write campaign JSON")
	fs.StringVar(&opts.DocumentID, "document-id", "", "document id; defaults to input filename slug")
	fs.StringVar(&opts.CampaignID, "campaign-id", "", "campaign id; defaults to document id")
	fs.StringVar(&opts.TitleJA, "title-ja", "", "Japanese document title; defaults to first source title when present")
	fs.StringVar(&opts.TitleEN, "title-en", "", "English document title")
	fs.StringVar(&opts.CampaignTitleJA, "campaign-title-ja", "", "Japanese campaign title; defaults to document title")
	fs.StringVar(&opts.CampaignTitleEN, "campaign-title-en", "", "English campaign title; defaults to title-en")
	fs.StringVar(&opts.DocumentFixturePath, "document-fixture", "", "document fixture path to store in campaign JSON; defaults to document-out")
	fs.StringVar(&opts.SourceDate, "source-date", "", "source date metadata to store in document JSON")
	fs.StringVar(&opts.StationPrefix, "station-prefix", "", "station id prefix for generated levels; defaults to campaign id")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if opts.InputPath == "" && fs.NArg() == 1 {
		opts.InputPath = fs.Arg(0)
	}
	if opts.InputPath == "" {
		return errors.New("missing -input path")
	}
	if documentOut == "" {
		return errors.New("missing -document-out path")
	}
	if campaignOut == "" {
		return errors.New("missing -campaign-out path")
	}
	if opts.DocumentFixturePath == "" {
		opts.DocumentFixturePath = documentOut
	}

	output, err := importer.ImportFile(opts)
	if err != nil {
		return err
	}
	if err := writeJSONFile(documentOut, output.Document); err != nil {
		return fmt.Errorf("write document JSON: %w", err)
	}
	if err := writeJSONFile(campaignOut, output.Campaign); err != nil {
		return fmt.Errorf("write campaign JSON: %w", err)
	}

	fmt.Fprintf(os.Stderr, "imported %s: %d sections -> %s, %s\n", opts.InputPath, len(output.Document.Sections), documentOut, campaignOut)
	return nil
}

func writeJSONFile(path string, value any) error {
	if path == "" {
		return errors.New("output path is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return importer.WriteJSON(f, value)
}
