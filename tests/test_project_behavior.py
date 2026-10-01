import importlib.util
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import unittest
from zipfile import ZipFile

from project_fixtures import write_fixture


ROOT = Path(__file__).resolve().parents[1]


@unittest.skipUnless(importlib.util.find_spec("pygame") and shutil.which("go"), "Behavior tests require Go and pygame")
class ProjectBehaviorTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.directory = tempfile.TemporaryDirectory(prefix="sb3topy-behavior-")
        cls.addClassCleanup(cls.directory.cleanup)
        cls.binary = str(Path(cls.directory.name) / "sb3topy")
        subprocess.run(["go", "build", "-o", cls.binary], cwd=ROOT, check=True, capture_output=True)

    def command(self, *arguments):
        result = subprocess.run([self.binary, *map(str, arguments)], capture_output=True, text=True, timeout=30)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        return result

    def check_scenario(self, scenario):
        with tempfile.TemporaryDirectory(prefix="sb3topy-project-") as directory:
            root = Path(directory)
            source, workspace, output = root / "input.sb3", root / "workspace", root / "output.sb3"
            write_fixture(source, scenario)
            self.command("to-python", source, workspace)
            self.command("verify", workspace)
            self.command("to-sb3", workspace, output)
            with ZipFile(source) as before, ZipFile(output) as after:
                self.assertEqual({n: before.read(n) for n in before.namelist()}, {n: after.read(n) for n in after.namelist()})
            environment = dict(os.environ, PYGAME_HIDE_SUPPORT_PROMPT="1", PYTHONDONTWRITEBYTECODE="1",
                               SDL_VIDEODRIVER="dummy", SDL_AUDIODRIVER="dummy")
            result = subprocess.run([sys.executable, "-B", str(ROOT / "tests" / "behavior_driver.py"), str(workspace), scenario],
                                    env=environment, capture_output=True, text=True, timeout=15)
            self.assertEqual(result.returncode, 0, result.stdout + result.stderr)

    def test_repeat_and_character_results(self):
        self.check_scenario("operators")

    def test_boolean_and_text_reporters(self):
        self.check_scenario("types")

    def test_timed_speech_and_thought_preserve_order(self):
        self.check_scenario("timing")

    def test_timer_threshold_uses_variables_and_rising_edges(self):
        self.check_scenario("timer")

    def test_edge_bounce_reflects_and_fences_each_edge(self):
        self.check_scenario("bounce")

    def test_numeric_motion_procedure_and_clone_positions(self):
        self.check_scenario("motion")

    def test_contact_counts_once(self):
        self.check_scenario("score")

    def test_green_flag_resets_health(self):
        self.check_scenario("reset")

    def test_unsupported_reporters_do_not_invent_values(self):
        self.check_scenario("unsupported")

    def test_cross_sprite_clones_are_detected_and_deleted(self):
        self.check_scenario("cross_clone")


if __name__ == "__main__":
    unittest.main()
