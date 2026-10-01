package converter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"
)

// validateProjectJSON checks the parts of the Scratch block graph that the
// converter traverses. The original JSON is still kept as opaque bytes for
// round-tripping; these checks only prevent malformed input from becoming an
// infinite traversal or an obviously broken SB3.
func validateProjectJSON(projectJSON []byte) error {
	var root map[string]json.RawMessage
	decoder := json.NewDecoder(bytes.NewReader(projectJSON))
	if err := decoder.Decode(&root); err != nil {
		return fmt.Errorf("invalid project.json: %w", err)
	}
	var extra interface{}
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return fmt.Errorf("invalid project.json: multiple JSON values")
		}
		return fmt.Errorf("invalid project.json: %w", err)
	}

	rawTargets, ok := root["targets"]
	if !ok {
		return fmt.Errorf("project.json is missing the targets array")
	}
	var targets []json.RawMessage
	if err := json.Unmarshal(rawTargets, &targets); err != nil {
		return fmt.Errorf("project.json targets must be an array: %w", err)
	}
	if len(targets) == 0 {
		return fmt.Errorf("project.json targets must not be empty")
	}

	for targetIndex, rawTarget := range targets {
		if err := validateTargetBlocks(rawTarget, targetIndex); err != nil {
			return err
		}
	}
	return nil
}

func validateTargetBlocks(rawTarget json.RawMessage, targetIndex int) error {
	var target map[string]json.RawMessage
	if err := json.Unmarshal(rawTarget, &target); err != nil {
		return fmt.Errorf("target[%d] must be a JSON object: %w", targetIndex, err)
	}

	blocks := make(map[string]*RawBlockData)
	if rawBlocks, ok := target["blocks"]; ok {
		var rawBlockMap map[string]json.RawMessage
		if err := json.Unmarshal(rawBlocks, &rawBlockMap); err != nil {
			return fmt.Errorf("target[%d] blocks must be an object: %w", targetIndex, err)
		}
		for blockID, rawBlock := range rawBlockMap {
			block, ok := ParseRawBlock(rawBlock)
			if !ok {
				return fmt.Errorf("target[%d] block %q is not a valid block object", targetIndex, blockID)
			}
			blocks[blockID] = block
		}
	}

	ids := make([]string, 0, len(blocks))
	for id := range blocks {
		ids = append(ids, id)
	}
	linkLegacyProcedureShadows(blocks)
	sort.Strings(ids)
	for _, id := range ids {
		block := blocks[id]
		if link, err := optionalBlockLink(block.Next, blocks, "next"); err != nil {
			return fmt.Errorf("target[%d] block %q: %w", targetIndex, id, err)
		} else if link != "" && blocks[link].Parent == nil {
			return fmt.Errorf("target[%d] block %q has next=%q, but the target block has no parent", targetIndex, id, link)
		} else if link != "" && blockID(blocks[link].Parent) != id {
			return fmt.Errorf("target[%d] block %q has next=%q, but the target parent is %q", targetIndex, id, link, blockID(blocks[link].Parent))
		}

		if block.TopLevel {
			if block.Parent != nil {
				return fmt.Errorf("target[%d] top-level block %q must not have parent=%q", targetIndex, id, blockID(block.Parent))
			}
		} else if block.Parent == nil {
			return fmt.Errorf("target[%d] non-top-level block %q has no parent", targetIndex, id)
		}

		if parentID, err := optionalBlockLink(block.Parent, blocks, "parent"); err != nil {
			return fmt.Errorf("target[%d] block %q: %w", targetIndex, id, err)
		} else if parentID != "" {
			if !containsBlockReference(blocks[parentID], id) {
				return fmt.Errorf("target[%d] block %q has parent=%q, but that parent does not reference it", targetIndex, id, parentID)
			}
		}

		for _, childID := range inputBlockReferenceIDs(block) {
			if _, exists := blocks[childID]; !exists {
				return fmt.Errorf("target[%d] block %q input references missing block %q", targetIndex, id, childID)
			}
		}
		for _, childID := range inputBlockReferences(block, blocks) {
			child := blocks[childID]
			// Scratch can retain a detached top-level shadow as the fallback
			// (third item) of an input tuple, especially around procedures.
			if child.TopLevel && child.Parent == nil {
				continue
			}
			if child.Parent == nil || blockID(child.Parent) != id {
				return fmt.Errorf("target[%d] block %q input references %q, but the child parent is %q", targetIndex, id, childID, blockID(child.Parent))
			}
		}
	}

	if err := validateBlockGraphAcyclic(blocks); err != nil {
		return fmt.Errorf("target[%d] has an invalid block graph: %w", targetIndex, err)
	}
	return nil
}

// Legacy Scratch projects can omit prototype parents and their argument input
// links. Scratch resolves procedures from definitions and mutation metadata.
// Reconstruct only this unambiguous shape in the validation view, never in the
// canonical JSON retained for round-tripping.
func linkLegacyProcedureShadows(blocks map[string]*RawBlockData) {
	for prototypeID, prototype := range blocks {
		if prototype.Opcode != "procedures_prototype" || !prototype.Shadow || prototype.TopLevel || prototype.Parent != nil || prototype.Next != nil || len(prototype.Inputs) != 0 {
			continue
		}
		var names, argumentIDs []string
		proccode, codeOK := prototype.Mutation["proccode"].(string)
		namesJSON, namesOK := prototype.Mutation["argumentnames"].(string)
		idsJSON, idsOK := prototype.Mutation["argumentids"].(string)
		if !codeOK || proccode == "" || !namesOK || !idsOK || json.Unmarshal([]byte(namesJSON), &names) != nil || json.Unmarshal([]byte(idsJSON), &argumentIDs) != nil || names == nil || argumentIDs == nil || len(names) != len(argumentIDs) {
			continue
		}
		seenIDs, seenNames, valid := map[string]bool{}, map[string]bool{}, true
		for i, name := range names {
			if name == "" || argumentIDs[i] == "" || seenIDs[argumentIDs[i]] || seenNames[name] {
				valid = false
				break
			}
			seenIDs[argumentIDs[i]], seenNames[name] = true, true
		}
		if !valid {
			continue
		}
		definitionID := ""
		definitions := 0
		for id, definition := range blocks {
			if definition.Opcode == "procedures_definition" && definition.TopLevel && definition.Parent == nil && getSubstack(definition, "custom_block") == prototypeID {
				definitionID = id
				definitions++
			}
		}
		if definitions != 1 {
			continue
		}
		prototype.Parent = definitionID
		inputs := map[string]interface{}{}
		for i, name := range names {
			shadowID, shadowCount := "", 0
			for id, shadow := range blocks {
				if blockID(shadow.Parent) == prototypeID && shadow.Shadow && !shadow.TopLevel && shadow.Next == nil && len(shadow.Inputs) == 0 &&
					(shadow.Opcode == "argument_reporter_string_number" || shadow.Opcode == "argument_reporter_boolean") && getFieldVal(shadow, "VALUE", "") == name {
					shadowID, shadowCount = id, shadowCount+1
				}
			}
			if shadowCount == 1 {
				inputs[argumentIDs[i]] = []interface{}{1, shadowID}
			}
		}
		prototype.Inputs = inputs
	}
}

func optionalBlockLink(value interface{}, blocks map[string]*RawBlockData, label string) (string, error) {
	if value == nil {
		return "", nil
	}
	id, ok := value.(string)
	if !ok || id == "" {
		return "", fmt.Errorf("%s must be null or a non-empty block ID string", label)
	}
	if _, exists := blocks[id]; !exists {
		return "", fmt.Errorf("%s=%q references a missing block", label, id)
	}
	return id, nil
}

func blockID(value interface{}) string {
	id, _ := value.(string)
	return id
}

func containsBlockReference(block *RawBlockData, wanted string) bool {
	if block == nil {
		return false
	}
	if nextID, _ := block.Next.(string); nextID == wanted {
		return true
	}
	for _, id := range inputBlockReferences(block, map[string]*RawBlockData{wanted: {}}) {
		if id == wanted {
			return true
		}
	}
	return false
}

func inputBlockReferences(block *RawBlockData, blocks map[string]*RawBlockData) []string {
	var refs []string
	for _, id := range inputBlockReferenceIDs(block) {
		if _, ok := blocks[id]; ok {
			refs = append(refs, id)
		}
	}
	return refs
}

func inputBlockReferenceIDs(block *RawBlockData) []string {
	if block == nil || block.Inputs == nil {
		return nil
	}
	seen := make(map[string]bool)
	var refs []string
	var visitReference func(interface{})
	visitReference = func(value interface{}) {
		switch value := value.(type) {
		case string:
			if value != "" && !seen[value] {
				seen[value] = true
				refs = append(refs, value)
			}
		case []interface{}:
			if len(value) >= 2 && toInt(value[0]) >= 1 && toInt(value[0]) <= 3 {
				visitReference(value[1])
				if len(value) >= 3 {
					visitReference(value[2])
				}
			}
		}
	}

	keys := make([]string, 0, len(block.Inputs))
	for key := range block.Inputs {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		visitReference(block.Inputs[key])
	}
	return refs
}

func validateBlockGraphAcyclic(blocks map[string]*RawBlockData) error {
	state := make(map[string]uint8, len(blocks))
	var visit func(string) error
	visit = func(id string) error {
		switch state[id] {
		case 1:
			return fmt.Errorf("cycle detected at block %q", id)
		case 2:
			return nil
		}
		state[id] = 1
		block := blocks[id]
		if nextID, ok := block.Next.(string); ok && nextID != "" {
			if err := visit(nextID); err != nil {
				return err
			}
		}
		for _, childID := range inputBlockReferences(block, blocks) {
			if err := visit(childID); err != nil {
				return err
			}
		}
		state[id] = 2
		return nil
	}

	ids := make([]string, 0, len(blocks))
	for id := range blocks {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		if err := visit(id); err != nil {
			return err
		}
	}
	return nil
}
