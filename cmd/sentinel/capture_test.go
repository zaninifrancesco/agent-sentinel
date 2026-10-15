package main

import (
	"flag"
	"io"
	"strings"
	"testing"
)

func TestCaptureModeRejectsEnforcementOptions(t *testing.T) {
	for _, args := range [][]string{
		{"--policy", "off"}, {"--policy", "rules.json"},
		{"--max-cost", "0"}, {"--approval-timeout", "30s"},
		{"--mode", "enforce"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			fs := flag.NewFlagSet("test", flag.ContinueOnError)
			fs.SetOutput(io.Discard)
			capture := addCaptureFlags(fs)
			if err := fs.Parse(args); err != nil {
				t.Fatal(err)
			}
			if err := capture.validate(fs); err == nil {
				t.Fatal("enforcement option silently accepted")
			}
			if code := runServe(args); code != 2 {
				t.Fatalf("serve exit = %d, want migration error", code)
			}
			if code := runMCP(append(append([]string{}, args...), "--", "unused-server")); code != 2 {
				t.Fatalf("mcp exit = %d, want migration error", code)
			}
		})
	}
}

func TestCaptureDefaultsToObservation(t *testing.T) {
	for _, args := range [][]string{nil, {"--mode", "observe"}} {
		fs := flag.NewFlagSet("test", flag.ContinueOnError)
		capture := addCaptureFlags(fs)
		if err := fs.Parse(args); err != nil {
			t.Fatal(err)
		}
		if err := capture.validate(fs); err != nil {
			t.Fatal(err)
		}
	}
}

func TestRecorderProfileFlagsValidateBeforeStarting(t *testing.T) {
	for _, args := range [][]string{{"--capture", "unknown"}, {"--max-payload-bytes", "1"}, {"--max-payload-bytes", "-1"}} {
		if runServe(args) != 2 || runMCP(append(append([]string{}, args...), "--", "unused-server")) != 2 {
			t.Fatalf("invalid capture settings accepted: %v", args)
		}
	}
	for _, profile := range []string{"standard", "full"} {
		fs := flag.NewFlagSet("test", flag.ContinueOnError)
		flags := addRecorderFlags(fs)
		if err := fs.Parse([]string{"--capture", profile, "--max-payload-bytes", "2048"}); err != nil {
			t.Fatal(err)
		}
		settings, err := flags.settings()
		if err != nil || settings.Profile != profile || settings.MaxPayloadBytes != 2048 {
			t.Fatalf("bad profile config: %+v %v", settings, err)
		}
	}
}
