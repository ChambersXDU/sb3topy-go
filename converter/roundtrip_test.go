package converter

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"testing/fstest"
)

func TestRoundTripVerifySyncAndReverse(t *testing.T) {
	specData, err := os.ReadFile("../specmap_data.json")
	if err != nil {
		t.Fatalf("read specmap: %v", err)
	}
	sm, err := LoadSpecMap(specData)
	if err != nil {
		t.Fatalf("load specmap: %v", err)
	}

	projectJSON := []byte(`{
		"targets": [{
			"isStage": true,
			"name": "Stage",
			"variables": {},
			"lists": {},
			"broadcasts": {},
			"blocks": {},
			"costumes": [],
			"sounds": [],
			"layerOrder": 0,
			"x": 0,
			"y": 0,
			"direction": 90,
			"visible": true
		}]
	}`)

	projectPy, err := TranspileProject(projectJSON, sm)
	if err != nil {
		t.Fatalf("transpile: %v", err)
	}

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "assets"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "project.py"), []byte(projectPy), 0644); err != nil {
		t.Fatal(err)
	}
	if err := writeRoundTripMetadata(dir, projectJSON, []byte(projectPy), "demo.sb3"); err != nil {
		t.Fatal(err)
	}

	if err := VerifyRoundTripProject(dir, specData); err != nil {
		t.Fatalf("verify should pass: %v", err)
	}

	if err := os.WriteFile(filepath.Join(dir, "project.py"), []byte(projectPy+"\n# drift\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := VerifyRoundTripProject(dir, specData); err == nil || !strings.Contains(err.Error(), "Python-only") {
		t.Fatalf("verify should reject Python-only drift, got: %v", err)
	}
	badOutput := filepath.Join(t.TempDir(), "bad.sb3")
	if err := Reverse(ReverseOptions{ProjectPath: dir, OutputSB3: badOutput, SpecmapData: specData}); err == nil || !strings.Contains(err.Error(), "Python-only") {
		t.Fatalf("reverse should reject Python-only drift, got: %v", err)
	}
	if _, err := os.Stat(badOutput); !os.IsNotExist(err) {
		t.Fatalf("reverse must not leave output after Python-only drift, stat error: %v", err)
	}

	if err := SyncRoundTripProject(dir, specData); err != nil {
		t.Fatalf("sync: %v", err)
	}
	if err := VerifyRoundTripProject(dir, specData); err != nil {
		t.Fatalf("verify after sync: %v", err)
	}

	out := filepath.Join(t.TempDir(), "demo.sb3")
	if err := Reverse(ReverseOptions{ProjectPath: dir, OutputSB3: out, SpecmapData: specData}); err != nil {
		t.Fatalf("reverse: %v", err)
	}

	zr, err := zip.OpenReader(out)
	if err != nil {
		t.Fatalf("open sb3 zip: %v", err)
	}
	defer zr.Close()

	foundProject := false
	for _, f := range zr.File {
		if f.Name == "project.json" {
			foundProject = true
		}
	}
	if !foundProject {
		t.Fatal("reverse output missing project.json")
	}
}

func TestHighFidelitySB3RoundTripAndCanonicalBlockEdit(t *testing.T) {
	specData, err := os.ReadFile("../specmap_data.json")
	if err != nil {
		t.Fatal(err)
	}
	projectJSON := []byte(`{
  "targets": [
    {
      "isStage": true,
      "name": "Stage",
      "variables": {"global-var": ["score", 7]},
      "lists": {"global-list": ["items", ["a", 2]]},
      "broadcasts": {"broadcast-id": "start"},
      "blocks": {
        "hat-id": {"opcode": "event_whenflagclicked", "next": "repeat-id", "parent": null, "inputs": {}, "fields": {}, "shadow": false, "topLevel": true, "x": 10, "y": 20},
        "repeat-id": {"opcode": "control_repeat", "next": null, "parent": "hat-id", "inputs": {"TIMES": [1, [4, "2"]], "SUBSTACK": [2, "move-id"]}, "fields": {}, "shadow": false, "topLevel": false},
        "move-id": {"opcode": "motion_movesteps", "next": null, "parent": "repeat-id", "inputs": {"STEPS": [1, [4, "10"]]}, "fields": {}, "shadow": false, "topLevel": false, "futureBlockField": {"preserve": true}}
      },
      "comments": {"comment-id": {"blockId": "move-id", "text": "keep me"}},
      "currentCostume": 0,
      "costumes": [{"assetId": "svgasset", "name": "backdrop", "bitmapResolution": 1, "md5ext": "svgasset.svg", "dataFormat": "svg", "rotationCenterX": 240, "rotationCenterY": 180, "futureCostumeField": 1}],
      "sounds": [{"assetId": "soundwav", "name": "beep", "dataFormat": "wav", "format": "", "rate": 44100, "sampleCount": 4, "md5ext": "soundwav.wav", "futureSoundField": true}],
      "volume": 100,
      "layerOrder": 0,
      "tempo": 60,
      "videoTransparency": 50,
      "videoState": "on",
      "textToSpeechLanguage": null,
      "futureTargetField": {"preserve": [1, 2, 3]}
    },
    {
      "isStage": false,
      "name": "Sprite One",
      "variables": {"sprite-var": ["health", 100]},
      "lists": {},
      "broadcasts": {},
      "blocks": {},
      "comments": {},
      "currentCostume": 1,
      "costumes": [
        {"assetId": "pngasset", "name": "png", "bitmapResolution": 2, "md5ext": "pngasset.png", "dataFormat": "png", "rotationCenterX": 1, "rotationCenterY": 2},
        {"assetId": "jpgasset", "name": "jpg", "bitmapResolution": 1, "md5ext": "jpgasset.jpg", "dataFormat": "jpg", "rotationCenterX": 3, "rotationCenterY": 4}
      ],
      "sounds": [{"assetId": "soundmp3", "name": "music", "dataFormat": "mp3", "rate": 22050, "sampleCount": 8, "md5ext": "soundmp3.mp3"}],
      "volume": 80,
      "layerOrder": 1,
      "visible": true,
      "x": 12,
      "y": -4,
      "size": 90,
      "direction": 45,
      "draggable": false,
      "rotationStyle": "all around"
    }
  ],
  "monitors": [{"id": "global-var", "mode": "default", "opcode": "data_variable", "params": {"VARIABLE": "score"}, "spriteName": null, "value": 7, "width": 0, "height": 0, "x": 5, "y": 6, "visible": true, "sliderMin": 0, "sliderMax": 100, "isDiscrete": true}],
  "extensions": ["pen", "music"],
  "meta": {"semver": "3.0.0", "vm": "test-vm", "agent": "test-agent", "futureMeta": {"keep": true}},
  "futureTopLevelField": {"nested": ["unchanged"]}
}
`)
	assetData := map[string][]byte{
		"svgasset.svg":      []byte(`<svg xmlns="http://www.w3.org/2000/svg"><rect width="1" height="1"/></svg>`),
		"pngasset.png":      []byte("\x89PNG\r\n\x1a\nfixture"),
		"jpgasset.jpg":      []byte("\xff\xd8\xfffixture\xff\xd9"),
		"soundwav.wav":      []byte("RIFF\x04\x00\x00\x00WAVE"),
		"soundmp3.mp3":      []byte("ID3fixture"),
		"extras/nested.bin": []byte("non-Scratch archive entry"),
	}

	root := t.TempDir()
	sourceSB3 := filepath.Join(root, "fixture.sb3")
	writeSB3Fixture(t, sourceSB3, projectJSON, assetData)
	workspace := filepath.Join(root, "workspace")
	engineFS := fstest.MapFS{"engine/runtime.py": {Data: []byte("# test runtime\n")}}
	if err := Run(ConvertOptions{SB3Path: sourceSB3, OutputDir: workspace, EngineFS: engineFS, SpecmapData: specData}); err != nil {
		t.Fatalf("to-python: %v", err)
	}

	canonicalPath := filepath.Join(workspace, roundTripDirName, roundTripProjectName)
	canonical, err := os.ReadFile(canonicalPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(canonical, projectJSON) {
		t.Fatal("to-python must preserve the original project.json bytes")
	}
	if err := VerifyRoundTripProject(workspace, specData); err != nil {
		t.Fatalf("baseline verify: %v", err)
	}
	extraPath := filepath.Join(workspace, "assets", "extras", "nested.bin")
	if err := os.Remove(extraPath); err != nil {
		t.Fatal(err)
	}
	if err := VerifyRoundTripProject(workspace, specData); err == nil || !strings.Contains(err.Error(), "original SB3 entry") {
		t.Fatalf("verify should detect a missing unreferenced source entry, got: %v", err)
	}
	if err := os.WriteFile(extraPath, assetData["extras/nested.bin"], 0644); err != nil {
		t.Fatal(err)
	}

	modified := bytes.Replace(canonical, []byte(`"STEPS": [1, [4, "10"]]`), []byte(`"STEPS": [1, [4, "20"]]`), 1)
	if bytes.Equal(modified, canonical) {
		t.Fatal("fixture block literal replacement did not run")
	}
	if err := os.WriteFile(canonicalPath, modified, 0644); err != nil {
		t.Fatal(err)
	}
	if err := SyncRoundTripProject(workspace, specData); err != nil {
		t.Fatalf("sync canonical edit: %v", err)
	}
	if err := VerifyRoundTripProject(workspace, specData); err != nil {
		t.Fatalf("verify canonical edit: %v", err)
	}
	projectPy, err := os.ReadFile(filepath.Join(workspace, "project.py"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(projectPy, []byte("self.move(20)")) {
		t.Fatalf("synced Python does not contain the edited literal:\n%s", projectPy)
	}

	outputSB3 := filepath.Join(root, "fixed.sb3")
	if err := Reverse(ReverseOptions{ProjectPath: workspace, OutputSB3: outputSB3, SpecmapData: specData}); err != nil {
		t.Fatalf("to-sb3: %v", err)
	}
	outputEntries := readZipEntries(t, outputSB3)
	outputProject, ok := outputEntries["project.json"]
	if !ok {
		t.Fatal("round-trip SB3 is missing root project.json")
	}
	if !bytes.Equal(outputProject, modified) {
		t.Fatal("to-sb3 must package the canonical project.json bytes without struct re-marshalling")
	}
	for name, original := range assetData {
		got, ok := outputEntries[name]
		if !ok || !bytes.Equal(got, original) {
			t.Errorf("archive entry %q was not preserved byte-for-byte", name)
		}
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal(outputProject, &decoded); err != nil {
		t.Fatalf("output project.json is invalid: %v", err)
	}
	targets := decoded["targets"].([]interface{})
	if len(targets) != 2 || targets[0].(map[string]interface{})["name"] != "Stage" || targets[1].(map[string]interface{})["name"] != "Sprite One" {
		t.Fatalf("target count or names changed: %#v", targets)
	}
	stage := targets[0].(map[string]interface{})
	if len(stage["blocks"].(map[string]interface{})) != 3 {
		t.Fatalf("block count changed: %#v", stage["blocks"])
	}
	move := stage["blocks"].(map[string]interface{})["move-id"].(map[string]interface{})
	steps := move["inputs"].(map[string]interface{})["STEPS"].([]interface{})[1].([]interface{})[1]
	if steps != "20" {
		t.Fatalf("fixed SB3 move-id STEPS = %#v, want \"20\"", steps)
	}
	if _, ok := decoded["futureTopLevelField"]; !ok {
		t.Fatal("unknown top-level field was lost")
	}
	if _, ok := stage["futureTargetField"]; !ok {
		t.Fatal("unknown target field was lost")
	}
	if _, ok := move["futureBlockField"]; !ok {
		t.Fatal("unknown block field was lost")
	}
}

func TestRunRejectsZipSlip(t *testing.T) {
	root := t.TempDir()
	sourceSB3 := filepath.Join(root, "malicious.sb3")
	out, err := os.Create(sourceSB3)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(out)
	if err := writeZipBytes(zw, "project.json", []byte(`{"targets":[{"isStage":true,"name":"Stage","blocks":{},"costumes":[],"sounds":[]}]}`)); err != nil {
		t.Fatal(err)
	}
	if err := writeZipBytes(zw, "../../../evil.txt", []byte("evil")); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := out.Close(); err != nil {
		t.Fatal(err)
	}

	specData, err := os.ReadFile("../specmap_data.json")
	if err != nil {
		t.Fatal(err)
	}
	workspace := filepath.Join(root, "workspace")
	err = Run(ConvertOptions{
		SB3Path:     sourceSB3,
		OutputDir:   workspace,
		EngineFS:    fstest.MapFS{"engine/runtime.py": {Data: []byte("# runtime\n")}},
		SpecmapData: specData,
	})
	if err == nil || !strings.Contains(err.Error(), "unsafe") {
		t.Fatalf("expected Zip Slip rejection, got %v", err)
	}
}

func writeSB3Fixture(t *testing.T, destination string, projectJSON []byte, assets map[string][]byte) {
	t.Helper()
	out, err := os.Create(destination)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(out)
	if err := writeZipBytes(zw, "project.json", projectJSON); err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(assets))
	for name := range assets {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if err := writeZipBytes(zw, name, assets[name]); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := out.Close(); err != nil {
		t.Fatal(err)
	}
}

func readZipEntries(t *testing.T, archivePath string) map[string][]byte {
	t.Helper()
	zr, err := zip.OpenReader(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	entries := make(map[string][]byte, len(zr.File))
	for _, file := range zr.File {
		if file.FileInfo().IsDir() {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(rc)
		closeErr := rc.Close()
		if err != nil {
			t.Fatal(err)
		}
		if closeErr != nil {
			t.Fatal(closeErr)
		}
		entries[file.Name] = data
	}
	return entries
}
