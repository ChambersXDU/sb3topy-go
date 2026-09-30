package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"sb3topy/converter"
)

func TestInspectCLIJSONIsCleanAndErrorsDoNotPolluteStdout(t *testing.T) {
	path := filepath.Join(t.TempDir(), "My project.sb3")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	archive := zip.NewWriter(file)
	entry, err := archive.Create("project.json")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(entry, `{"targets":[{"isStage":true,"name":"Stage","blocks":{}}]}`); err != nil {
		t.Fatal(err)
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	var output, errors bytes.Buffer
	if err := runInspect([]string{"--json", path}, &output, &errors); err != nil {
		t.Fatal(err)
	}
	decoder := json.NewDecoder(&output)
	var report converter.Inspection
	if err := decoder.Decode(&report); err != nil {
		t.Fatalf("stdout is not JSON: %v", err)
	}
	if decoder.Decode(new(interface{})) != io.EOF || errors.Len() != 0 {
		t.Fatal("JSON output contains logs or multiple values")
	}
	if report.SchemaVersion != 1 || len(report.Targets) != 1 || !report.Graph.OK {
		t.Fatalf("unexpected JSON: %+v", report)
	}
	output.Reset()
	if err := runInspect([]string{"--json", "--block", "missing", path}, &output, &errors); err == nil {
		t.Fatal("expected missing block error")
	}
	if output.Len() != 0 {
		t.Fatal("error wrote partial JSON to stdout")
	}
	if err := runInspect([]string{"--json"}, &output, &errors); err == nil {
		t.Fatal("missing path was accepted")
	}
	if errors.Len() == 0 {
		t.Fatal("usage did not reach stderr")
	}
}
