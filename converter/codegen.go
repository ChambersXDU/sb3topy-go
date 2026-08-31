package converter

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// TranspileProject transpiles Scratch project.json into a Python project script.
func TranspileProject(projJsonBytes []byte, sm *SpecMap) (string, error) {
	if err := validateProjectJSON(projJsonBytes); err != nil {
		return "", err
	}

	var proj ProjectJSON
	if err := json.Unmarshal(projJsonBytes, &proj); err != nil {
		return "", fmt.Errorf("invalid project.json: %w", err)
	}

	var sb strings.Builder

	// 1. File Header
	sb.WriteString("import math\nimport time\n\n")
	sb.WriteString("import engine\n")
	sb.WriteString("from engine.events import *\n")
	sb.WriteString("from engine.operators import *\n")
	sb.WriteString("from engine.types import *\n\n\n")

	// 2. Targets
	for _, target := range proj.Targets {
		code, err := transpileTarget(&target, sm)
		if err != nil {
			return "", err
		}
		sb.WriteString(code)
		sb.WriteString("\n\n\n")
	}

	// 3. File Footer
	sb.WriteString("\n\nif __name__ == '__main__':\n")
	sb.WriteString("    engine.start_program()\n")

	return sb.String(), nil
}

func transpileTarget(target *TargetJSON, sm *SpecMap) (string, error) {
	var sb strings.Builder

	cleanClassName := "Stage"
	if !target.IsStage {
		cleanClassName = "Sprite" + CleanIdentifier(target.Name, "Sprite")
	}

	// @sprite('...')
	// class ClassName(Target):
	sb.WriteString(fmt.Sprintf("# sb3topy:target %s\n", QuoteString(target.Name)))
	sb.WriteString(fmt.Sprintf("@sprite(%s)\n", QuoteField(target.Name)))
	sb.WriteString(fmt.Sprintf("class %s(Target):\n", cleanClassName))
	sb.WriteString(fmt.Sprintf("    %s\n\n", QuoteString("Sprite "+target.Name)))

	// __init__ method
	sb.WriteString("    def __init__(self, parent=None):\n")
	sb.WriteString("        super().__init__(parent)\n")
	sb.WriteString("        if parent is not None:\n")
	sb.WriteString("            return\n\n")

	// Transform & Properties
	sb.WriteString(fmt.Sprintf("        self._xpos = %v\n", target.X))
	sb.WriteString(fmt.Sprintf("        self._ypos = %v\n", target.Y))
	sb.WriteString(fmt.Sprintf("        self._direction = %v\n", target.Direction))
	visStr := "True"
	if !target.Visible {
		visStr = "False"
	}
	sb.WriteString(fmt.Sprintf("        self.shown = %s\n", visStr))
	sb.WriteString("        self.pen = Pen(self)\n\n")

	// Costumes
	size := 100.0
	if target.Size != nil {
		size = *target.Size
	}
	rotationStyle := target.RotationStyle
	if rotationStyle == "" {
		rotationStyle = "None"
	}
	sb.WriteString("        self.costume = Costumes(\n")
	sb.WriteString(fmt.Sprintf("           %d, %v, %s, [\n", target.CurrentCostume, size, QuoteString(rotationStyle)))
	for i, c := range target.Costumes {
		sb.WriteString("            {\n")
		sb.WriteString(fmt.Sprintf("                'name': %s,\n", QuoteString(c.Name)))
		md5 := c.Md5Ext
		if md5 == "" {
			md5 = c.AssetID + "." + c.DataFormat
		}
		sb.WriteString(fmt.Sprintf("                'path': %s,\n", QuoteString(md5)))
		scale := c.BitmapResolution
		if scale <= 0 {
			scale = 1
		}
		sb.WriteString(fmt.Sprintf("                'center': (%v, %v),\n", c.CenterX, c.CenterY))
		sb.WriteString(fmt.Sprintf("                'scale': %v\n", scale))
		if i == len(target.Costumes)-1 {
			sb.WriteString("            }\n")
		} else {
			sb.WriteString("            },\n")
		}
	}
	sb.WriteString("        ])\n\n")

	// Sounds
	sb.WriteString("        self.sounds = Sounds(\n")
	vol := 100.0
	if target.Volume != nil {
		vol = *target.Volume
	}
	sb.WriteString(fmt.Sprintf("            %v, [\n", vol))
	for i, s := range target.Sounds {
		sb.WriteString("            {\n")
		sb.WriteString(fmt.Sprintf("                'name': %s,\n", QuoteString(s.Name)))
		md5 := s.Md5Ext
		if md5 == "" {
			md5 = s.AssetID + "." + s.DataFormat
		}
		sb.WriteString(fmt.Sprintf("                'path': %s\n", QuoteString(md5)))
		if i == len(target.Sounds)-1 {
			sb.WriteString("            }\n")
		} else {
			sb.WriteString("            },\n")
		}
	}
	sb.WriteString("        ])\n\n")

	// Variables
	varKeys := make([]string, 0, len(target.Variables))
	for k := range target.Variables {
		varKeys = append(varKeys, k)
	}
	sort.Strings(varKeys)

	for _, k := range varKeys {
		vData := target.Variables[k]
		varName := k
		varVal := interface{}(0)

		if slice, ok := vData.([]interface{}); ok && len(slice) >= 2 {
			varName = fmt.Sprint(slice[0])
			varVal = slice[1]
		}
		cleanVar := CleanIdentifier(varName, "var")
		sb.WriteString(fmt.Sprintf("        self.var_%s = %s\n", cleanVar, formatPyValue(varVal)))
	}
	if len(varKeys) > 0 {
		sb.WriteString("\n")
	}

	// Lists
	listKeys := make([]string, 0, len(target.Lists))
	for k := range target.Lists {
		listKeys = append(listKeys, k)
	}
	sort.Strings(listKeys)

	for _, k := range listKeys {
		lData := target.Lists[k]
		listName := k
		var listVals []interface{}

		if slice, ok := lData.([]interface{}); ok && len(slice) >= 2 {
			listName = fmt.Sprint(slice[0])
			if vals, ok := slice[1].([]interface{}); ok {
				listVals = vals
			}
		}

		cleanList := CleanIdentifier(listName, "list")
		sb.WriteString(fmt.Sprintf("        self.list_%s = List(\n", cleanList))
		sb.WriteString("            [")
		strVals := make([]string, 0, len(listVals))
		for _, val := range listVals {
			strVals = append(strVals, formatPyValue(val))
		}
		sb.WriteString(strings.Join(strVals, ", "))
		sb.WriteString("]\n        )\n")
	}
	if len(listKeys) > 0 {
		sb.WriteString("\n")
	}

	sb.WriteString(fmt.Sprintf("        self.sprite.layer = %d\n", target.LayerOrder))

	// Hat methods / Blocks
	hatCode := transpileTargetBlocks(target, sm)
	if hatCode != "" {
		sb.WriteString("\n")
		sb.WriteString(hatCode)
	}

	return sb.String(), nil
}

type markerState struct {
	emitted          map[string]bool
	pending          map[string]bool
	targetIsStage    bool
	localVariableIDs map[string]bool
	localListIDs     map[string]bool
}

func newMarkerState(target *TargetJSON) *markerState {
	state := &markerState{
		emitted:          make(map[string]bool),
		pending:          make(map[string]bool),
		targetIsStage:    target.IsStage,
		localVariableIDs: make(map[string]bool, len(target.Variables)),
		localListIDs:     make(map[string]bool, len(target.Lists)),
	}
	for id := range target.Variables {
		state.localVariableIDs[id] = true
	}
	for id := range target.Lists {
		state.localListIDs[id] = true
	}
	return state
}

func (s *markerState) dataReference(prefix, name, id string) string {
	clean := CleanIdentifier(name, prefix)
	local := s.targetIsStage
	if !local && id != "" {
		if prefix == "var" {
			local = s.localVariableIDs[id]
		} else {
			local = s.localListIDs[id]
		}
	}
	if local {
		return "self." + prefix + "_" + clean
	}
	return "util.sprites.stage." + prefix + "_" + clean
}

func (s *markerState) queue(blockID string) {
	if blockID != "" && !s.emitted[blockID] {
		s.pending[blockID] = true
	}
}

func (s *markerState) snapshotPending() map[string]bool {
	snapshot := make(map[string]bool, len(s.pending))
	for id := range s.pending {
		snapshot[id] = true
	}
	return snapshot
}

func (s *markerState) flushSince(sb *strings.Builder, blocksMap map[string]*RawBlockData, indent string, baseline map[string]bool) {
	ids := make([]string, 0, len(s.pending))
	for id := range s.pending {
		if !baseline[id] {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	for _, id := range ids {
		if s.emitted[id] {
			delete(s.pending, id)
			continue
		}
		block := blocksMap[id]
		if block == nil {
			continue
		}
		sb.WriteString(fmt.Sprintf("%s# sb3topy:block id=%s opcode=%s kind=reporter shadow=%t\n", indent, QuoteString(id), QuoteString(block.Opcode), block.Shadow))
		s.emitted[id] = true
		delete(s.pending, id)
	}
}

func emitBlockMarker(sb *strings.Builder, indent, id string, block *RawBlockData, kind string) {
	sb.WriteString(fmt.Sprintf("%s# sb3topy:block id=%s opcode=%s kind=%s shadow=%t\n", indent, QuoteString(id), QuoteString(block.Opcode), kind, block.Shadow))
}

func transpileTargetBlocks(target *TargetJSON, sm *SpecMap) string {
	if len(target.Blocks) == 0 {
		return ""
	}

	blocksMap := make(map[string]*RawBlockData)
	var hatIDs []string

	for blockID, rawObj := range target.Blocks {
		block, ok := ParseRawBlock(rawObj)
		if !ok {
			continue
		}
		blocksMap[blockID] = block
		if block.TopLevel && sm.IsHat(block.Opcode) {
			hatIDs = append(hatIDs, blockID)
		}
	}

	sort.Strings(hatIDs)

	var sb strings.Builder
	hatIndex := map[string]int{}
	markers := newMarkerState(target)

	for _, hatID := range hatIDs {
		hatBlock := blocksMap[hatID]
		opcode := hatBlock.Opcode
		sb.WriteString(fmt.Sprintf("    # sb3topy:hat id=%s opcode=%s\n", QuoteString(hatID), QuoteString(opcode)))
		markers.emitted[hatID] = true

		methodName := "green_flag"
		decorator := "@on_green_flag"

		switch opcode {
		case "event_whenflagclicked":
			methodName = "green_flag"
			decorator = "@on_green_flag"
		case "event_whenkeypressed":
			keyVal := getOptionField(hatBlock, "KEY_OPTION", "space")
			methodName = fmt.Sprintf("key_%s_pressed", CleanIdentifier(keyVal, "key"))
			decorator = fmt.Sprintf("@on_pressed(%s)", QuoteField(keyVal))
		case "event_whenbroadcastreceived":
			bcastVal := getOptionField(hatBlock, "BROADCAST_OPTION", "message1")
			methodName = fmt.Sprintf("broadcast_%s", CleanIdentifier(bcastVal, "broadcast"))
			decorator = fmt.Sprintf("@on_broadcast(%s)", QuoteField(bcastVal))
		case "event_whenthisspriteclicked", "event_whenstageclicked":
			methodName = "sprite_clicked"
			decorator = "@on_clicked"
		case "event_whenbackdropswitchesto":
			backdrop := getFieldVal(hatBlock, "BACKDROP", "")
			methodName = "on_backdrop_" + CleanIdentifier(backdrop, "backdrop")
			decorator = fmt.Sprintf("@on_backdrop(%s)", QuoteField(backdrop))
		case "event_whengreaterthan":
			source := strings.ToLower(getFieldVal(hatBlock, "WHENGREATERTHANMENU", "timer"))
			value := transpileInput(hatBlock, "VALUE", blocksMap, sm, "10", markers)
			methodName = "on_" + CleanIdentifier(source, "greater")
			decorator = fmt.Sprintf("@on_greater(%s, %s)", QuoteField(source), value)
		case "control_start_as_clone":
			methodName = "clone_start"
			decorator = "@on_clone_start"
		case "procedures_definition":
			procName := "custom_proc"
			var argNames []string
			customID := getSubstack(hatBlock, "custom_block")
			if custIDStr, ok := customID.(string); ok {
				if custBlock, ok := blocksMap[custIDStr]; ok && custBlock.Mutation != nil {
					if code, ok := custBlock.Mutation["proccode"].(string); ok {
						procName = CleanIdentifier(code, "proc")
					}
					if rawArgs, ok := custBlock.Mutation["argumentnames"]; ok {
						if argsStr, ok := rawArgs.(string); ok {
							var parsedArgs []string
							if err := json.Unmarshal([]byte(argsStr), &parsedArgs); err == nil {
								for _, a := range parsedArgs {
									argNames = append(argNames, CleanIdentifier(a, "arg"))
								}
							}
						} else if argsSlice, ok := rawArgs.([]interface{}); ok {
							for _, a := range argsSlice {
								argNames = append(argNames, CleanIdentifier(fmt.Sprint(a), "arg"))
							}
						}
					}
				}
			}
			methodName = procName
			decorator = "# custom procedure"
			paramsStr := ""
			if len(argNames) > 0 {
				paramsStr = ", " + strings.Join(argNames, ", ")
			}
			sb.WriteString(fmt.Sprintf("    async def %s(self, util%s):\n", methodName, paramsStr))
			bodyCode := transpileBlockStack(hatBlock.Next, blocksMap, sm, "        ", markers)
			if bodyCode == "" {
				bodyCode = "        await self.yield_()\n"
			}
			sb.WriteString(bodyCode)
			sb.WriteString("\n")
			continue
		}

		hatIndex[methodName]++
		if hatIndex[methodName] > 1 {
			methodName = fmt.Sprintf("%s%d", methodName, hatIndex[methodName]-1)
		}

		sb.WriteString(fmt.Sprintf("    %s\n", decorator))
		sb.WriteString(fmt.Sprintf("    async def %s(self, util):\n", methodName))

		bodyCode := transpileBlockStack(hatBlock.Next, blocksMap, sm, "        ", markers)
		if bodyCode == "" {
			bodyCode = "        await self.yield_()\n"
		}
		sb.WriteString(bodyCode)
		sb.WriteString("\n")
	}

	// Keep a deterministic marker for blocks that are not reachable from a
	// supported hat. This includes unsupported stacks and otherwise-unused
	// reporters, so an agent can still locate every valid Scratch block ID.
	remainingIDs := make([]string, 0, len(blocksMap))
	for id := range blocksMap {
		if !markers.emitted[id] {
			remainingIDs = append(remainingIDs, id)
		}
	}
	sort.Strings(remainingIDs)
	for _, id := range remainingIDs {
		emitBlockMarker(&sb, "    ", id, blocksMap[id], "unmapped")
		markers.emitted[id] = true
	}

	return sb.String()
}

func transpileBlockStack(startBlockID interface{}, blocksMap map[string]*RawBlockData, sm *SpecMap, indent string, markers *markerState) string {
	var sb strings.Builder
	currID := startBlockID
	visited := make(map[string]bool)

	for currID != nil {
		idStr, ok := currID.(string)
		if !ok || idStr == "" {
			break
		}
		if visited[idStr] {
			break
		}
		visited[idStr] = true

		block, ok := blocksMap[idStr]
		if !ok {
			break
		}

		pendingBefore := markers.snapshotPending()
		line := transpileSingleBlock(block, blocksMap, sm, indent, markers)
		markers.flushSince(&sb, blocksMap, indent, pendingBefore)
		if !markers.emitted[idStr] {
			emitBlockMarker(&sb, indent, idStr, block, "stack")
			markers.emitted[idStr] = true
		}
		if line != "" {
			sb.WriteString(line)
			if !strings.HasSuffix(line, "\n") {
				sb.WriteString("\n")
			}
		}

		currID = block.Next
	}

	return sb.String()
}

func transpileSingleBlock(block *RawBlockData, blocksMap map[string]*RawBlockData, sm *SpecMap, indent string, markers *markerState) string {
	opcode := block.Opcode

	// Shadow / Menu blocks return string literal value directly
	if block.Shadow || strings.HasSuffix(opcode, "_menu") || opcode == "sensing_keyoptions" || opcode == "note" {
		for _, fVal := range block.Fields {
			str := parseFieldValue(fVal)
			return QuoteString(str)
		}
	}

	switch opcode {
	case "control_forever":
		var body strings.Builder
		body.WriteString(indent + "while True:\n")
		subStack := getSubstack(block, "SUBSTACK")
		subCode := transpileBlockStack(subStack, blocksMap, sm, indent+"    ", markers)
		if subCode == "" {
			subCode = indent + "    await self.yield_()\n"
		}
		body.WriteString(subCode)
		body.WriteString(indent + "    await self.yield_()\n")
		return body.String()

	case "control_repeat":
		times := transpileInput(block, "TIMES", blocksMap, sm, "10", markers)
		var body strings.Builder
		body.WriteString(fmt.Sprintf("%sfor _ in range(toint(%s)):\n", indent, times))
		subStack := getSubstack(block, "SUBSTACK")
		subCode := transpileBlockStack(subStack, blocksMap, sm, indent+"    ", markers)
		if subCode == "" {
			subCode = indent + "    await self.yield_()\n"
		}
		body.WriteString(subCode)
		body.WriteString(indent + "    await self.yield_()\n")
		return body.String()

	case "control_if":
		cond := transpileInput(block, "CONDITION", blocksMap, sm, "True", markers)
		var body strings.Builder
		body.WriteString(fmt.Sprintf("%sif %s:\n", indent, cond))
		subStack := getSubstack(block, "SUBSTACK")
		subCode := transpileBlockStack(subStack, blocksMap, sm, indent+"    ", markers)
		if subCode == "" {
			subCode = indent + "    pass\n"
		}
		body.WriteString(subCode)
		return body.String()

	case "control_if_else":
		cond := transpileInput(block, "CONDITION", blocksMap, sm, "True", markers)
		var body strings.Builder
		body.WriteString(fmt.Sprintf("%sif %s:\n", indent, cond))
		subStack1 := getSubstack(block, "SUBSTACK")
		subCode1 := transpileBlockStack(subStack1, blocksMap, sm, indent+"    ", markers)
		if subCode1 == "" {
			subCode1 = indent + "    pass\n"
		}
		body.WriteString(subCode1)

		body.WriteString(indent + "else:\n")
		subStack2 := getSubstack(block, "SUBSTACK2")
		subCode2 := transpileBlockStack(subStack2, blocksMap, sm, indent+"    ", markers)
		if subCode2 == "" {
			subCode2 = indent + "    pass\n"
		}
		body.WriteString(subCode2)
		return body.String()

	case "control_repeat_until":
		cond := transpileInput(block, "CONDITION", blocksMap, sm, "False", markers)
		var body strings.Builder
		body.WriteString(fmt.Sprintf("%swhile not (%s):\n", indent, cond))
		subStack := getSubstack(block, "SUBSTACK")
		subCode := transpileBlockStack(subStack, blocksMap, sm, indent+"    ", markers)
		if subCode == "" {
			subCode = indent + "    await self.yield_()\n"
		}
		body.WriteString(subCode)
		body.WriteString(indent + "    await self.yield_()\n")
		return body.String()

	case "control_wait_until":
		cond := transpileInput(block, "CONDITION", blocksMap, sm, "True", markers)
		var body strings.Builder
		body.WriteString(fmt.Sprintf("%swhile not (%s):\n", indent, cond))
		body.WriteString(indent + "    await self.yield_()\n")
		return body.String()

	case "control_wait":
		duration := transpileInput(block, "DURATION", blocksMap, sm, "1", markers)
		return fmt.Sprintf("%sawait self.sleep(%s)", indent, duration)

	case "data_setvariableto":
		ref := dataReferenceFromField(block, "VARIABLE", "var", "variable", markers)
		val := transpileInput(block, "VALUE", blocksMap, sm, "0", markers)
		return fmt.Sprintf("%s%s = %s", indent, ref, val)

	case "data_changevariableby":
		ref := dataReferenceFromField(block, "VARIABLE", "var", "variable", markers)
		val := transpileInput(block, "VALUE", blocksMap, sm, "1", markers)
		return fmt.Sprintf("%s%s = tonum(%s) + tonum(%s)", indent, ref, ref, val)

	case "data_variable":
		return dataReferenceFromField(block, "VARIABLE", "var", "variable", markers)

	case "data_deletealloflist":
		ref := dataReferenceFromField(block, "LIST", "list", "list", markers)
		return fmt.Sprintf("%s%s.delete_all()", indent, ref)

	case "data_addtolist":
		ref := dataReferenceFromField(block, "LIST", "list", "list", markers)
		itemVal := transpileInput(block, "ITEM", blocksMap, sm, "\"\"", markers)
		return fmt.Sprintf("%s%s.append(%s)", indent, ref, itemVal)

	case "data_itemoflist":
		ref := dataReferenceFromField(block, "LIST", "list", "list", markers)
		idxVal := transpileInput(block, "INDEX", blocksMap, sm, "1", markers)
		return fmt.Sprintf("%s[toint(%s)]", ref, idxVal)

	case "data_listcontents":
		ref := dataReferenceFromField(block, "LIST", "list", "list", markers)
		return ref + ".join()"

	case "control_create_clone_of":
		opt := transpileInput(block, "CLONE_OPTION", blocksMap, sm, QuoteString("_myself_"), markers)
		return fmt.Sprintf("%sself.create_clone_of(util, %s)", indent, opt)

	case "control_delete_this_clone":
		return fmt.Sprintf("%sself.delete_clone(util)", indent)

	case "looks_switchcostumeto":
		costumeVal := transpileInput(block, "COSTUME", blocksMap, sm, "\"\"", markers)
		return fmt.Sprintf("%sself.costume.switch(%s)", indent, costumeVal)

	case "looks_changeeffectby":
		effectVal := getFieldVal(block, "EFFECT", "COLOR")
		changeVal := transpileInput(block, "CHANGE", blocksMap, sm, "25", markers)
		return fmt.Sprintf("%sself.costume.change_effect(%s, %s)", indent, QuoteString(strings.ToLower(effectVal)), changeVal)

	case "sound_play":
		soundVal := transpileInput(block, "SOUND_MENU", blocksMap, sm, "\"\"", markers)
		return fmt.Sprintf("%sself.sounds.play(%s)", indent, soundVal)

	case "motion_movesteps":
		steps := transpileInput(block, "STEPS", blocksMap, sm, "10", markers)
		return fmt.Sprintf("%sself.move(%s)", indent, steps)

	case "motion_gotoxy":
		x := transpileInput(block, "X", blocksMap, sm, "0", markers)
		y := transpileInput(block, "Y", blocksMap, sm, "0", markers)
		return fmt.Sprintf("%sself.gotoxy(%s, %s)", indent, x, y)

	case "motion_goto":
		to := transpileInput(block, "TO", blocksMap, sm, "\"_mouse_\"", markers)
		return fmt.Sprintf("%sself.goto(util, %s)", indent, to)

	case "motion_pointtowards":
		towards := transpileInput(block, "TOWARDS", blocksMap, sm, "\"_mouse_\"", markers)
		return fmt.Sprintf("%sself.point_towards(util, %s)", indent, towards)

	case "sensing_touchingobject":
		obj := transpileInput(block, "TOUCHINGOBJECTMENU", blocksMap, sm, "\"_edge_\"", markers)
		return fmt.Sprintf("self.get_touching(util, %s)", obj)

	case "sensing_keypressed":
		keyOpt := transpileInput(block, "KEY_OPTION", blocksMap, sm, "\"space\"", markers)
		return fmt.Sprintf("util.inputs[%s]", keyOpt)

	case "sensing_of":
		object := transpileInput(block, "OBJECT", blocksMap, sm, QuoteString("Stage"), markers)
		target := fmt.Sprintf("util.sprites.get_target(%s)", object)
		property := strings.ToLower(strings.TrimSpace(getFieldVal(block, "PROPERTY", "")))
		switch property {
		case "x position":
			return target + ".xpos"
		case "y position":
			return target + ".ypos"
		case "direction":
			return target + ".direction"
		case "costume #", "backdrop #":
			return target + ".costume.number"
		case "costume name", "backdrop name":
			return target + ".costume.name"
		case "size":
			return "round(" + target + ".costume.size)"
		case "volume":
			return target + ".sounds.volume"
		default:
			attribute := "var_" + CleanIdentifier(property, "variable")
			return fmt.Sprintf("getattr(%s, %s, 0)", target, QuoteString(attribute))
		}

	case "argument_reporter_string_number", "argument_reporter_boolean":
		valName := getFieldVal(block, "VALUE", "arg")
		return CleanIdentifier(valName, "arg")

	case "procedures_call":
		procName := "custom_proc"
		var argVals []string
		if block.Mutation != nil {
			if code, ok := block.Mutation["proccode"].(string); ok {
				procName = CleanIdentifier(code, "proc")
			}
			if rawIds, ok := block.Mutation["argumentids"]; ok {
				var argIDs []string
				if idsStr, ok := rawIds.(string); ok {
					json.Unmarshal([]byte(idsStr), &argIDs)
				} else if idsSlice, ok := rawIds.([]interface{}); ok {
					for _, id := range idsSlice {
						argIDs = append(argIDs, fmt.Sprint(id))
					}
				}
				for _, id := range argIDs {
					val := transpileInput(block, id, blocksMap, sm, "\"\"", markers)
					argVals = append(argVals, val)
				}
			}
		}
		argsStr := ""
		if len(argVals) > 0 {
			argsStr = ", " + strings.Join(argVals, ", ")
		}
		return fmt.Sprintf("%sawait self.%s(util%s)", indent, procName, argsStr)

	case "operator_add":
		v1 := transpileInput(block, "NUM1", blocksMap, sm, "0", markers)
		v2 := transpileInput(block, "NUM2", blocksMap, sm, "0", markers)
		return fmt.Sprintf("(tonum(%s) + tonum(%s))", v1, v2)

	case "operator_subtract":
		v1 := transpileInput(block, "NUM1", blocksMap, sm, "0", markers)
		v2 := transpileInput(block, "NUM2", blocksMap, sm, "0", markers)
		return fmt.Sprintf("(tonum(%s) - tonum(%s))", v1, v2)

	case "operator_multiply":
		v1 := transpileInput(block, "NUM1", blocksMap, sm, "0", markers)
		v2 := transpileInput(block, "NUM2", blocksMap, sm, "0", markers)
		return fmt.Sprintf("(tonum(%s) * tonum(%s))", v1, v2)

	case "operator_random":
		from := transpileInput(block, "FROM", blocksMap, sm, "1", markers)
		to := transpileInput(block, "TO", blocksMap, sm, "10", markers)
		return fmt.Sprintf("pick_rand(tonum(%s), tonum(%s))", from, to)

	case "operator_equals":
		v1 := transpileInput(block, "OPERAND1", blocksMap, sm, "\"\"", markers)
		v2 := transpileInput(block, "OPERAND2", blocksMap, sm, "\"\"", markers)
		return fmt.Sprintf("eq(%s, %s)", v1, v2)

	case "operator_gt":
		v1 := transpileInput(block, "OPERAND1", blocksMap, sm, "0", markers)
		v2 := transpileInput(block, "OPERAND2", blocksMap, sm, "0", markers)
		return fmt.Sprintf("gt(%s, %s)", v1, v2)

	case "operator_lt":
		v1 := transpileInput(block, "OPERAND1", blocksMap, sm, "0", markers)
		v2 := transpileInput(block, "OPERAND2", blocksMap, sm, "0", markers)
		return fmt.Sprintf("lt(%s, %s)", v1, v2)

	case "operator_and":
		v1 := transpileInput(block, "OPERAND1", blocksMap, sm, "False", markers)
		v2 := transpileInput(block, "OPERAND2", blocksMap, sm, "False", markers)
		return fmt.Sprintf("(%s and %s)", v1, v2)

	case "operator_or":
		v1 := transpileInput(block, "OPERAND1", blocksMap, sm, "False", markers)
		v2 := transpileInput(block, "OPERAND2", blocksMap, sm, "False", markers)
		return fmt.Sprintf("(%s or %s)", v1, v2)

	case "operator_not":
		v := transpileInput(block, "OPERAND", blocksMap, sm, "False", markers)
		return fmt.Sprintf("not %s", v)

	case "operator_join":
		v1 := transpileInput(block, "STRING1", blocksMap, sm, "\"\"", markers)
		v2 := transpileInput(block, "STRING2", blocksMap, sm, "\"\"", markers)
		return fmt.Sprintf("(str(%s) + str(%s))", v1, v2)

	case "operator_letter_of":
		text := transpileInput(block, "STRING", blocksMap, sm, "\"\"", markers)
		index := transpileInput(block, "LETTER", blocksMap, sm, "1", markers)
		return fmt.Sprintf("letter_of(str(%s), toint(%s))", text, index)

	case "operator_length":
		text := transpileInput(block, "STRING", blocksMap, sm, "\"\"", markers)
		return fmt.Sprintf("len(str(%s))", text)

	case "operator_contains":
		text := transpileInput(block, "STRING1", blocksMap, sm, "\"\"", markers)
		substring := transpileInput(block, "STRING2", blocksMap, sm, "\"\"", markers)
		return fmt.Sprintf("(str(%s).lower() in str(%s).lower())", substring, text)

	case "operator_round":
		value := transpileInput(block, "NUM", blocksMap, sm, "0", markers)
		return fmt.Sprintf("math.floor(tonum(%s) + 0.5)", value)
	}

	// Generic SpecMap lookup fallback
	resolvedOpcode, spec, ok := resolveBlockSpec(sm, opcode, block)
	if ok {
		args := make(map[string]string)
		missingArg := false
		for argName, argType := range spec.Args {
			if inputVal, ok := block.Inputs[argName]; ok {
				value := parseInputValue(inputVal, blocksMap, sm, markers)
				if strings.TrimSpace(value) == "" {
					missingArg = true
				} else {
					args[argName] = value
				}
			} else if fieldVal, ok := block.Fields[argName]; ok {
				switch argType {
				case "variable":
					args[argName] = dataReferenceFromFieldValue(fieldVal, "var", "variable", markers)
				case "list":
					args[argName] = dataReferenceFromFieldValue(fieldVal, "list", "list", markers)
				case "field":
					args[argName] = QuoteField(strings.ToLower(parseFieldValue(fieldVal)))
				case "str":
					args[argName] = QuoteString(parseFieldValue(fieldVal))
				default:
					args[argName] = parseFieldValue(fieldVal)
				}
			} else {
				missingArg = true
			}
		}
		code := ""
		if missingArg {
			return fmt.Sprintf("%spass  # sb3topy:unsupported-input opcode=%s", indent, QuoteString(opcode))
		}
		code = sm.FormatCode(resolvedOpcode, args)
		if code != "" {
			lines := strings.Split(code, "\n")
			for i, l := range lines {
				lines[i] = indent + l
			}
			return strings.Join(lines, "\n")
		}
	}

	return fmt.Sprintf("%spass  # sb3topy:unsupported opcode=%s", indent, QuoteString(opcode))
}

func resolveBlockSpec(sm *SpecMap, opcode string, block *RawBlockData) (string, *BlockSpec, bool) {
	spec, ok := sm.Get(opcode)
	if !ok {
		return opcode, nil, false
	}
	if spec.Switch == "" {
		return opcode, spec, true
	}

	resolved := spec.Switch
	for {
		start := strings.IndexByte(resolved, '{')
		if start < 0 {
			break
		}
		endOffset := strings.IndexByte(resolved[start+1:], '}')
		if endOffset < 0 {
			return opcode, nil, false
		}
		end := start + 1 + endOffset
		fieldName := resolved[start+1 : end]
		fieldValue := getFieldVal(block, fieldName, "")
		if fieldValue == "" {
			return opcode, nil, false
		}
		fieldValue = normalizeSwitchValue(fieldValue)
		resolved = resolved[:start] + fieldValue + resolved[end+1:]
	}

	resolvedSpec, ok := sm.Get(resolved)
	if !ok {
		return opcode, nil, false
	}
	return resolved, resolvedSpec, true
}

func normalizeSwitchValue(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	return strings.Join(strings.Fields(value), "_")
}

func getSubstack(block *RawBlockData, key string) interface{} {
	if block.Inputs == nil {
		return nil
	}
	val, ok := block.Inputs[key]
	if !ok {
		return nil
	}
	if slice, ok := val.([]interface{}); ok && len(slice) >= 2 {
		return slice[1]
	}
	return nil
}

func getFieldVal(block *RawBlockData, key, defaultVal string) string {
	if block.Fields == nil {
		return defaultVal
	}
	val, ok := block.Fields[key]
	if !ok {
		return defaultVal
	}
	if slice, ok := val.([]interface{}); ok && len(slice) >= 1 {
		return fmt.Sprint(slice[0])
	}
	return defaultVal
}

func fieldID(fieldVal interface{}) string {
	slice, ok := fieldVal.([]interface{})
	if !ok || len(slice) < 2 {
		return ""
	}
	id, _ := slice[1].(string)
	return id
}

func dataReferenceFromField(block *RawBlockData, key, prefix, defaultName string, markers *markerState) string {
	if block.Fields == nil {
		return markers.dataReference(prefix, defaultName, "")
	}
	fieldVal, ok := block.Fields[key]
	if !ok {
		return markers.dataReference(prefix, defaultName, "")
	}
	return dataReferenceFromFieldValue(fieldVal, prefix, defaultName, markers)
}

func dataReferenceFromFieldValue(fieldVal interface{}, prefix, defaultName string, markers *markerState) string {
	name := parseFieldValue(fieldVal)
	if name == "" {
		name = defaultName
	}
	return markers.dataReference(prefix, name, fieldID(fieldVal))
}

func getOptionField(block *RawBlockData, key, defaultVal string) string {
	if block.Inputs != nil {
		if inputVal, ok := block.Inputs[key]; ok {
			if slice, ok := inputVal.([]interface{}); ok && len(slice) >= 2 {
				if strVal, ok := slice[1].(string); ok {
					return strVal
				}
			}
		}
	}
	return getFieldVal(block, key, defaultVal)
}

func transpileInput(block *RawBlockData, key string, blocksMap map[string]*RawBlockData, sm *SpecMap, defaultVal string, markers *markerState) string {
	if block.Inputs == nil {
		return defaultVal
	}
	val, ok := block.Inputs[key]
	if !ok {
		return defaultVal
	}
	res := strings.TrimSpace(parseInputValue(val, blocksMap, sm, markers))
	if res == "" {
		return defaultVal
	}
	return res
}

func parseInputValue(inputVal interface{}, blocksMap map[string]*RawBlockData, sm *SpecMap, markers *markerState) string {
	slice, ok := inputVal.([]interface{})
	if !ok || len(slice) == 0 {
		return ""
	}

	typeCode := toInt(slice[0])
	if typeCode == 1 || typeCode == 2 || typeCode == 3 {
		if len(slice) >= 2 {
			if blockID, ok := slice[1].(string); ok {
				if subBlock, ok := blocksMap[blockID]; ok {
					markers.queue(blockID)
					expr := transpileSingleBlock(subBlock, blocksMap, sm, "", markers)
					if strings.HasPrefix(expr, "pass") || strings.HasPrefix(expr, "#") || strings.Contains(expr, "\n") {
						return ""
					}
					return expr
				}
			}
			if innerSlice, ok := slice[1].([]interface{}); ok {
				return parseInputValue(innerSlice, blocksMap, sm, markers)
			}
		}
	}

	if typeCode == 12 && len(slice) >= 2 {
		varName := fmt.Sprint(slice[1])
		varID := ""
		if len(slice) >= 3 {
			varID, _ = slice[2].(string)
		}
		return markers.dataReference("var", varName, varID)
	}

	if typeCode == 13 && len(slice) >= 2 {
		listName := fmt.Sprint(slice[1])
		listID := ""
		if len(slice) >= 3 {
			listID, _ = slice[2].(string)
		}
		return markers.dataReference("list", listName, listID)
	}

	if len(slice) >= 2 {
		switch typeCode {
		case 4, 5, 6, 7, 8:
			return formatPyNumberLiteral(slice[1])
		case 9, 10, 11:
			return QuoteString(fmt.Sprint(slice[1]))
		default:
			return formatPyValue(slice[1])
		}
	}
	return ""
}

func parseFieldValue(fieldVal interface{}) string {
	slice, ok := fieldVal.([]interface{})
	if !ok || len(slice) == 0 {
		return ""
	}
	return fmt.Sprint(slice[0])
}

func formatPyNumberLiteral(val interface{}) string {
	switch v := val.(type) {
	case string:
		num, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return "0"
		}
		return fmt.Sprint(num)
	case float64:
		return fmt.Sprint(v)
	case int:
		return fmt.Sprint(v)
	default:
		return "0"
	}
}

func formatPyValue(val interface{}) string {
	if val == nil {
		return "None"
	}
	switch v := val.(type) {
	case string:
		return QuoteString(v)
	case bool:
		if v {
			return "True"
		}
		return "False"
	case float64:
		return fmt.Sprint(v)
	case int:
		return fmt.Sprint(v)
	}
	return QuoteString(fmt.Sprint(val))
}

func toInt(val interface{}) int {
	switch v := val.(type) {
	case float64:
		return int(v)
	case int:
		return v
	case string:
		i, _ := strconv.Atoi(v)
		return i
	}
	return 0
}
