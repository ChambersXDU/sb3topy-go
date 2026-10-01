package converter

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

func TestUnsupportedInputsPropagateWithoutInventingDefaults(t *testing.T) {
	for _, tc := range []struct{ opcode, key, extra string }{
		{"motion_movesteps", "STEPS", ""},
		{"data_setvariableto", "VALUE", `,"fields":{"VARIABLE":["value","v"]}`},
		{"control_if", "CONDITION", ""},
		{"control_wait", "DURATION", ""},
		{"looks_sayforsecs", "SECS", ""},
	} {
		t.Run(tc.opcode, func(t *testing.T) {
			project := []byte(fmt.Sprintf(`{"targets":[{"isStage":true,"name":"Stage","variables":{"v":["value",7]},"blocks":{
			"hat":{"opcode":"event_whenflagclicked","topLevel":true,"parent":null,"next":"parent"},
			"parent":{"opcode":%q,"parent":"hat","next":"after","inputs":{%q:[2,"add"],"MESSAGE":[1,[10,"hello"]]}%s},
			"add":{"opcode":"operator_add","parent":"parent","inputs":{"NUM1":[2,"unknown"],"NUM2":[1,[4,"5"]]}},
			"unknown":{"opcode":"extension_reporter_unknown","parent":"add"},
			"after":{"opcode":"looks_show","parent":"parent"}
			}}]}`, tc.opcode, tc.key, tc.extra))
			if tc.opcode == "control_if" {
				var root map[string]interface{}
				if err := json.Unmarshal(project, &root); err != nil {
					t.Fatal(err)
				}
				blocks := root["targets"].([]interface{})[0].(map[string]interface{})["blocks"].(map[string]interface{})
				blocks["parent"].(map[string]interface{})["inputs"].(map[string]interface{})["SUBSTACK"] = []interface{}{2, "body"}
				blocks["body"] = map[string]interface{}{"opcode": "looks_hide", "parent": "parent"}
				project, _ = json.Marshal(root)
			}
			python, err := TranspileProject(project, mustInspectionSpecMap(t))
			if err != nil {
				t.Fatal(err)
			}
			assertContains(t, python, `pass  # sb3topy:unsupported-input opcode="`+tc.opcode+`"`)
			assertContains(t, python, "self.shown = True")
			if tc.opcode == "control_if" {
				assertContains(t, python, `id="body" opcode="looks_hide" kind=unmapped`)
			}
			for _, id := range []string{"parent", "add", "unknown", "after"} {
				if strings.Count(python, "id="+QuoteString(id)) != 1 {
					t.Fatalf("source association lost or repeated for %s", id)
				}
			}
			path := filepath.Join(t.TempDir(), "input.sb3")
			writeSB3Fixture(t, path, project, nil)
			report, err := Inspect(InspectOptions{Path: path, SpecmapData: inspectionSpec(t), BlockID: "parent"})
			if err != nil {
				t.Fatal(err)
			}
			if report.Blocks[0].TranslationStatus != "unsupported" || len(report.Issues) != 1 || report.Issues[0].Code != "unsupported_input" {
				t.Fatalf("unsupported input was hidden: %+v", report)
			}
		})
	}
}

func TestInspectDistinguishesTimerThresholdLimitations(t *testing.T) {
	for _, tc := range []struct{ source, input, status, issue string }{
		{"TIMER", `[1,[10,"0.1"]]`, "partial", "partial_block"},
		{"LOUDNESS", `[1,[10,"10"]]`, "unsupported", "unsupported_block"},
		{"TIMER", `[2,"unknown"]`, "unsupported", "unsupported_input"},
	} {
		t.Run(tc.source+tc.issue, func(t *testing.T) {
			unknown := ""
			if tc.input == `[2,"unknown"]` {
				unknown = `,"unknown":{"opcode":"extension_reporter_unknown","topLevel":false,"parent":"hat"}`
			}
			project := []byte(fmt.Sprintf(`{"targets":[{"isStage":true,"name":"Stage","blocks":{
			"hat":{"opcode":"event_whengreaterthan","topLevel":true,"parent":null,"fields":{"WHENGREATERTHANMENU":[%q,null]},"inputs":{"VALUE":%s}}%s
			}}]}`, tc.source, tc.input, unknown))
			path := filepath.Join(t.TempDir(), "timer.sb3")
			writeSB3Fixture(t, path, project, nil)
			report, err := Inspect(InspectOptions{Path: path, SpecmapData: inspectionSpec(t), BlockID: "hat"})
			if err != nil {
				t.Fatal(err)
			}
			if !report.Graph.OK || report.Blocks[0].TranslationStatus != tc.status || len(report.Issues) != 1 || report.Issues[0].Code != tc.issue {
				t.Fatalf("unexpected threshold diagnosis: %+v", report)
			}
		})
	}
}
