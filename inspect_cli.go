package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"

	"sb3topy/converter"
)

func runInspect(args []string, output, errorOutput io.Writer) error {
	flags := flag.NewFlagSet("inspect", flag.ContinueOnError)
	flags.SetOutput(errorOutput)
	jsonOutput := flags.Bool("json", false, "Print a machine-readable JSON report")
	target := flags.String("target", "", "Select a stage or sprite by its exact Scratch name")
	block := flags.String("block", "", "Include the original Scratch block with this ID")
	flags.Usage = func() {
		fmt.Fprintln(errorOutput, "Usage: sb3topy inspect [--json] [--target NAME] [--block ID] <project.sb3|workspace|project.py>")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}
	if flags.NArg() != 1 {
		flags.Usage()
		return fmt.Errorf("inspect needs one SB3 file or generated workspace; place options before the path")
	}
	report, err := converter.Inspect(converter.InspectOptions{Path: flags.Arg(0), SpecmapData: specmapData, Target: *target, BlockID: *block})
	if err != nil {
		return err
	}
	if *jsonOutput {
		encoder := json.NewEncoder(output)
		encoder.SetIndent("", "  ")
		return encoder.Encode(report)
	}
	fmt.Fprintf(output, "Scratch project: %s\nSource: %s\n", report.Source, report.SourceKind)
	if report.CanonicalPath != "" {
		fmt.Fprintf(output, "Scratch source to edit: %s\nPython analysis view: %s\n", report.CanonicalPath, report.PythonPath)
	}
	for _, check := range []struct {
		name  string
		value *converter.InspectionCheck
	}{
		{"Block graph", &report.Graph}, {"Python generation", &report.Generation}, {"Round-trip verification", report.RoundTrip},
	} {
		if check.value == nil {
			continue
		}
		if check.value.OK {
			fmt.Fprintf(output, "%s: passed\n", check.name)
		} else {
			fmt.Fprintf(output, "%s: %s\n", check.name, check.value.Detail)
		}
	}
	for _, target := range report.Targets {
		kind := "Sprite"
		if target.IsStage {
			kind = "Stage"
		}
		fmt.Fprintf(output, "\n%s %q (target index %d): %d blocks, %d scripts, %d variables, %d lists\n", kind, target.Name, target.Index, target.BlockCount, len(target.Scripts), len(target.Variables), len(target.Lists))
		for _, script := range target.Scripts {
			fmt.Fprintf(output, "  Script: id=%q opcode=%s\n", script.ID, script.Opcode)
		}
	}
	fmt.Fprintf(output, "\nDetected translation issues: %d\n", len(report.Issues))
	for _, issue := range report.Issues {
		fmt.Fprintf(output, "  %s: target=%q block=%q opcode=%s generatedPythonLine=%d\n    %s\n", issue.Code, issue.Target, issue.BlockID, issue.Opcode, issue.GeneratedPythonLine, issue.Message)
	}
	for _, block := range report.Blocks {
		fmt.Fprintf(output, "\nBlock %q in %q (target index %d, generatedPythonLine=%d, kind=%s):\n", block.ID, block.Target, block.TargetIndex, block.GeneratedPythonLine, block.MarkerKind)
		encoder := json.NewEncoder(output)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(block.Raw); err != nil {
			return err
		}
	}
	fmt.Fprintln(output, "\nInspection is read-only. Generation and verification checks do not prove Scratch runtime behavior.")
	return nil
}
