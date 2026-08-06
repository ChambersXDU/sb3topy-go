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
	var proj ProjectJSON
	if err := json.Unmarshal(projJsonBytes, &proj); err != nil {
		return "", fmt.Errorf("解析 project.json 失败: %w", err)
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
	sb.WriteString(fmt.Sprintf("@sprite(%s)\n", QuoteField(target.Name)))
	sb.WriteString(fmt.Sprintf("class %s(Target):\n", cleanClassName))
	sb.WriteString(fmt.Sprintf("    \"\"\"Sprite %s\"\"\"\n\n", target.Name))

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
	sb.WriteString("        self.costume = Costumes(\n")
	sb.WriteString("           0, 100, \"None\", [\n")
	for i, c := range target.Costumes {
		sb.WriteString("            {\n")
		sb.WriteString(fmt.Sprintf("                'name': %s,\n", QuoteString(c.Name)))
		md5 := c.Md5Ext
		if md5 == "" {
			md5 = c.AssetID + "." + c.DataFormat
		}
		sb.WriteString(fmt.Sprintf("                'path': %s,\n", QuoteString(md5)))
		cx := c.CenterX
		cy := c.CenterY
		if cx == 0 && c.RotationCenterX != 0 {
			cx = c.RotationCenterX
		}
		sb.WriteString(fmt.Sprintf("                'center': (%v, %v),\n", cx, cy))
		sb.WriteString("                'scale': 1\n")
		if i == len(target.Costumes)-1 {
			sb.WriteString("            }\n")
		} else {
			sb.WriteString("            },\n")
		}
	}
	sb.WriteString("        ])\n\n")

	// Sounds
	sb.WriteString("        self.sounds = Sounds(\n")
	vol := target.Volume
	if vol == 0 {
		vol = 100
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

	for _, hatID := range hatIDs {
		hatBlock := blocksMap[hatID]
		opcode := hatBlock.Opcode

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
			bodyCode := transpileBlockStack(hatBlock.Next, blocksMap, sm, "        ")
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

		bodyCode := transpileBlockStack(hatBlock.Next, blocksMap, sm, "        ")
		if bodyCode == "" {
			bodyCode = "        await self.yield_()\n"
		}
		sb.WriteString(bodyCode)
		sb.WriteString("\n")
	}

	return sb.String()
}

func transpileBlockStack(startBlockID interface{}, blocksMap map[string]*RawBlockData, sm *SpecMap, indent string) string {
	var sb strings.Builder
	currID := startBlockID

	for currID != nil {
		idStr, ok := currID.(string)
		if !ok || idStr == "" {
			break
		}

		block, ok := blocksMap[idStr]
		if !ok {
			break
		}

		line := transpileSingleBlock(block, blocksMap, sm, indent)
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

func transpileSingleBlock(block *RawBlockData, blocksMap map[string]*RawBlockData, sm *SpecMap, indent string) string {
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
		subCode := transpileBlockStack(subStack, blocksMap, sm, indent+"    ")
		if subCode == "" {
			subCode = indent + "    await self.yield_()\n"
		}
		body.WriteString(subCode)
		body.WriteString(indent + "    await self.yield_()\n")
		return body.String()

	case "control_repeat":
		times := transpileInput(block, "TIMES", blocksMap, sm, "10")
		var body strings.Builder
		body.WriteString(fmt.Sprintf("%sfor _ in range(toint(%s)):\n", indent, times))
		subStack := getSubstack(block, "SUBSTACK")
		subCode := transpileBlockStack(subStack, blocksMap, sm, indent+"    ")
		if subCode == "" {
			subCode = indent + "    await self.yield_()\n"
		}
		body.WriteString(subCode)
		body.WriteString(indent + "    await self.yield_()\n")
		return body.String()

	case "control_if":
		cond := transpileInput(block, "CONDITION", blocksMap, sm, "True")
		var body strings.Builder
		body.WriteString(fmt.Sprintf("%sif %s:\n", indent, cond))
		subStack := getSubstack(block, "SUBSTACK")
		subCode := transpileBlockStack(subStack, blocksMap, sm, indent+"    ")
		if subCode == "" {
			subCode = indent + "    pass\n"
		}
		body.WriteString(subCode)
		return body.String()

	case "control_if_else":
		cond := transpileInput(block, "CONDITION", blocksMap, sm, "True")
		var body strings.Builder
		body.WriteString(fmt.Sprintf("%sif %s:\n", indent, cond))
		subStack1 := getSubstack(block, "SUBSTACK")
		subCode1 := transpileBlockStack(subStack1, blocksMap, sm, indent+"    ")
		if subCode1 == "" {
			subCode1 = indent + "    pass\n"
		}
		body.WriteString(subCode1)

		body.WriteString(indent + "else:\n")
		subStack2 := getSubstack(block, "SUBSTACK2")
		subCode2 := transpileBlockStack(subStack2, blocksMap, sm, indent+"    ")
		if subCode2 == "" {
			subCode2 = indent + "    pass\n"
		}
		body.WriteString(subCode2)
		return body.String()

	case "control_repeat_until":
		cond := transpileInput(block, "CONDITION", blocksMap, sm, "False")
		var body strings.Builder
		body.WriteString(fmt.Sprintf("%swhile not (%s):\n", indent, cond))
		subStack := getSubstack(block, "SUBSTACK")
		subCode := transpileBlockStack(subStack, blocksMap, sm, indent+"    ")
		if subCode == "" {
			subCode = indent + "    await self.yield_()\n"
		}
		body.WriteString(subCode)
		body.WriteString(indent + "    await self.yield_()\n")
		return body.String()

	case "control_wait_until":
		cond := transpileInput(block, "CONDITION", blocksMap, sm, "True")
		var body strings.Builder
		body.WriteString(fmt.Sprintf("%swhile not (%s):\n", indent, cond))
		body.WriteString(indent + "    await self.yield_()\n")
		return body.String()

	case "control_wait":
		duration := transpileInput(block, "DURATION", blocksMap, sm, "1")
		return fmt.Sprintf("%sawait self.sleep(%s)", indent, duration)

	case "data_setvariableto":
		varName := getFieldVal(block, "VARIABLE", "variable")
		cleanVar := CleanIdentifier(varName, "var")
		val := transpileInput(block, "VALUE", blocksMap, sm, "0")
		return fmt.Sprintf("%sutil.sprites.stage.var_%s = %s", indent, cleanVar, val)

	case "data_changevariableby":
		varName := getFieldVal(block, "VARIABLE", "variable")
		cleanVar := CleanIdentifier(varName, "var")
		val := transpileInput(block, "VALUE", blocksMap, sm, "1")
		return fmt.Sprintf("%sutil.sprites.stage.var_%s += %s", indent, cleanVar, val)

	case "data_variable":
		varName := getFieldVal(block, "VARIABLE", "variable")
		cleanVar := CleanIdentifier(varName, "var")
		return fmt.Sprintf("util.sprites.stage.var_%s", cleanVar)

	case "data_deletealloflist":
		listName := getFieldVal(block, "LIST", "list")
		cleanList := CleanIdentifier(listName, "list")
		return fmt.Sprintf("%sutil.sprites.stage.list_%s.delete_all()", indent, cleanList)

	case "data_addtolist":
		listName := getFieldVal(block, "LIST", "list")
		cleanList := CleanIdentifier(listName, "list")
		itemVal := transpileInput(block, "ITEM", blocksMap, sm, "\"\"")
		return fmt.Sprintf("%sutil.sprites.stage.list_%s.append(%s)", indent, cleanList, itemVal)

	case "data_itemoflist":
		listName := getFieldVal(block, "LIST", "list")
		cleanList := CleanIdentifier(listName, "list")
		idxVal := transpileInput(block, "INDEX", blocksMap, sm, "1")
		return fmt.Sprintf("util.sprites.stage.list_%s[toint(%s)]", cleanList, idxVal)

	case "data_listcontents":
		listName := getFieldVal(block, "LIST", "list")
		cleanList := CleanIdentifier(listName, "list")
		return fmt.Sprintf("util.sprites.stage.list_%s", cleanList)

	case "control_create_clone_of":
		opt := getOptionField(block, "CLONE_OPTION", "_myself_")
		if opt == "_myself_" {
			return fmt.Sprintf("%sself.create_clone_of(util, \"_myself_\")", indent)
		}
		return fmt.Sprintf("%sself.create_clone_of(util, %s)", indent, QuoteString(opt))

	case "control_delete_this_clone":
		return fmt.Sprintf("%sself.delete_clone(util)", indent)

	case "looks_switchcostumeto":
		costumeVal := transpileInput(block, "COSTUME", blocksMap, sm, "\"\"")
		return fmt.Sprintf("%sself.costume.switch(%s)", indent, costumeVal)

	case "looks_changeeffectby":
		effectVal := getFieldVal(block, "EFFECT", "COLOR")
		changeVal := transpileInput(block, "CHANGE", blocksMap, sm, "25")
		return fmt.Sprintf("%sself.costume.change_effect(%s, %s)", indent, QuoteString(strings.ToLower(effectVal)), changeVal)

	case "sound_play":
		soundVal := transpileInput(block, "SOUND_MENU", blocksMap, sm, "\"\"")
		return fmt.Sprintf("%sself.sounds.play(%s)", indent, soundVal)

	case "motion_movesteps":
		steps := transpileInput(block, "STEPS", blocksMap, sm, "10")
		return fmt.Sprintf("%sself.move(%s)", indent, steps)

	case "motion_gotoxy":
		x := transpileInput(block, "X", blocksMap, sm, "0")
		y := transpileInput(block, "Y", blocksMap, sm, "0")
		return fmt.Sprintf("%sself.gotoxy(%s, %s)", indent, x, y)

	case "motion_goto":
		to := transpileInput(block, "TO", blocksMap, sm, "\"_mouse_\"")
		return fmt.Sprintf("%sself.goto(util, %s)", indent, to)

	case "motion_pointtowards":
		towards := transpileInput(block, "TOWARDS", blocksMap, sm, "\"_mouse_\"")
		return fmt.Sprintf("%sself.point_towards(util, %s)", indent, towards)

	case "sensing_touchingobject":
		obj := transpileInput(block, "TOUCHINGOBJECTMENU", blocksMap, sm, "\"_edge_\"")
		return fmt.Sprintf("self.get_touching(util, %s)", obj)

	case "sensing_keypressed":
		keyOpt := transpileInput(block, "KEY_OPTION", blocksMap, sm, "\"space\"")
		keyOpt = strings.Trim(keyOpt, "\"")
		return fmt.Sprintf("util.inputs[%s]", QuoteString(keyOpt))

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
					val := transpileInput(block, id, blocksMap, sm, "\"\"")
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
		v1 := transpileInput(block, "NUM1", blocksMap, sm, "0")
		v2 := transpileInput(block, "NUM2", blocksMap, sm, "0")
		return fmt.Sprintf("(tonum(%s) + tonum(%s))", v1, v2)

	case "operator_subtract":
		v1 := transpileInput(block, "NUM1", blocksMap, sm, "0")
		v2 := transpileInput(block, "NUM2", blocksMap, sm, "0")
		return fmt.Sprintf("(tonum(%s) - tonum(%s))", v1, v2)

	case "operator_equals":
		v1 := transpileInput(block, "OPERAND1", blocksMap, sm, "\"\"")
		v2 := transpileInput(block, "OPERAND2", blocksMap, sm, "\"\"")
		return fmt.Sprintf("eq(%s, %s)", v1, v2)

	case "operator_gt":
		v1 := transpileInput(block, "OPERAND1", blocksMap, sm, "0")
		v2 := transpileInput(block, "OPERAND2", blocksMap, sm, "0")
		return fmt.Sprintf("gt(%s, %s)", v1, v2)

	case "operator_lt":
		v1 := transpileInput(block, "OPERAND1", blocksMap, sm, "0")
		v2 := transpileInput(block, "OPERAND2", blocksMap, sm, "0")
		return fmt.Sprintf("lt(%s, %s)", v1, v2)

	case "operator_and":
		v1 := transpileInput(block, "OPERAND1", blocksMap, sm, "False")
		v2 := transpileInput(block, "OPERAND2", blocksMap, sm, "False")
		return fmt.Sprintf("(%s and %s)", v1, v2)

	case "operator_or":
		v1 := transpileInput(block, "OPERAND1", blocksMap, sm, "False")
		v2 := transpileInput(block, "OPERAND2", blocksMap, sm, "False")
		return fmt.Sprintf("(%s or %s)", v1, v2)

	case "operator_not":
		v := transpileInput(block, "OPERAND", blocksMap, sm, "False")
		return fmt.Sprintf("not %s", v)

	case "operator_join":
		v1 := transpileInput(block, "STRING1", blocksMap, sm, "\"\"")
		v2 := transpileInput(block, "STRING2", blocksMap, sm, "\"\"")
		return fmt.Sprintf("(str(%s) + str(%s))", v1, v2)
	}

	// Generic SpecMap lookup fallback
	if spec, ok := sm.Get(opcode); ok {
		args := make(map[string]string)
		for argName := range spec.Args {
			if inputVal, ok := block.Inputs[argName]; ok {
				args[argName] = parseInputValue(inputVal, blocksMap, sm)
			} else if fieldVal, ok := block.Fields[argName]; ok {
				args[argName] = parseFieldValue(fieldVal)
			}
		}
		code := sm.FormatCode(opcode, args)
		if code != "" {
			lines := strings.Split(code, "\n")
			for i, l := range lines {
				lines[i] = indent + l
			}
			return strings.Join(lines, "\n")
		}
	}

	return fmt.Sprintf("%spass", indent)
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

func transpileInput(block *RawBlockData, key string, blocksMap map[string]*RawBlockData, sm *SpecMap, defaultVal string) string {
	if block.Inputs == nil {
		return defaultVal
	}
	val, ok := block.Inputs[key]
	if !ok {
		return defaultVal
	}
	res := strings.TrimSpace(parseInputValue(val, blocksMap, sm))
	if res == "" {
		return defaultVal
	}
	return res
}

func parseInputValue(inputVal interface{}, blocksMap map[string]*RawBlockData, sm *SpecMap) string {
	slice, ok := inputVal.([]interface{})
	if !ok || len(slice) == 0 {
		return ""
	}

	typeCode := toInt(slice[0])
	if typeCode == 1 || typeCode == 2 || typeCode == 3 {
		if len(slice) >= 2 {
			if blockID, ok := slice[1].(string); ok {
				if subBlock, ok := blocksMap[blockID]; ok {
					expr := transpileSingleBlock(subBlock, blocksMap, sm, "")
					if expr == "pass" || strings.HasPrefix(expr, "#") || strings.Contains(expr, "\n") {
						return ""
					}
					return expr
				}
			}
			if innerSlice, ok := slice[1].([]interface{}); ok {
				return parseInputValue(innerSlice, blocksMap, sm)
			}
		}
	}

	if typeCode == 12 && len(slice) >= 2 {
		varName := fmt.Sprint(slice[1])
		return "util.sprites.stage.var_" + CleanIdentifier(varName, "var")
	}

	if typeCode == 13 && len(slice) >= 2 {
		listName := fmt.Sprint(slice[1])
		return "util.sprites.stage.list_" + CleanIdentifier(listName, "list")
	}

	if len(slice) >= 2 {
		return formatPyValue(slice[1])
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

func formatPyValue(val interface{}) string {
	if val == nil {
		return "None"
	}
	switch v := val.(type) {
	case string:
		if num, err := strconv.ParseFloat(v, 64); err == nil {
			return fmt.Sprint(num)
		}
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
