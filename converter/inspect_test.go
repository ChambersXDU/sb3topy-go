package converter

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const inspectionFixture = `{
 "extensions":["pen"],
 "targets":[
  {"isStage":true,"name":"Stage","variables":{"g":["score",0]},"lists":{},"blocks":{
   "stage-hat":{"opcode":"event_whenflagclicked","next":"shared","parent":null,"topLevel":true},
   "shared":{"opcode":"motion_movesteps","next":null,"parent":"stage-hat","inputs":{"STEPS":[1,[4,"10"]]},"futureValue":9007199254740993}
  }},
  {"isStage":false,"name":"角色\"1","variables":{},"lists":{"l":["items",[1,2]]},"blocks":{
   "hat":{"opcode":"event_whenkeypressed","fields":{"KEY_OPTION":["space",null]},"next":"shared","parent":null,"topLevel":true},
   "shared":{"opcode":"extension_unknown","next":"set","parent":"hat"},
   "set":{"opcode":"motion_setx","next":null,"parent":"shared","inputs":{"X":[2,"reporter"]}},
   "reporter":{"opcode":"extension_reporter_unknown","next":null,"parent":"set"},
   "unused":{"opcode":"extension_unknown","next":null,"parent":null,"topLevel":true}
  }}
 ]
}`

func inspectionSpec(t *testing.T) []byte {
	t.Helper()
	spec, err := os.ReadFile("../specmap_data.json")
	if err != nil {
		t.Fatal(err)
	}
	return spec
}

func TestInspectArchiveIsReadOnlyAndFindsTranslationLimitations(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "学生的作品.sb3")
	writeSB3Fixture(t, path, []byte(inspectionFixture), nil)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	opts := InspectOptions{Path: path, SpecmapData: inspectionSpec(t)}
	report, err := Inspect(opts)
	if err != nil {
		t.Fatal(err)
	}
	if !report.Graph.OK || !report.Generation.OK || report.RoundTrip != nil || report.SourceKind != "sb3" {
		t.Fatalf("unexpected checks: %+v", report)
	}
	if len(report.Targets) != 2 || len(report.Targets[0].Scripts) != 1 || len(report.Targets[1].Scripts) != 1 || len(report.Targets[1].Lists) != 1 {
		t.Fatalf("unexpected target summaries: %+v", report.Targets)
	}
	if len(report.Extensions) != 1 || report.Extensions[0] != "pen" {
		t.Fatal(report.Extensions)
	}
	if field := parseFieldValue(report.Targets[1].Scripts[0].Fields["KEY_OPTION"]); field != "space" {
		t.Fatalf("script entry condition was lost: %q", field)
	}
	if len(report.Issues) != 3 {
		t.Fatalf("unexpected issues: %+v", report.Issues)
	}
	python, err := TranspileProject([]byte(inspectionFixture), mustInspectionSpecMap(t))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(python, "\n")
	for _, issue := range report.Issues {
		if issue.TargetIndex != 1 || issue.Target != "角色\"1" || issue.BlockID == "unused" {
			t.Fatalf("wrong block attribution: %+v", issue)
		}
		if issue.GeneratedPythonLine < 1 || !strings.Contains(lines[issue.GeneratedPythonLine-1], "id="+QuoteString(issue.BlockID)) {
			t.Fatalf("source line does not locate the block: %+v", issue)
		}
	}
	second, err := Inspect(opts)
	if err != nil {
		t.Fatal(err)
	}
	firstJSON, _ := json.Marshal(report)
	secondJSON, _ := json.Marshal(second)
	if !bytes.Equal(firstJSON, secondJSON) {
		t.Fatal("inspection output is not deterministic")
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) || len(entries) != 1 {
		t.Fatal("inspection changed the input or created files")
	}
}

func mustInspectionSpecMap(t *testing.T) *SpecMap {
	t.Helper()
	sm, err := LoadSpecMap(inspectionSpec(t))
	if err != nil {
		t.Fatal(err)
	}
	return sm
}

func TestInspectReportsPartialTimingAndUnimplementedBubbles(t *testing.T) {
	project := []byte(`{"targets":[{"isStage":true,"name":"Stage","blocks":{
		"hat":{"opcode":"event_whenflagclicked","topLevel":true,"parent":null,"next":"say"},
		"say":{"opcode":"looks_sayforsecs","parent":"hat","next":"think","inputs":{"MESSAGE":[1,[10,"hello"]],"SECS":[1,[10,"0.1"]]}},
		"think":{"opcode":"looks_think","parent":"say","next":"hide","inputs":{"MESSAGE":[1,[10,"hmm"]]}},
		"hide":{"opcode":"data_hidevariable","parent":"think","next":null,"fields":{"VARIABLE":["score","v"]}}
	},"variables":{"v":["score",0]}}]}`)
	path := filepath.Join(t.TempDir(), "bubbles.sb3")
	writeSB3Fixture(t, path, project, nil)
	report, err := Inspect(InspectOptions{Path: path, SpecmapData: inspectionSpec(t), BlockID: "say"})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Blocks) != 1 || report.Blocks[0].TranslationStatus != "partial" || len(report.Issues) != 1 || report.Issues[0].Code != "partial_block" {
		t.Fatalf("partial translation was not reported: %+v", report)
	}
	report, err = Inspect(InspectOptions{Path: path, SpecmapData: inspectionSpec(t)})
	if err != nil {
		t.Fatal(err)
	}
	counts := report.Targets[0].TranslationCounts
	if counts["implemented"] != 1 || counts["partial"] != 1 || counts["unsupported"] != 2 || len(report.Issues) != 3 {
		t.Fatalf("placeholder behaviors were hidden: %+v", report)
	}
	python, err := TranspileProject(project, mustInspectionSpecMap(t))
	if err != nil {
		t.Fatal(err)
	}
	assertContains(t, python, `await self.sleep(max(0, tonum("0.1")))`)
}

func TestInspectBlockSelectionPreservesUnknownFieldsAndTargetIdentity(t *testing.T) {
	path := filepath.Join(t.TempDir(), "game.sb3")
	writeSB3Fixture(t, path, []byte(inspectionFixture), nil)
	opts := InspectOptions{Path: path, SpecmapData: inspectionSpec(t), BlockID: "shared"}
	report, err := Inspect(opts)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Blocks) != 2 || report.Blocks[0].TargetIndex != 0 || report.Blocks[1].TargetIndex != 1 {
		t.Fatalf("duplicate IDs lost target identity: %+v", report.Blocks)
	}
	if !bytes.Contains(report.Blocks[0].Raw, []byte(`9007199254740993`)) {
		t.Fatal("original unknown value was rounded or dropped")
	}
	if report.Blocks[0].GeneratedPythonLine == report.Blocks[1].GeneratedPythonLine {
		t.Fatal("blocks from different targets share a source location")
	}
	opts.Target = "角色\"1"
	report, err = Inspect(opts)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Targets) != 1 || len(report.Blocks) != 1 || len(report.Issues) != 1 || report.Blocks[0].TargetIndex != 1 {
		t.Fatalf("target selection failed: %+v", report)
	}
	opts.BlockID = "missing"
	if _, err := Inspect(opts); err == nil {
		t.Fatal("missing block was silently accepted")
	}
	opts.Target = "missing"
	if _, err := Inspect(opts); err == nil {
		t.Fatal("missing target was silently accepted")
	}
}

func TestInspectMalformedGraphStillReturnsBlockForDiagnosis(t *testing.T) {
	path := filepath.Join(t.TempDir(), "broken.sb3")
	broken := strings.Replace(inspectionFixture, `"next":"shared"`, `"next":"missing"`, 1)
	writeSB3Fixture(t, path, []byte(broken), nil)
	report, err := Inspect(InspectOptions{Path: path, SpecmapData: inspectionSpec(t), BlockID: "stage-hat"})
	if err != nil {
		t.Fatal(err)
	}
	if report.Graph.OK || report.Generation.OK || report.Graph.Detail == "" {
		t.Fatalf("broken graph was not reported: %+v", report)
	}
	if len(report.Blocks) != 1 || report.Blocks[0].GeneratedPythonLine != 0 || !bytes.Contains(report.Blocks[0].Raw, []byte(`"next":"missing"`)) {
		t.Fatal("broken block could not be inspected without generation")
	}
}

func TestInspectWorkspaceReportsPythonDriftWithoutSyncing(t *testing.T) {
	dir := t.TempDir()
	spec := inspectionSpec(t)
	python, err := TranspileProject([]byte(inspectionFixture), mustInspectionSpecMap(t))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "assets"), 0755); err != nil {
		t.Fatal(err)
	}
	pyPath := filepath.Join(dir, "project.py")
	if err := os.WriteFile(pyPath, []byte(python), 0644); err != nil {
		t.Fatal(err)
	}
	if err := writeRoundTripMetadata(dir, []byte(inspectionFixture), []byte(python), "game.sb3"); err != nil {
		t.Fatal(err)
	}
	report, err := Inspect(InspectOptions{Path: pyPath, SpecmapData: spec})
	if err != nil {
		t.Fatal(err)
	}
	if report.RoundTrip == nil || !report.RoundTrip.OK {
		t.Fatalf("clean workspace failed: %+v", report)
	}
	drifted := []byte(python + "\n# student experiment\n")
	if err := os.WriteFile(pyPath, drifted, 0644); err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(dir, roundTripDirName, roundTripManifestName)
	beforeManifest, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	report, err = Inspect(InspectOptions{Path: dir, SpecmapData: spec})
	if err != nil {
		t.Fatal(err)
	}
	if report.RoundTrip.OK || !strings.Contains(report.RoundTrip.Detail, "Python-only") {
		t.Fatalf("drift was not reported: %+v", report)
	}
	afterPython, err := os.ReadFile(pyPath)
	if err != nil {
		t.Fatal(err)
	}
	afterManifest, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(afterPython, drifted) || !bytes.Equal(beforeManifest, afterManifest) {
		t.Fatal("inspect synchronized or changed the workspace")
	}
}
