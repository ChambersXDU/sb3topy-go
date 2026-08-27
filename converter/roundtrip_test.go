package converter

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
