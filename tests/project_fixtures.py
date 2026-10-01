import json
from zipfile import ZipFile


def text(value):
    return [1, [10, str(value)]]


def variable(name):
    return [3, [12, name, name], [10, ""]]


class Script:
    def __init__(self, target, opcode="event_whenflagclicked", fields=None):
        self.target = target
        self.blocks = target["blocks"]
        self.previous = self.add(opcode, fields=fields, top_level=True)

    def add(self, opcode, inputs=None, fields=None, parent=None, top_level=False, mutation=None, shadow=False):
        block_id = "b" + str(len(self.blocks))
        self.blocks[block_id] = {
            "opcode": opcode, "next": None, "parent": parent,
            "inputs": inputs or {}, "fields": fields or {},
            "topLevel": top_level, "shadow": shadow,
        }
        if mutation is not None:
            self.blocks[block_id]["mutation"] = mutation
        return block_id

    def stack(self, opcode, inputs=None, fields=None):
        block_id = self.add(opcode, inputs, fields, parent=self.previous)
        self.blocks[self.previous]["next"] = block_id
        self.previous = block_id
        return block_id

    def set(self, name, value):
        return self.stack("data_setvariableto", {"VALUE": value}, {"VARIABLE": [name, name]})

    def reporter(self, parent, input_name, opcode, inputs=None, fields=None):
        block_id = self.add(opcode, inputs, fields, parent=parent)
        self.blocks[parent]["inputs"][input_name] = [2, block_id]
        return block_id


def target(name, stage=False, variables=None, x=0):
    return {
        "name": name, "isStage": stage, "variables": {key: [key, value] for key, value in (variables or {}).items()},
        "lists": {}, "broadcasts": {}, "blocks": {}, "x": x, "y": 0, "direction": 90,
        "size": 100, "visible": True, "volume": 100, "layerOrder": 0 if stage else 1,
        "currentCostume": 0, "rotationStyle": "all around", "sounds": [],
        "costumes": [{"name": "square", "assetId": "square", "md5ext": "square.svg", "dataFormat": "svg",
                      "bitmapResolution": 1, "rotationCenterX": 5, "rotationCenterY": 5}],
    }


def fixture(scenario):
    stage = target("Stage", True, {"score": 0, "health": 3, "phase": 0, "count": 0,
                                   "letter0": "", "letter1": "", "letter2": "", "clones": 0, "delete": 0})
    targets = [stage]
    script = Script(stage)
    if scenario == "operators":
        repeat = script.stack("control_repeat", {"TIMES": text("2.5")})
        body = script.add("data_changevariableby", {"VALUE": text("1")}, {"VARIABLE": ["count", "count"]}, parent=repeat)
        script.blocks[repeat]["inputs"]["SUBSTACK"] = [2, body]
        for name, index in (("letter0", "0"), ("letter1", "-1"), ("letter2", "1.9")):
            parent = script.set(name, text(""))
            script.reporter(parent, "VALUE", "operator_letter_of", {"STRING": text("ABC"), "LETTER": text(index)})
    elif scenario == "types":
        values = {"falseText": "false", "zeroText": "0", "trueValue": True,
                  "joined": "", "length": 0, "contains": False, "booleanNot": False,
                  "booleanAnd": True, "booleanOr": True, "equalsText": False}
        stage["variables"].update({key: [key, value] for key, value in values.items()})
        for name, opcode, inputs in (
            ("joined", "operator_join", {"STRING1": variable("trueValue"), "STRING2": text("x")}),
            ("length", "operator_length", {"STRING": text("🙂A")}),
            ("contains", "operator_contains", {"STRING1": variable("trueValue"), "STRING2": text("TRU")}),
            ("booleanNot", "operator_not", {"OPERAND": variable("falseText")}),
            ("booleanAnd", "operator_and", {"OPERAND1": variable("falseText"), "OPERAND2": variable("trueValue")}),
            ("booleanOr", "operator_or", {"OPERAND1": variable("zeroText"), "OPERAND2": text("false")}),
            ("equalsText", "operator_equals", {"OPERAND1": variable("trueValue"), "OPERAND2": text("TRUE")}),
        ):
            parent = script.set(name, text(""))
            script.reporter(parent, "VALUE", opcode, inputs)
        for value in (variable("falseText"), variable("zeroText"), variable("trueValue"), None):
            condition = script.stack("control_if", {"CONDITION": value} if value is not None else {})
            body = script.add("data_changevariableby", {"VALUE": text("1")}, {"VARIABLE": ["count", "count"]}, parent=condition)
            script.blocks[condition]["inputs"]["SUBSTACK"] = [2, body]
        for value in (variable("falseText"), variable("trueValue")):
            call = script.stack("procedures_call", {"flag": value})
            script.blocks[call]["mutation"] = {"proccode": "check %b", "argumentids": '["flag"]'}
        procedure = Script(stage, "procedures_definition")
        definition = procedure.previous
        prototype = procedure.add("procedures_prototype", parent=definition, shadow=True, mutation={
            "proccode": "check %b", "argumentids": '["flag"]', "argumentnames": '["flag"]',
            "argumentdefaults": '[false]', "warp": "false"})
        procedure.blocks[definition]["inputs"]["custom_block"] = [1, prototype]
        argument = procedure.add("argument_reporter_boolean", fields={"VALUE": ["flag", None]}, parent=prototype, shadow=True)
        procedure.blocks[prototype]["inputs"]["flag"] = [1, argument]
        condition = procedure.stack("control_if")
        procedure.reporter(condition, "CONDITION", "argument_reporter_boolean", fields={"VALUE": ["flag", None]})
        body = procedure.add("data_changevariableby", {"VALUE": text("1")}, {"VARIABLE": ["count", "count"]}, parent=condition)
        procedure.blocks[condition]["inputs"]["SUBSTACK"] = [2, body]
        script.set("phase", text("1"))
    elif scenario == "bounce":
        for name, x, y, direction in (("Left", -238, 0, -90), ("Right", 238, 0, 90),
                                       ("Top", 0, 178, 0), ("Bottom", 0, -178, 180),
                                       ("Middle", 0, 0, 45), ("Grazing", 238, 0, 0)):
            actor = target(name, x=x)
            actor["y"], actor["direction"] = y, direction
            targets.append(actor)
            bounce = Script(actor)
            bounce.stack("motion_ifonedgebounce")
            bounce.stack("data_changevariableby", {"VALUE": text("1")}, {"VARIABLE": ["count", "count"]})
    elif scenario == "timer":
        stage["variables"]["threshold"] = ["threshold", "0.02"]
        threshold = Script(stage, "event_whengreaterthan", {"WHENGREATERTHANMENU": ["TIMER", None]})
        threshold.blocks[threshold.previous]["inputs"]["VALUE"] = variable("threshold")
        threshold.stack("data_changevariableby", {"VALUE": text("1")}, {"VARIABLE": ["count", "count"]})
    elif scenario == "timing":
        script.stack("looks_sayforsecs", {"MESSAGE": text("hello"), "SECS": text("0.2")})
        script.set("phase", text("1"))
        script.stack("looks_thinkforsecs", {"MESSAGE": text("hmm"), "SECS": text("0.2")})
        script.set("phase", text("2"))
    elif scenario == "motion":
        actor = target("Actor", variables={"duration": "0.02", "xBeforeGlide": 0, "yBeforeGlide": 0})
        targets.append(actor)
        motion = Script(actor)
        motion.stack("motion_pointindirection", {"DIRECTION": text("90")})
        motion.stack("motion_turnright", {"DEGREES": text("45")})
        motion.stack("motion_turnleft", {"DEGREES": text("45")})
        motion.stack("motion_gotoxy", {"X": text("1.5"), "Y": text("2.5")})
        motion.stack("motion_changexby", {"DX": text("2")})
        motion.stack("motion_changeyby", {"DY": text("-1")})
        for name, opcode in (("xBeforeGlide", "motion_xposition"), ("yBeforeGlide", "motion_yposition")):
            parent = motion.set(name, text(""))
            motion.reporter(parent, "VALUE", opcode)
        motion.stack("motion_setx", {"X": text("10")})
        motion.stack("motion_sety", {"Y": text("20")})
        motion.stack("motion_glideto", {"SECS": variable("duration"), "TO": text("_mouse_")})
        call = motion.stack("procedures_call", {"duration": text("0.02")})
        motion.blocks[call]["mutation"] = {"proccode": "travel %s", "argumentids": '["duration"]'}
        motion.stack("control_create_clone_of", {"CLONE_OPTION": text("_myself_")})
        procedure = Script(actor, "procedures_definition")
        definition = procedure.previous
        prototype = procedure.add("procedures_prototype", parent=definition, shadow=True, mutation={
            "proccode": "travel %s", "argumentids": '["duration"]', "argumentnames": '["duration"]',
            "argumentdefaults": '[""]', "warp": "false"})
        procedure.blocks[definition]["inputs"]["custom_block"] = [1, prototype]
        argument = procedure.add("argument_reporter_string_number", fields={"VALUE": ["duration", None]}, parent=prototype, shadow=True)
        procedure.blocks[prototype]["inputs"]["duration"] = [1, argument]
        glide = procedure.stack("motion_glidesecstoxy", {"X": text("30"), "Y": text("40")})
        procedure.reporter(glide, "SECS", "argument_reporter_string_number", fields={"VALUE": ["duration", None]})
        clone = Script(actor, "control_start_as_clone")
        clone.stack("motion_changexby", {"DX": text("5")})
        clone.stack("data_changevariableby", {"VALUE": text("1")}, {"VARIABLE": ["clones", "clones"]})
    elif scenario == "score":
        actor, apple = target("Actor"), target("Apple")
        targets.extend([actor, apple])
        touch = Script(actor, "event_whenkeypressed", {"KEY_OPTION": ["space", None]})
        condition = touch.stack("control_if")
        touch.reporter(condition, "CONDITION", "sensing_touchingobject", {"TOUCHINGOBJECTMENU": text("Apple")})
        once = touch.add("control_if", parent=condition)
        touch.blocks[condition]["inputs"]["SUBSTACK"] = [2, once]
        touch.reporter(once, "CONDITION", "operator_equals", {"OPERAND1": variable("score"), "OPERAND2": text("0")})
        increment = touch.add("data_changevariableby", {"VALUE": text("1")}, {"VARIABLE": ["score", "score"]}, parent=once)
        touch.blocks[once]["inputs"]["SUBSTACK"] = [2, increment]
    elif scenario == "unsupported":
        actor = target("Actor", variables={"value": "unchanged"})
        targets.append(actor)
        unsupported = Script(actor)
        move = unsupported.stack("motion_movesteps")
        add = unsupported.reporter(move, "STEPS", "operator_add", {"NUM2": text("5")})
        unsupported.reporter(add, "NUM1", "extension_reporter_unknown")
        assignment = unsupported.set("value", text(""))
        unsupported.reporter(assignment, "VALUE", "extension_reporter_unknown")
        unsupported.set("phase", text("1"))
    elif scenario == "cross_clone":
        creator = target("Creator", variables={"touchesClone": False}, x=-100)
        original = target("Original", x=30)
        creator["layerOrder"] = 6
        targets.extend([creator, original])
        creation = Script(creator)
        creation.stack("control_create_clone_of", {"CLONE_OPTION": text("Original")})
        creation.stack("control_wait", {"DURATION": text("0.03")})
        creation.stack("motion_gotoxy", {"X": [1, [4, "30"]], "Y": [1, [4, "0"]]})
        contact = creation.set("touchesClone", text(""))
        creation.reporter(contact, "VALUE", "sensing_touchingobject", {"TOUCHINGOBJECTMENU": text("Original")})
        creation.set("phase", text("1"))
        move_original = Script(original)
        ready = move_original.stack("control_wait_until")
        move_original.reporter(ready, "CONDITION", "operator_equals", {"OPERAND1": variable("clones"), "OPERAND2": text("1")})
        move_original.stack("motion_setx", {"X": [1, [4, "150"]]})
        clone = Script(original, "control_start_as_clone")
        clone.stack("data_changevariableby", {"VALUE": text("1")}, {"VARIABLE": ["clones", "clones"]})
        delete = clone.stack("control_wait_until")
        clone.reporter(delete, "CONDITION", "operator_equals", {"OPERAND1": variable("delete"), "OPERAND2": text("1")})
        clone.stack("control_delete_this_clone")
    elif scenario == "reset":
        script.set("health", text("3"))
        damage = Script(stage, "event_whenkeypressed", {"KEY_OPTION": ["space", None]})
        damage.stack("data_changevariableby", {"VALUE": text("-1")}, {"VARIABLE": ["health", "health"]})
    else:
        raise ValueError("Unknown fixture scenario: " + scenario)
    return {"targets": targets, "extensions": [], "monitors": [], "meta": {"semver": "3.0.0"}}


def write_fixture(path, scenario):
    project = fixture(scenario)
    with ZipFile(path, "w") as archive:
        archive.writestr("project.json", json.dumps(project, ensure_ascii=False))
        archive.writestr("square.svg", '<svg xmlns="http://www.w3.org/2000/svg" width="10" height="10"><rect width="10" height="10" fill="red"/></svg>')
