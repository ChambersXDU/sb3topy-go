package converter

import (
	"os"
	"testing"
)

func TestTranspileProjectAddsScratchSourceMarkers(t *testing.T) {
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
			"isStage": true,
			"name": "Stage",
			"variables": {},
			"lists": {},
			"broadcasts": {},
			"blocks": {
				"hat-1": {
					"opcode": "event_whenflagclicked",
					"next": "move-1",
					"parent": null,
					"inputs": {},
					"fields": {},
					"shadow": false,
					"topLevel": true
				},
				"move-1": {
					"opcode": "motion_movesteps",
					"next": null,
					"parent": "hat-1",
					"inputs": {"STEPS": [1, [4, "10"]]},
					"fields": {},
					"shadow": false,
					"topLevel": false
				}
			},
			"costumes": [],
			"sounds": [],
			"layerOrder": 0,
			"x": 0,
			"y": 0,
			"direction": 90,
			"visible": true
		}]
	}`)

	py, err := TranspileProject(projectJSON, sm)
	if err != nil {
		t.Fatal(err)
	}
	assertContains(t, py, "# sb3topy:target 'Stage'")
	assertContains(t, py, "# sb3topy:hat hat-1 event_whenflagclicked")
	assertContains(t, py, "# sb3topy:block move-1 motion_movesteps")
}
