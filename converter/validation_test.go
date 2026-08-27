package converter

import (
	"strings"
	"testing"
)

func TestValidateProjectJSONRejectsBrokenBlockGraphs(t *testing.T) {
	tests := []struct {
		name    string
		blocks  string
		message string
	}{
		{
			name: "missing next",
			blocks: `{
				"hat":{"opcode":"event_whenflagclicked","next":"missing","parent":null,"inputs":{},"fields":{},"shadow":false,"topLevel":true}
			}`,
			message: "references a missing block",
		},
		{
			name: "missing parent",
			blocks: `{
				"move":{"opcode":"motion_movesteps","next":null,"parent":"missing","inputs":{},"fields":{},"shadow":false,"topLevel":false}
			}`,
			message: "references a missing block",
		},
		{
			name: "missing substack",
			blocks: `{
				"hat":{"opcode":"event_whenflagclicked","next":"repeat","parent":null,"inputs":{},"fields":{},"shadow":false,"topLevel":true},
				"repeat":{"opcode":"control_repeat","next":null,"parent":"hat","inputs":{"SUBSTACK":[2,"missing"]},"fields":{},"shadow":false,"topLevel":false}
			}`,
			message: "input references missing block",
		},
		{
			name: "top-level parent",
			blocks: `{
				"a":{"opcode":"looks_show","next":"b","parent":null,"inputs":{},"fields":{},"shadow":false,"topLevel":true},
				"b":{"opcode":"looks_hide","next":null,"parent":"a","inputs":{},"fields":{},"shadow":false,"topLevel":true}
			}`,
			message: "top-level block",
		},
		{
			name: "cycle",
			blocks: `{
				"a":{"opcode":"looks_show","next":"b","parent":"b","inputs":{},"fields":{},"shadow":false,"topLevel":false},
				"b":{"opcode":"looks_hide","next":"a","parent":"a","inputs":{},"fields":{},"shadow":false,"topLevel":false}
			}`,
			message: "cycle detected",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			project := []byte(`{"targets":[{"isStage":true,"name":"Stage","variables":{},"lists":{},"broadcasts":{},"blocks":` + test.blocks + `,"costumes":[],"sounds":[],"layerOrder":0}]}`)
			err := validateProjectJSON(project)
			if err == nil || !strings.Contains(err.Error(), test.message) {
				t.Fatalf("expected error containing %q, got %v", test.message, err)
			}
		})
	}
}
