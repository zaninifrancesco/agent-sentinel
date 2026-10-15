package main

import (
	"flag"
	"fmt"
)

// captureFlags makes the change from enforcement to observation explicit.
// Removed options remain parseable only to return an actionable migration error.
type captureFlags struct {
	mode *string
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
