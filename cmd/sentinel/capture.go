package main

import (
	"flag"
	"fmt"

	"github.com/zaninifrancesco/agent-sentinel/internal/recorder"
)

// captureFlags makes the change from enforcement to observation explicit.
// Removed options remain parseable only to return an actionable migration error.
type captureFlags struct {
	mode *string
}

type recorderFlags struct {
	profile *string
	limit   *int
}

func addRecorderFlags(fs *flag.FlagSet) recorderFlags {
	return recorderFlags{
		profile: fs.String("capture", "standard", "payload profile: standard (redacted, 64 KiB) or full (unredacted, 1 MiB; upstream omissions still apply)"),
		limit:   fs.Int("max-payload-bytes", 0, "maximum retained JSON bytes per payload (256 to 16777216; 0 uses profile default)"),
	}
}

func (f recorderFlags) settings() (recorder.CaptureSettings, error) {
	return recorder.CaptureOptions(*f.profile, *f.limit)
}

func addCaptureFlags(fs *flag.FlagSet) captureFlags {
	f := captureFlags{mode: fs.String("mode", "observe", "capture mode (observe: records without changing agent permissions)")}
	fs.String("policy", "", "removed: Sentinel now records without enforcing policy")
	fs.String("max-cost", "", "removed: Sentinel no longer enforces spending limits")
	fs.String("approval-timeout", "", "removed: Sentinel no longer holds calls for approval")
	return f
}

func (f captureFlags) validate(fs *flag.FlagSet) error {
	var removed string
	fs.Visit(func(v *flag.Flag) {
		switch v.Name {
		case "policy", "max-cost", "approval-timeout":
			if removed == "" {
				removed = v.Name
			}
		}
	})
	if removed != "" {
		return fmt.Errorf("--%s is no longer supported: Sentinel is an observation-only recorder; remove this option. Keep the previous release if you need its enforcement", removed)
	}
	if *f.mode != "observe" {
		return fmt.Errorf("unsupported --mode %q: use observe; Sentinel no longer blocks commands or asks for approval", *f.mode)
	}
	return nil
}
