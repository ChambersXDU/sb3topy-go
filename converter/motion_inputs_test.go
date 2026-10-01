package converter

import (
	"fmt"
	"strings"
	"testing"
)

func TestMotionNumericInputsIncludeVariablesAndProcedureArguments(t *testing.T) {
	sm := mustInspectionSpecMap(t)
	for _, tc := range []struct {
		opcode, expected string
		keys             []string
	}{
		{"motion_turnright", "self.direction += tonum(%s)", []string{"DEGREES"}},
		{"motion_turnleft", "self.direction -= tonum(%s)", []string{"DEGREES"}},
		{"motion_pointindirection", "self.direction = tonum(%s)", []string{"DIRECTION"}},
		{"motion_changexby", "self.xpos += tonum(%s)", []string{"DX"}},
		{"motion_changeyby", "self.ypos += tonum(%s)", []string{"DY"}},
		{"motion_setx", "self.xpos = tonum(%s)", []string{"X"}},
		{"motion_sety", "self.ypos = tonum(%s)", []string{"Y"}},
		{"motion_gotoxy", "self.gotoxy(tonum(%s), tonum(%s))", []string{"X", "Y"}},
		{"motion_glidesecstoxy", "await self.glide(tonum(%s), tonum(%s), tonum(%s))", []string{"SECS", "X", "Y"}},
		{"motion_glideto", `await self.glideto(util, tonum(%s), "_mouse_")`, []string{"SECS"}},
	} {
		for _, mode := range []string{"text", "variable", "argument"} {
			t.Run(tc.opcode+"/"+mode, func(t *testing.T) {
				block := &RawBlockData{Opcode: tc.opcode, Inputs: map[string]interface{}{}}
				blocks := map[string]*RawBlockData{}
				var input interface{} = []interface{}{1, []interface{}{10, "-2.5"}}
				expression := `"-2.5"`
				if mode == "variable" {
					input = []interface{}{3, []interface{}{12, "amount", "v"}, []interface{}{10, ""}}
					expression = "self.var_amount"
				} else if mode == "argument" {
					input = []interface{}{2, "argument"}
					blocks["argument"] = &RawBlockData{Opcode: "argument_reporter_string_number", Fields: map[string]interface{}{"VALUE": []interface{}{"amount", nil}}}
					expression = "amount"
				}
				args := make([]interface{}, len(tc.keys))
				for i, key := range tc.keys {
					block.Inputs[key] = input
					args[i] = expression
				}
				if tc.opcode == "motion_glideto" {
					block.Inputs["TO"] = []interface{}{1, []interface{}{10, "_mouse_"}}
				}
				actual := strings.TrimSpace(transpileSingleBlock(block, blocks, sm, "", newMarkerState(&TargetJSON{IsStage: true})))
				if expected := fmt.Sprintf(tc.expected, args...); actual != expected {
					t.Fatalf("got %s; want %s", actual, expected)
				}
			})
		}
	}
}
