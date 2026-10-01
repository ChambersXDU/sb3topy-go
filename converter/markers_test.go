package converter

import (
	"os"
	"strings"
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
	assertContains(t, py, "# sb3topy:target \"Stage\"")
	assertContains(t, py, `# sb3topy:hat id="hat-1" opcode="event_whenflagclicked"`)
	assertContains(t, py, `# sb3topy:block id="move-1" opcode="motion_movesteps" kind=stack shadow=false`)
}

func TestSourceMarkersCoverNestedReporterShadowAndProcedureBlocksOnce(t *testing.T) {
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
				"hat": {"opcode":"event_whenflagclicked","next":"if","parent":null,"inputs":{},"fields":{},"shadow":false,"topLevel":true},
				"if": {"opcode":"control_if","next":"call","parent":"hat","inputs":{"CONDITION":[2,"equals"],"SUBSTACK":[2,"move"]},"fields":{},"shadow":false,"topLevel":false},
				"equals": {"opcode":"operator_equals","next":null,"parent":"if","inputs":{"OPERAND1":[1,[4,"1"]],"OPERAND2":[1,[4,"1"]]},"fields":{},"shadow":false,"topLevel":false},
				"move": {"opcode":"motion_movesteps","next":null,"parent":"if","inputs":{"STEPS":[3,"add","number-shadow"]},"fields":{},"shadow":false,"topLevel":false},
				"add": {"opcode":"operator_add","next":null,"parent":"move","inputs":{"NUM1":[1,[4,"5"]],"NUM2":[1,[4,"5"]]},"fields":{},"shadow":false,"topLevel":false},
				"number-shadow": {"opcode":"math_number","next":null,"parent":"move","inputs":{},"fields":{"NUM":["10",null]},"shadow":true,"topLevel":false},
				"call": {"opcode":"procedures_call","next":null,"parent":"if","inputs":{},"fields":{},"shadow":false,"topLevel":false,"mutation":{"proccode":"demo","argumentids":"[]"}},
				"definition": {"opcode":"procedures_definition","next":"body","parent":null,"inputs":{"custom_block":[1,"prototype"]},"fields":{},"shadow":false,"topLevel":true},
				"prototype": {"opcode":"procedures_prototype","next":null,"parent":"definition","inputs":{},"fields":{},"shadow":true,"topLevel":false,"mutation":{"proccode":"demo","argumentnames":"[]"}},
				"body": {"opcode":"motion_movesteps","next":null,"parent":"definition","inputs":{"STEPS":[1,[4,"3"]]},"fields":{},"shadow":false,"topLevel":false},
				"orphan": {"opcode":"looks_show","next":null,"parent":null,"inputs":{},"fields":{},"shadow":false,"topLevel":true}
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
	for _, id := range []string{"hat", "if", "equals", "move", "add", "number-shadow", "call", "definition", "prototype", "body", "orphan"} {
		marker := `id="` + id + `"`
		if count := strings.Count(py, marker); count != 1 {
			t.Errorf("block %q should have exactly one source association, got %d\n%s", id, count, py)
		}
	}
	assertContains(t, py, `# sb3topy:block id="equals" opcode="operator_equals" kind=reporter shadow=false`)
	assertContains(t, py, `# sb3topy:block id="add" opcode="operator_add" kind=reporter shadow=false`)
	assertContains(t, py, `# sb3topy:block id="number-shadow" opcode="math_number" kind=unmapped shadow=true`)
	assertContains(t, py, `# sb3topy:block id="move" opcode="motion_movesteps" kind=stack shadow=false`)
	assertContains(t, py, `# sb3topy:block id="prototype" opcode="procedures_prototype" kind=unmapped shadow=true`)
	assertContains(t, py, `# sb3topy:block id="call" opcode="procedures_call" kind=stack shadow=false`)
	equalsMarker := strings.Index(py, `# sb3topy:block id="equals"`)
	ifMarker := strings.Index(py, `# sb3topy:block id="if"`)
	ifCode := strings.Index(py, "if tobool(eq(1, 1)):")
	if equalsMarker < 0 || ifMarker < 0 || ifCode < 0 || !(equalsMarker < ifMarker && ifMarker < ifCode) {
		t.Fatalf("condition reporter marker should be adjacent to and precede its stack block:\n%s", py)
	}
}
