package converter

import (
	"os"
	"strings"
	"testing"
)

func TestTranspileUsesLocalDataAndPreservesTextLiterals(t *testing.T) {
	specData, err := os.ReadFile("../specmap_data.json")
	if err != nil {
		t.Fatal(err)
	}
	sm, err := LoadSpecMap(specData)
	if err != nil {
		t.Fatal(err)
	}

	projectJSON := []byte(`{
		"targets": [
			{
				"isStage": true,
				"name": "Stage",
				"variables": {"g": ["score", "000"]},
				"lists": {},
				"broadcasts": {},
				"blocks": {},
				"costumes": [],
				"sounds": [],
				"layerOrder": 0,
				"volume": 100,
				"visible": true
			},
			{
				"isStage": false,
				"name": "Sprite",
				"variables": {"l": ["local", 0]},
				"lists": {"ll": ["items", ["001"]]},
				"broadcasts": {},
				"blocks": {
					"hat": {
						"opcode": "event_whenflagclicked",
						"next": "set-local",
						"parent": null,
						"inputs": {},
						"fields": {},
						"shadow": false,
						"topLevel": true
					},
					"set-local": {
						"opcode": "data_setvariableto",
						"next": "change-local",
						"parent": "hat",
						"inputs": {"VALUE": [1, [10, "001"]]},
						"fields": {"VARIABLE": ["local", "l"]},
						"shadow": false,
						"topLevel": false
					},
					"change-local": {
						"opcode": "data_changevariableby",
						"next": "add-list",
						"parent": "set-local",
						"inputs": {"VALUE": [1, [4, "2"]]},
						"fields": {"VARIABLE": ["local", "l"]},
						"shadow": false,
						"topLevel": false
					},
					"add-list": {
						"opcode": "data_addtolist",
						"next": "delete-list",
						"parent": "change-local",
						"inputs": {"ITEM": [1, [10, "007"]]},
						"fields": {"LIST": ["items", "ll"]},
						"shadow": false,
						"topLevel": false
					},
					"delete-list": {
						"opcode": "data_deleteoflist",
						"next": "set-global",
						"parent": "add-list",
						"inputs": {"INDEX": [1, [7, "1"]]},
						"fields": {"LIST": ["items", "ll"]},
						"shadow": false,
						"topLevel": false
					},
					"set-global": {
						"opcode": "data_setvariableto",
						"next": "say-list",
						"parent": "delete-list",
						"inputs": {"VALUE": [3, [12, "local", "l"], [10, ""]]},
						"fields": {"VARIABLE": ["score", "g"]},
						"shadow": false,
						"topLevel": false
					},
					"say-list": {
						"opcode": "looks_say",
						"next": null,
						"parent": "set-global",
						"inputs": {"MESSAGE": [1, "list-reporter"]},
						"fields": {},
						"shadow": false,
						"topLevel": false
					},
					"list-reporter": {
						"opcode": "data_listcontents",
						"next": null,
						"parent": "say-list",
						"inputs": {},
						"fields": {"LIST": ["items", "ll"]},
						"shadow": false,
						"topLevel": false
					}
				},
				"costumes": [],
				"sounds": [],
				"layerOrder": 1,
				"volume": 100,
				"x": 0,
				"y": 0,
				"size": 100,
				"direction": 90,
				"visible": true,
				"rotationStyle": "all around"
			}
		]
	}`)

	py, err := TranspileProject(projectJSON, sm)
	if err != nil {
		t.Fatal(err)
	}

	assertContains(t, py, `self.var_score = "000"`)
	assertContains(t, py, `self.list_items = List(
            ["001"]`)
	assertContains(t, py, `self.var_local = "001"`)
	assertContains(t, py, `self.var_local = tonum(self.var_local) + tonum(2)`)
	assertContains(t, py, `self.list_items.append("007")`)
	assertContains(t, py, `self.list_items.delete(1)`)
	assertContains(t, py, `util.sprites.stage.var_score = self.var_local`)
	assertContains(t, py, `self.list_items.join()`)
}

func TestTranspileMapsCommonHatsMenusAndFields(t *testing.T) {
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
			"name": "Sprite\"\"\"Name",
			"variables": {},
			"lists": {},
			"broadcasts": {},
			"blocks": {
				"click": {
					"opcode": "event_whenthisspriteclicked",
					"next": "clone",
					"parent": null,
					"inputs": {},
					"fields": {},
					"shadow": false,
					"topLevel": true
				},
				"clone": {
					"opcode": "control_create_clone_of",
					"next": "rotation",
					"parent": "click",
					"inputs": {"CLONE_OPTION": [1, "clone-menu"]},
					"fields": {},
					"shadow": false,
					"topLevel": false
				},
				"clone-menu": {
					"opcode": "control_create_clone_of_menu",
					"next": null,
					"parent": "clone",
					"inputs": {},
					"fields": {"CLONE_OPTION": ["_myself_", null]},
					"shadow": true,
					"topLevel": false
				},
				"rotation": {
					"opcode": "motion_setrotationstyle",
					"next": null,
					"parent": "clone",
					"inputs": {},
					"fields": {"STYLE": ["left-right", null]},
					"shadow": false,
					"topLevel": false
				},
				"backdrop": {
					"opcode": "event_whenbackdropswitchesto",
					"next": null,
					"parent": null,
					"inputs": {},
					"fields": {"BACKDROP": ["Backdrop 1", null]},
					"shadow": false,
					"topLevel": true
				},
				"greater": {
					"opcode": "event_whengreaterthan",
					"next": null,
					"parent": null,
					"inputs": {"VALUE": [1, [4, "10"]]},
					"fields": {"WHENGREATERTHANMENU": ["TIMER", null]},
					"shadow": false,
					"topLevel": true
				}
			},
			"costumes": [],
			"sounds": [],
			"layerOrder": 1,
			"volume": 100,
			"x": 0,
			"y": 0,
			"size": 100,
			"direction": 90,
			"visible": true,
			"rotationStyle": "all around"
		}]
	}`)

	py, err := TranspileProject(projectJSON, sm)
	if err != nil {
		t.Fatal(err)
	}

	assertContains(t, py, `@on_clicked`)
	assertContains(t, py, `@on_backdrop('Backdrop 1')`)
	assertContains(t, py, `@on_greater('timer', 10)`)
	assertContains(t, py, `self.create_clone_of(util, "_myself_")`)
	assertContains(t, py, `self.costume.rotation_style = 'left-right'`)
	assertContains(t, py, `"Sprite Sprite\"\"\"Name"`)
	if strings.Contains(py, "@on_green_flag") {
		t.Fatalf("non-green-flag hats were incorrectly compiled as green flag:\n%s", py)
	}
	if strings.Contains(py, `self.create_clone_of(util, "clone-menu")`) {
		t.Fatalf("clone menu block ID leaked into generated code:\n%s", py)
	}
}
