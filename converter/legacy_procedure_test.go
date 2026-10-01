package converter

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
)

const legacyProcedureFixture = `{"targets":[{"isStage":true,"name":"Stage","blocks":{
 "definition":{"opcode":"procedures_definition","topLevel":true,"parent":null,"inputs":{"custom_block":[1,"prototype"]}},
 "prototype":{"opcode":"procedures_prototype","shadow":true,"inputs":{},"mutation":{"proccode":"demo %n","argumentnames":"[\"amount\"]","argumentids":"[\"input0\"]","argumentdefaults":"[1]"}},
 "argument":{"opcode":"argument_reporter_string_number","shadow":true,"parent":"prototype","fields":{"VALUE":["amount",null]}}
}}]}`

func TestLegacyProcedureGraphRoundTripPreservesOriginalJSON(t *testing.T) {
	data := []byte(legacyProcedureFixture)
	if err := validateProjectJSON(data); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	source, workspace, output := filepath.Join(root, "input.sb3"), filepath.Join(root, "workspace"), filepath.Join(root, "output.sb3")
	writeSB3Fixture(t, source, data, nil)
	spec := inspectionSpec(t)
	if err := Run(ConvertOptions{SB3Path: source, OutputDir: workspace, SpecmapData: spec, EngineFS: fstest.MapFS{"engine/__init__.py": {Data: []byte("")}}}); err != nil {
		t.Fatal(err)
	}
	if err := SyncRoundTripProject(workspace, spec); err != nil {
		t.Fatal(err)
	}
	if err := VerifyRoundTripProject(workspace, spec); err != nil {
		t.Fatal(err)
	}
	canonical, err := os.ReadFile(filepath.Join(workspace, roundTripDirName, roundTripProjectName))
	if err != nil || !bytes.Equal(canonical, data) {
		t.Fatalf("canonical JSON was rewritten: %v", err)
	}
	if err := Reverse(ReverseOptions{ProjectPath: workspace, OutputSB3: output, SpecmapData: spec}); err != nil {
		t.Fatal(err)
	}
	archive, err := zip.OpenReader(output)
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	for _, entry := range archive.File {
		if entry.Name != "project.json" {
			continue
		}
		reader, err := entry.Open()
		if err != nil {
			t.Fatal(err)
		}
		content, err := io.ReadAll(reader)
		reader.Close()
		if err != nil || !bytes.Equal(content, data) {
			t.Fatalf("repacked JSON changed: %v", err)
		}
		return
	}
	t.Fatal("repacked project.json missing")
}

func TestLegacyProcedureCompatibilityStillRejectsAmbiguousOrBrokenGraphs(t *testing.T) {
	for _, kind := range []string{"duplicate_definition", "wrong_argument", "non_shadow_argument", "duplicate_shadow", "missing_definition", "ordinary_orphan", "malformed_mutation"} {
		t.Run(kind, func(t *testing.T) {
			var project map[string]interface{}
			if err := json.Unmarshal([]byte(legacyProcedureFixture), &project); err != nil {
				t.Fatal(err)
			}
			target := project["targets"].([]interface{})[0].(map[string]interface{})
			blocks := target["blocks"].(map[string]interface{})
			argument := blocks["argument"].(map[string]interface{})
			switch kind {
			case "duplicate_definition":
				blocks["other_definition"] = blocks["definition"]
			case "wrong_argument":
				argument["fields"] = map[string]interface{}{"VALUE": []interface{}{"other", nil}}
			case "non_shadow_argument":
				argument["shadow"] = false
			case "duplicate_shadow":
				blocks["other_argument"] = argument
			case "missing_definition":
				delete(blocks, "definition")
			case "ordinary_orphan":
				blocks["orphan"] = map[string]interface{}{"opcode": "motion_movesteps", "topLevel": false}
			case "malformed_mutation":
				blocks["prototype"].(map[string]interface{})["mutation"] = map[string]interface{}{"argumentnames": "bad", "argumentids": "[]"}
			}
			data, err := json.Marshal(project)
			if err != nil {
				t.Fatal(err)
			}
			if err := validateProjectJSON(data); err == nil {
				t.Fatal("invalid graph was accepted")
			}
		})
	}
}
