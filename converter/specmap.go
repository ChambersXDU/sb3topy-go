package converter

import (
	"encoding/json"
	"fmt"
	"strings"
)

// BlockSpec defines mapping rules from JSON specmap
type BlockSpec struct {
	Type       string            `json:"type"`
	Args       map[string]string `json:"args"`
	CodeRaw    interface{}       `json:"code"`
	CodeStr    string
	CodeLines  []string
	Switch     string `json:"switch"`
	Basename   string `json:"basename"`
	Support    string `json:"support,omitempty"`
	Limitation string `json:"limitation,omitempty"`
}

// SpecMap holds opcode to BlockSpec mappings
type SpecMap struct {
	opcodes map[string]*BlockSpec
}

var hatOpcodes = map[string]bool{
	"procedures_definition":        true,
	"event_whenflagclicked":        true,
	"event_whenkeypressed":         true,
	"event_whenthisspriteclicked":  true,
	"event_whenstageclicked":       true,
	"event_whenbackdropswitchesto": true,
	"event_whengreaterthan":        true,
	"event_whenbroadcastreceived":  true,
	"control_start_as_clone":       true,
}

var loopOpcodes = map[string]bool{
	"control_repeat":       true,
	"control_forever":      true,
	"control_repeat_until": true,
	"control_for_each":     true,
	"control_while":        true,
}

// LoadSpecMap parses embedded json into SpecMap struct
func LoadSpecMap(jsonData []byte) (*SpecMap, error) {
	var rawMap map[string]json.RawMessage
	if err := json.Unmarshal(jsonData, &rawMap); err != nil {
		return nil, fmt.Errorf("invalid specmap json: %w", err)
	}

	sm := &SpecMap{opcodes: make(map[string]*BlockSpec)}

	for opcode, rawBlock := range rawMap {
		var spec BlockSpec
		if err := json.Unmarshal(rawBlock, &spec); err != nil {
			continue
		}

		// Handle string or []string for code
		if spec.CodeRaw != nil {
			switch v := spec.CodeRaw.(type) {
			case string:
				spec.CodeStr = v
				spec.CodeLines = strings.Split(v, "\n")
			case []interface{}:
				lines := make([]string, 0, len(v))
				for _, line := range v {
					lines = append(lines, fmt.Sprint(line))
				}
				spec.CodeLines = lines
				spec.CodeStr = strings.Join(lines, "\n")
			}
		}

		sm.opcodes[opcode] = &spec
	}

	return sm, nil
}

// Get returns the BlockSpec for an opcode
func (sm *SpecMap) Get(opcode string) (*BlockSpec, bool) {
	spec, ok := sm.opcodes[opcode]
	return spec, ok
}

// IsHat checks if an opcode is a hat block
func (sm *SpecMap) IsHat(opcode string) bool {
	if hatOpcodes[opcode] {
		return true
	}
	if spec, ok := sm.opcodes[opcode]; ok {
		return spec.Type == "hat"
	}
	return false
}

// IsLoop checks if an opcode is a loop block
func (sm *SpecMap) IsLoop(opcode string) bool {
	if loopOpcodes[opcode] {
		return true
	}
	if spec, ok := sm.opcodes[opcode]; ok {
		return spec.Type == "loop"
	}
	return false
}

// FormatCode formats template string with given args map
func (sm *SpecMap) FormatCode(opcode string, args map[string]string) string {
	spec, ok := sm.opcodes[opcode]
	if !ok || spec.CodeStr == "" {
		return ""
	}

	res := spec.CodeStr
	for k, v := range args {
		placeholder := "{" + k + "}"
		res = strings.ReplaceAll(res, placeholder, v)
	}
	return res
}
