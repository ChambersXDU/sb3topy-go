package converter

import (
	"os"
	"strings"
	"testing"
)

func TestTranspileProject(t *testing.T) {
	specData, err := os.ReadFile("../specmap_data.json")
	if err != nil {
		t.Fatalf("Failed to read specmap_data.json: %v", err)
	}

	sm, err := LoadSpecMap(specData)
	if err != nil {
		t.Fatalf("Failed to load specmap: %v", err)
	}

	// Self-contained mock project.json for CI/CD test environment
	projJson := []byte(`{
		"targets": [
			{
				"isStage": true,
				"name": "Stage",
				"variables": {},
				"lists": {},
				"broadcasts": {},
				"blocks": {},
				"costumes": [],
				"sounds": [],
				"layerOrder": 0
			},
			{
				"isStage": false,
				"name": "玩家",
				"variables": {
					"v1": ["玩家血量", 5]
				},
				"lists": {
					"l1": ["武器", ["步枪", "手枪"]]
				},
				"broadcasts": {},
				"blocks": {
					"b1": {
						"opcode": "event_whenflagclicked",
						"next": null,
						"parent": null,
						"inputs": {},
						"fields": {},
						"shadow": false,
						"topLevel": true
					}
				},
				"costumes": [],
				"sounds": [],
				"layerOrder": 1
			}
		]
	}`)

	pyCode, err := TranspileProject(projJson, sm)
	if err != nil {
		t.Fatalf("TranspileProject failed: %v", err)
	}

	// Assertions for generated Python code
	assertContains(t, pyCode, "import math")
	assertContains(t, pyCode, "import engine")
	assertContains(t, pyCode, "@sprite('Stage')")
	assertContains(t, pyCode, "class Stage(Target):")

	// Chinese Unicode identifiers assertions
	assertContains(t, pyCode, "@sprite('玩家')")
	assertContains(t, pyCode, "class Sprite玩家(Target):")
	assertContains(t, pyCode, "self.var_玩家血量 = 5")
	assertContains(t, pyCode, "self.list_武器 = List(")

	// Event hat decorators assertion
	assertContains(t, pyCode, "@on_green_flag")
	assertContains(t, pyCode, "if __name__ == '__main__':")
	assertContains(t, pyCode, "engine.start_program()")

	// Ensure no unhandled opcodes in generated code
	if strings.Contains(pyCode, "# opcode:") {
		t.Errorf("Generated code contains unhandled opcodes: %s", pyCode)
	}
}

func TestTranspileProjectPreservesZeroVolumeAndCostumeProperties(t *testing.T) {
	specData, err := os.ReadFile("../specmap_data.json")
	if err != nil {
		t.Fatal(err)
	}
	sm, err := LoadSpecMap(specData)
	if err != nil {
		t.Fatal(err)
	}
	projectJSON := []byte(`{
		"targets": [{
			"isStage": false,
			"name": "Sprite",
			"variables": {},
			"lists": {},
			"broadcasts": {},
			"blocks": {},
			"currentCostume": 1,
			"costumes": [
				{"name":"one","assetId":"one","dataFormat":"png","md5ext":"one.png","bitmapResolution":2,"rotationCenterX":0,"rotationCenterY":3},
				{"name":"two","assetId":"two","dataFormat":"svg","md5ext":"two.svg","bitmapResolution":1,"rotationCenterX":4,"rotationCenterY":5}
			],
			"sounds": [],
			"volume": 0,
			"size": 75,
			"rotationStyle": "left-right",
			"layerOrder": 1,
			"visible": true
		}]
	}`)
	py, err := TranspileProject(projectJSON, sm)
	if err != nil {
		t.Fatal(err)
	}
	assertContains(t, py, "           1, 75, \"left-right\", [")
	assertContains(t, py, "'center': (0, 3)")
	assertContains(t, py, "'scale': 2")
	assertContains(t, py, "self.sounds = Sounds(\n            0, [")
}

func assertContains(t *testing.T, code, sub string) {
	t.Helper()
	if !strings.Contains(code, sub) {
		t.Errorf("Expected generated Python code to contain %q", sub)
	}
}
