import importlib.util
import math
from pathlib import Path
import unittest


spec = importlib.util.spec_from_file_location(
    "sb3topy_operators", Path(__file__).resolve().parents[1] / "engine" / "operators.py")
operators = importlib.util.module_from_spec(spec)
spec.loader.exec_module(operators)


class OperatorTests(unittest.TestCase):
    def test_boolean_cast_matches_scratch(self):
        for value, expected in (("", False), ("0", False), ("false", False), ("FALSE", False),
                                (" false ", True), ("00", True), (" ", True), ("true", True),
                                (0, False), (1, True), (False, False), (True, True), (math.nan, False)):
            with self.subTest(value=value):
                self.assertIs(operators.tobool(value), expected)

    def test_text_cast_and_length_share_character_rules(self):
        for value, expected in ((True, "true"), (False, "false"), (1.0, "1"), (-0.0, "0"),
                                (math.nan, "NaN"), (math.inf, "Infinity"), (-math.inf, "-Infinity"),
                                ("🙂A", "🙂A")):
            with self.subTest(value=value):
                self.assertEqual(operators.tostr(value), expected)
                self.assertEqual(operators.string_length(value), len(expected.encode('utf-16-le')) // 2)
        self.assertTrue(operators.eq(True, "TRUE"))
        self.assertTrue(operators.lt(False, "true"))

    def test_repeat_counts_round_ties_towards_positive_infinity(self):
        for value, expected in ((2.5, 3), (1.5, 2), (0.5, 1), (-1.5, -1),
                                ("2.5", 3), (" 2.4 ", 2), ("", 0), ("bad", 0),
                                (math.nan, 0), (math.inf, 0)):
            with self.subTest(value=value):
                self.assertEqual(operators.toint(value), expected)

    def test_letter_indices_check_bounds_before_truncating(self):
        for index, expected in ((0, ""), (-1, ""), (0.9, ""), (1, "A"),
                                (1.9, "A"), (2.5, "B"), (3.9, "C"), (4, ""),
                                ("1.9", "A"), ("bad", ""), (math.inf, "")):
            with self.subTest(index=index):
                self.assertEqual(operators.letter_of("ABC", index), expected)
        self.assertEqual(operators.letter_of("", 1), "")

    def test_letter_uses_scratch_text_and_utf16_units(self):
        self.assertEqual(operators.letter_of(123, 2), "2")
        self.assertEqual(operators.letter_of(1.0, 2), "")
        self.assertEqual(operators.letter_of(True, 1), "t")
        self.assertEqual(operators.letter_of("🙂A", 1), "\ud83d")
        self.assertEqual(operators.letter_of("🙂A", 2), "\ude42")
        self.assertEqual(operators.letter_of("🙂A", 3), "A")


if __name__ == "__main__":
    unittest.main()
