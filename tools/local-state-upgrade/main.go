package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/MoeForge/moego/ojson"
)

func main() {
	flags := flag.NewFlagSet("local-state-upgrade", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	source := flags.String("source", "", "Read-only source credential-login.db")
	authDir := flags.String("auth-dir", "", "Existing local credential directory")
	destination := flags.String("destination", "", "New credential-login.db, must not exist")
	qualityState := flags.String("quality-state", "", "Read-only host quality-state.db")
	qualityDB := flags.String("quality-db", "", "Read-only plugin quality.db")
	if err := flags.Parse(os.Args[1:]); err != nil || flags.NArg() != 0 || *source == "" || *authDir == "" || *destination == "" || (*qualityState == "") != (*qualityDB == "") {
		fmt.Fprintln(os.Stderr, "usage: local-state-upgrade --source DATABASE --auth-dir DIRECTORY --destination NEW_DATABASE [--quality-state HOST_DATABASE --quality-db PLUGIN_DATABASE]")
		os.Exit(2)
	}
	report, err := upgrade(options{Source: *source, AuthDir: *authDir, Destination: *destination, QualityState: *qualityState, QualityDB: *qualityDB})
	if err != nil {
		// Errors are fixed labels; never print paths, account identities or database values.
		fmt.Fprintln(os.Stderr, "upgrade_failed:", err)
		os.Exit(1)
	}
	raw, err := reportBytes(report)
	if err != nil {
		os.Exit(1)
	}
	if _, err := os.Stdout.Write(append(raw, '\n')); err != nil {
		os.Exit(1)
	}
}

func reportBytes(value report) ([]byte, error) {
	doc := ojson.NewDocString(`{}`)
	if err := doc.Set("report", value); err != nil {
		return nil, err
	}
	return []byte(ojson.ParseBytes(doc.Bytes()).Get("report").Raw), nil
}
