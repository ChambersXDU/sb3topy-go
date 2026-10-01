package converter

import (
	"os"
	"os/exec"
	"path/filepath"
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

func TestTranspileResolvesSpecSwitchesAndProducesValidPython(t *testing.T) {
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
			"variables": {"v": ["value", 0]},
			"lists": {},
			"broadcasts": {},
			"blocks": {
				"hat": {"opcode":"event_whenflagclicked","next":"front","parent":null,"inputs":{},"fields":{},"shadow":false,"topLevel":true},
				"front": {"opcode":"looks_gotofrontback","next":"layers","parent":"hat","inputs":{},"fields":{"FRONT_BACK":["front",null]},"shadow":false,"topLevel":false},
				"layers": {"opcode":"looks_goforwardbackwardlayers","next":"set-x","parent":"front","inputs":{"NUM":[1,[4,"2"]]},"fields":{"FORWARD_BACKWARD":["backward",null]},"shadow":false,"topLevel":false},
				"set-x": {"opcode":"motion_setx","next":"set-y","parent":"layers","inputs":{"X":[1,"x-of"]},"fields":{},"shadow":false,"topLevel":false},
				"x-of": {"opcode":"sensing_of","next":null,"parent":"set-x","inputs":{"OBJECT":[1,"object-menu"]},"fields":{"PROPERTY":["x position",null]},"shadow":false,"topLevel":false},
				"object-menu": {"opcode":"sensing_of_object_menu","next":null,"parent":"x-of","inputs":{},"fields":{"OBJECT":["Sprite",null]},"shadow":true,"topLevel":false},
				"set-y": {"opcode":"motion_sety","next":"direction","parent":"set-x","inputs":{"Y":[1,"sqrt"]},"fields":{},"shadow":false,"topLevel":false},
				"sqrt": {"opcode":"operator_mathop","next":null,"parent":"set-y","inputs":{"NUM":[1,[4,"9"]]},"fields":{"OPERATOR":["sqrt",null]},"shadow":false,"topLevel":false},
				"direction": {"opcode":"motion_pointindirection","next":"set-value","parent":"set-y","inputs":{"DIRECTION":[1,"current"]},"fields":{},"shadow":false,"topLevel":false},
				"current": {"opcode":"sensing_current","next":null,"parent":"direction","inputs":{},"fields":{"CURRENTMENU":["year",null]},"shadow":false,"topLevel":false},
				"set-value": {"opcode":"data_setvariableto","next":"bad-input","parent":"direction","inputs":{"VALUE":[1,"costume-number"]},"fields":{"VARIABLE":["value","v"]},"shadow":false,"topLevel":false},
				"costume-number": {"opcode":"looks_costumenumbername","next":null,"parent":"set-value","inputs":{},"fields":{"NUMBER_NAME":["number",null]},"shadow":false,"topLevel":false},
				"bad-input": {"opcode":"motion_setx","next":"unknown","parent":"set-value","inputs":{"X":[1,"bad-reporter"]},"fields":{},"shadow":false,"topLevel":false},
				"bad-reporter": {"opcode":"extension_reporter_not_supported","next":null,"parent":"bad-input","inputs":{},"fields":{},"shadow":false,"topLevel":false},
				"unknown": {"opcode":"extension_not_supported","next":"stop","parent":"bad-input","inputs":{},"fields":{},"shadow":false,"topLevel":false},
				"stop": {"opcode":"control_stop","next":null,"parent":"unknown","inputs":{},"fields":{"STOP_OPTION":["all",null]},"shadow":false,"topLevel":false}
			},
			"costumes": [],
			"sounds": [],
			"layerOrder": 0,
			"volume": 100,
			"visible": true
		}]
	}`)

	py, err := TranspileProject(projectJSON, sm)
	if err != nil {
		t.Fatal(err)
	}
	assertContains(t, py, "self.front_layer(util)")
	assertContains(t, py, "self.change_layer(util, -2)")
	assertContains(t, py, `self.xpos = util.sprites.get_target("Sprite").xpos`)
	assertContains(t, py, "self.ypos = sqrt(tonum(9))")
	assertContains(t, py, "self.direction = time.localtime().tm_year")
	assertContains(t, py, "self.var_value = self.costume.number")
	assertContains(t, py, `pass  # sb3topy:unsupported-input opcode="motion_setx"`)
	assertContains(t, py, `pass  # sb3topy:unsupported opcode="extension_not_supported"`)
	assertContains(t, py, "util.stop_all()\n        return None")
	assertPythonCompiles(t, py)
}

func TestTranspileCoercesCommonScratchOperatorInputs(t *testing.T) {
	specData, err := os.ReadFile("../specmap_data.json")
	if err != nil {
		t.Fatal(err)
	}
	sm, err := LoadSpecMap(specData)
	if err != nil {
		t.Fatal(err)
	}
	markers := newMarkerState(&TargetJSON{
		IsStage:   true,
		Variables: map[string]interface{}{"v": []interface{}{"value", "2"}},
	})
	variable := []interface{}{3, []interface{}{12, "value", "v"}, []interface{}{4, ""}}
	number := func(value string) []interface{} {
		return []interface{}{1, []interface{}{4, value}}
	}
	text := func(value string) []interface{} {
		return []interface{}{1, []interface{}{10, value}}
	}

	tests := []struct {
		name  string
		block *RawBlockData
		want  string
	}{
		{"multiply", &RawBlockData{Opcode: "operator_multiply", Inputs: map[string]interface{}{"NUM1": variable, "NUM2": number("3")}}, `(tonum(self.var_value) * tonum(3))`},
		{"random", &RawBlockData{Opcode: "operator_random", Inputs: map[string]interface{}{"FROM": variable, "TO": number("9")}}, `pick_rand(tonum(self.var_value), tonum(9))`},
		{"letter", &RawBlockData{Opcode: "operator_letter_of", Inputs: map[string]interface{}{"STRING": number("123"), "LETTER": text("2")}}, `letter_of(str(123), toint("2"))`},
		{"length", &RawBlockData{Opcode: "operator_length", Inputs: map[string]interface{}{"STRING": number("123")}}, `len(str(123))`},
		{"contains", &RawBlockData{Opcode: "operator_contains", Inputs: map[string]interface{}{"STRING1": text("Hello"), "STRING2": text("EL")}}, `(str("EL").lower() in str("Hello").lower())`},
		{"round", &RawBlockData{Opcode: "operator_round", Inputs: map[string]interface{}{"NUM": variable}}, `math.floor(tonum(self.var_value) + 0.5)`},
		{"mathop", &RawBlockData{Opcode: "operator_mathop", Inputs: map[string]interface{}{"NUM": variable}, Fields: map[string]interface{}{"OPERATOR": []interface{}{"sqrt", nil}}}, `sqrt(tonum(self.var_value))`},
		{"next backdrop", &RawBlockData{Opcode: "looks_nextbackdrop", Inputs: map[string]interface{}{}, Fields: map[string]interface{}{}}, "util.sprites.stage.costume.next()\nutil.send_event('backdrop_' + util.sprites.stage.costume.name, True)"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := transpileSingleBlock(test.block, map[string]*RawBlockData{}, sm, "", markers)
			if got != test.want {
				t.Fatalf("got %q, want %q", got, test.want)
			}
		})
	}
}

func TestTranspiledListBlocksExecute(t *testing.T) {
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 is not available")
	}
	specData, err := os.ReadFile("../specmap_data.json")
	if err != nil {
		t.Fatal(err)
	}
	sm, err := LoadSpecMap(specData)
	if err != nil {
		t.Fatal(err)
	}
	markers := newMarkerState(&TargetJSON{
		Lists:     map[string]interface{}{"local": []interface{}{"items", []interface{}{"a", "b"}}},
		Variables: map[string]interface{}{"index": []interface{}{"index", "last"}},
	})
	text := func(value string) []interface{} {
		return []interface{}{1, []interface{}{10, value}}
	}
	variableIndex := []interface{}{3, []interface{}{12, "index", "index"}, []interface{}{7, "1"}}
	steps := []struct {
		opcode string
		inputs map[string]interface{}
		listID string
		check  string
	}{
		{"data_itemoflist", map[string]interface{}{"INDEX": variableIndex}, "local", `assert result == "b"`},
		{"data_replaceitemoflist", map[string]interface{}{"INDEX": text("last"), "ITEM": text("B")}, "local", `assert self.list_items.list == ["a", "B"]`},
		{"data_insertatlist", map[string]interface{}{"INDEX": text("2.9"), "ITEM": text("new")}, "local", `assert self.list_items.list == ["a", "new", "B"]`},
		{"data_itemoflist", map[string]interface{}{"INDEX": text("2.9")}, "local", `assert result == "new"`},
		{"data_deleteoflist", map[string]interface{}{"INDEX": text("last")}, "local", `assert self.list_items.list == ["a", "new"]`},
		{"data_deleteoflist", map[string]interface{}{"INDEX": text("all")}, "local", `assert self.list_items.list == []`},
		{"data_itemoflist", map[string]interface{}{"INDEX": text("random")}, "local", `assert result == ""`},
		{"data_addtolist", map[string]interface{}{"ITEM": []interface{}{1, []interface{}{4, "2"}}}, "local", `assert self.list_items.list == [2]`},
		{"data_addtolist", map[string]interface{}{"ITEM": text("A")}, "local", `assert self.list_items.list == [2, "A"]`},
		{"data_listcontents", map[string]interface{}{}, "local", `assert result == "2 A"`},
		{"data_itemoflist", map[string]interface{}{"INDEX": text("last")}, "global", `assert result == "stage item"`},
		{"data_insertatlist", map[string]interface{}{"INDEX": text("last"), "ITEM": text("new")}, "global", `assert util.sprites.stage.list_items.list == ["stage item", "new"]`},
	}
	var script strings.Builder
	script.WriteString(`import sys
from types import SimpleNamespace
sys.path.insert(0, sys.argv[1])
from test_lists import List
self = SimpleNamespace(list_items=List(["a", "b"]), var_index="last")
util = SimpleNamespace(sprites=SimpleNamespace(stage=SimpleNamespace(list_items=List(["stage item"]))))
`)
	for _, step := range steps {
		block := &RawBlockData{
			Opcode: step.opcode,
			Inputs: step.inputs,
			Fields: map[string]interface{}{"LIST": []interface{}{"items", step.listID}},
		}
		code := transpileSingleBlock(block, map[string]*RawBlockData{}, sm, "", markers)
		if step.opcode == "data_itemoflist" || step.opcode == "data_listcontents" {
			script.WriteString("result = ")
		}
		script.WriteString(code + "\n" + step.check + "\n")
	}
	testsDir, err := filepath.Abs("../tests")
	if err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command(python, "-B", "-c", script.String(), testsDir).CombinedOutput(); err != nil {
		t.Fatalf("transpiled list operations failed: %v\n%s\n%s", err, output, script.String())
	}
}

func TestTranspiledWaitAndMoveBlocksExecute(t *testing.T) {
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 is not available")
	}
	specData, err := os.ReadFile("../specmap_data.json")
	if err != nil {
		t.Fatal(err)
	}
	sm, err := LoadSpecMap(specData)
	if err != nil {
		t.Fatal(err)
	}
	markers := newMarkerState(&TargetJSON{
		IsStage: true,
		Variables: map[string]interface{}{
			"wait": []interface{}{"wait", "0.032"},
		},
	})
	blocks := map[string]*RawBlockData{
		"interval": {
			Opcode: "argument_reporter_string_number",
			Fields: map[string]interface{}{"VALUE": []interface{}{"interval", nil}},
		},
	}
	text := func(value string) []interface{} {
		return []interface{}{1, []interface{}{10, value}}
	}
	inputs := []interface{}{
		[]interface{}{1, []interface{}{4, "0"}},
		text("2"),
		text(" 0.125 "),
		text("-2"),
		text(""),
		text("not a number"),
		[]interface{}{3, []interface{}{12, "wait", "wait"}, []interface{}{4, "1"}},
		[]interface{}{1, "interval"},
	}
	var script strings.Builder
	script.WriteString(`import asyncio
import runpy
import sys
tonum = runpy.run_path(sys.argv[1])["tonum"]
class Target:
    var_wait = "0.032"
    def __init__(self):
        self.delays = []
        self.steps = []
    async def sleep(self, duration):
        assert isinstance(duration, (int, float)), repr(duration)
        self.delays.append(duration)
        await asyncio.sleep(0)
    def move(self, steps):
        assert isinstance(steps, (int, float)), repr(steps)
        self.steps.append(steps)
self = Target()
interval = "2"
async def run():
`)
	for _, input := range inputs {
		block := &RawBlockData{Opcode: "control_wait", Inputs: map[string]interface{}{"DURATION": input}}
		code := transpileSingleBlock(block, blocks, sm, "    ", markers)
		script.WriteString(code + "\n")
		move := &RawBlockData{Opcode: "motion_movesteps", Inputs: map[string]interface{}{"STEPS": input}}
		script.WriteString(transpileSingleBlock(move, blocks, sm, "    ", markers) + "\n")
	}
	script.WriteString(`asyncio.run(run())
assert self.delays == [0, 2, 0.125, 0, 0, 0, 0.032, 2], self.delays
assert self.steps == [0, 2, 0.125, -2, 0, 0, 0.032, 2], self.steps
`)
	operators, err := filepath.Abs("../engine/operators.py")
	if err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command(python, "-B", "-c", script.String(), operators).CombinedOutput(); err != nil {
		t.Fatalf("transpiled waits and moves failed: %v\n%s\n%s", err, output, script.String())
	}
}

func assertPythonCompiles(t *testing.T, source string) {
	t.Helper()
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 is not available")
	}
	path := filepath.Join(t.TempDir(), "project.py")
	if err := os.WriteFile(path, []byte(source), 0644); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command(python, "-m", "py_compile", path).CombinedOutput(); err != nil {
		t.Fatalf("generated Python is invalid: %v\n%s\n%s", err, output, source)
	}
}
