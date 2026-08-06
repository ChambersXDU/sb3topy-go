package converter

import (
	"os"
	"testing"
)

func TestSpecMapLoading(t *testing.T) {
	data, err := os.ReadFile("../specmap_data.json")
	if err != nil {
		t.Fatalf("Failed to read specmap_data.json: %v", err)
	}

	sm, err := LoadSpecMap(data)
	if err != nil {
		t.Fatalf("Failed to parse specmap_data.json: %v", err)
	}

	// Verify known opcodes exist
	opcodes := []string{
		"event_whenflagclicked",
		"motion_movesteps",
		"control_forever",
		"control_if",
		"data_setvariableto",
	}

	for _, op := range opcodes {
		spec, ok := sm.Get(op)
		if !ok {
			t.Errorf("Expected opcode %s in SpecMap", op)
		}
		if spec.Type == "" {
			t.Errorf("Opcode %s has empty type", op)
		}
	}

	if !sm.IsHat("event_whenflagclicked") {
		t.Errorf("event_whenflagclicked should be hat block")
	}
	if !sm.IsLoop("control_forever") {
		t.Errorf("control_forever should be loop block")
	}
}

func TestSpecMapFormatting(t *testing.T) {
	data, err := os.ReadFile("../specmap_data.json")
	if err != nil {
		t.Fatalf("Failed to read specmap_data.json: %v", err)
	}

	sm, err := LoadSpecMap(data)
	if err != nil {
		t.Fatalf("Failed to parse specmap_data.json: %v", err)
	}

	// Test motion_movesteps formatting
	code := sm.FormatCode("motion_movesteps", map[string]string{"STEPS": "10"})
	if code != "self.move(10)" {
		t.Errorf("FormatCode motion_movesteps = %q; want %q", code, "self.move(10)")
	}
}
