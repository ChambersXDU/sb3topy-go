import asyncio
import logging
import math
import os
from pathlib import Path
import runpy
import sys
import time


workspace, scenario = Path(sys.argv[1]).resolve(), sys.argv[2]
os.chdir(workspace)
sys.path.insert(0, str(workspace))
runpy.run_path("project.py", run_name="sb3topy_behavior_test")
from engine.events import SPRITES
from engine.runtime import Runtime
import pygame

errors = []


class Capture(logging.Handler):
    def emit(self, record):
        if record.levelno >= logging.ERROR:
            errors.append(self.format(record))


handler = Capture()
logging.getLogger().addHandler(handler)
runtime = Runtime(SPRITES)


async def tick():
    await runtime.step_threads()
    runtime.draw()
    await asyncio.sleep(1 / 240)
    assert not errors, "\n".join(errors)


async def until(condition, seconds=3):
    deadline = time.monotonic() + seconds
    while not condition():
        assert time.monotonic() < deadline, "Expected project state was not reached"
        await tick()


async def run():
    asyncio.get_running_loop().set_exception_handler(lambda loop, context: errors.append(str(context)))
    runtime.running = True
    stage = runtime.sprites.stage
    started = time.monotonic()
    runtime.events.send(runtime.util, runtime.sprites, "green_flag")
    if scenario == "operators":
        await until(lambda: stage.var_count == 3)
        await tick()
        assert stage.var_letter0 == "" and stage.var_letter1 == "" and stage.var_letter2 == "A"
    elif scenario == "types":
        await until(lambda: stage.var_phase == "1")
        assert stage.var_joined == "truex" and stage.var_length == 3
        assert stage.var_contains is True and stage.var_equalsText is True
        assert stage.var_booleanNot is True
        assert stage.var_booleanAnd is False and stage.var_booleanOr is False
        assert stage.var_count == 2, "Scratch-false, empty, or procedure conditions executed the wrong branch"
    elif scenario == "bounce":
        await until(lambda: stage.var_count == 6)
        for name, direction in (("Left", 90), ("Right", -90), ("Top", 180), ("Bottom", 0),
                                ("Middle", 45), ("Grazing", -math.degrees(math.atan(0.2)))):
            actor = runtime.sprites.targets[name]
            assert abs(((actor.direction - direction + 180) % 360) - 180) < 1e-6, "Incorrect reflection at " + name
            actor.update(runtime.display)
            bounds = actor.sprite.image.get_bounding_rect().move(actor.sprite.rect.topleft)
            assert runtime.display.rect.contains(bounds), "Sprite was not fenced after bouncing at " + name
        middle = runtime.sprites.targets["Middle"]
        assert middle.xpos == 0 and middle.ypos == 0, "A sprite away from the edges moved"
    elif scenario == "timer":
        await until(lambda: stage.var_count == 1)
        deadline = time.monotonic() + 0.06
        while time.monotonic() < deadline:
            await tick()
        assert stage.var_count == 1, "Timer hat repeatedly fired while the condition stayed true"
        runtime.util.timer.reset()
        await tick()
        await until(lambda: stage.var_count == 2)
        stage.var_threshold = "100"
        await tick()
        stage.var_threshold = "0"
        await until(lambda: stage.var_count == 3)
    elif scenario == "timing":
        await until(lambda: stage.var_phase == "1")
        first = time.monotonic()
        assert first - started >= 0.19, "Speech duration was skipped"
        await until(lambda: stage.var_phase == "2")
        assert time.monotonic() - first >= 0.18, "Thought duration was skipped"
    elif scenario == "motion":
        actor = runtime.sprites.targets["Actor"]
        await until(lambda: stage.var_clones == 1)
        assert actor.var_xBeforeGlide == 3.5 and actor.var_yBeforeGlide == 1.5
        assert actor.direction == 90 and actor.xpos == 30 and actor.ypos == 40
        assert len(actor.clones) == 1
        assert actor.clones[0].xpos == 35 and actor.clones[0].ypos == 40
    elif scenario == "score":
        await tick()
        for _ in range(2):
            await runtime.events.send_wait(runtime.util, runtime.sprites, "key_space_pressed", True)
            await tick()
        assert stage.var_score == 1, "One contact was counted more than once"
    elif scenario == "unsupported":
        await until(lambda: stage.var_phase == "1")
        actor = runtime.sprites.targets["Actor"]
        assert actor.xpos == 0, "An unsupported reporter was replaced by a movement default"
        assert actor.var_value == "unchanged", "An unsupported reporter overwrote a variable"
    elif scenario == "cross_clone":
        await until(lambda: stage.var_phase == "1")
        creator = runtime.sprites.targets["Creator"]
        original = runtime.sprites.targets["Original"]
        assert len(creator.clones) == 0 and len(original.clones) == 1, "Clone registered under the creator"
        assert original.xpos == 150 and original.clones[0].xpos == 30
        group = runtime.sprites.group
        assert group.get_layer_of_sprite(original.clones[0].sprite) < group.get_layer_of_sprite(original.sprite) < group.get_layer_of_sprite(creator.sprite), "Clone was placed behind the creator instead of its original"
        assert creator.var_touchesClone is True, "Touching a sprite did not include its clones"
        stage.var_delete = 1
        await until(lambda: not original.clones)
        assert not original._clones, "Deleted clone remained in the global pool"
    elif scenario == "reset":
        await until(lambda: stage.var_health == "3")
        await runtime.events.send_wait(runtime.util, runtime.sprites, "key_space_pressed", True)
        assert stage.var_health == 2
        runtime.events.send(runtime.util, runtime.sprites, "green_flag", True)
        await until(lambda: stage.var_health == "3")
    else:
        raise ValueError("Unknown behavior scenario: " + scenario)
    await tick()


try:
    asyncio.run(run())
    print("Behavior assertions passed: " + scenario)
finally:
    runtime.quit()
    logging.getLogger().removeHandler(handler)
    pygame.quit()
