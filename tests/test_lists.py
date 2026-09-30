import importlib
import math
from pathlib import Path
import sys
import types
import unittest
from unittest.mock import patch


# Load the data runtime without starting Pygame or importing rendering modules.
engine_path = Path(__file__).resolve().parents[1] / "engine"
for name, path in (
    ("_sb3topy_test_engine", engine_path),
    ("_sb3topy_test_engine.types", engine_path / "types"),
):
    package = types.ModuleType(name)
    package.__path__ = [str(path)]
    sys.modules[name] = package
lists = importlib.import_module("_sb3topy_test_engine.types.lists")
List = lists.List


class ListRuntimeTests(unittest.TestCase):
    def test_numeric_indices_are_one_based_and_floored(self):
        values = List(["a", "b", "c"])
        for index, expected in ((1, "a"), ("2", "b"), (2.9, "b"), ("1.9", "a")):
            with self.subTest(index=index):
                self.assertEqual(values[index], expected)
        values["2.9"] = "B"
        values.insert("2.9", "new")
        values.delete("1.9")
        self.assertEqual(values.list, ["new", "B", "c"])

    def test_special_indices_work_for_current_list_operations(self):
        values = List(["a", "b"])
        self.assertEqual(values["last"], "b")
        values["last"] = "B"
        values.insert("last", "c")
        self.assertEqual(values.list, ["a", "B", "c"])
        values.delete("last")
        self.assertEqual(values.list, ["a", "B"])
        values.delete("all")
        self.assertEqual(values.list, [])

    def test_random_and_any_include_the_last_insertion_slot(self):
        for keyword in ("random", "any"):
            with self.subTest(keyword=keyword):
                values = List(["a", "b"])
                with patch.object(lists.random, "randint", return_value=3) as draw:
                    values.insert(keyword, "c")
                    draw.assert_called_once_with(1, 3)
                self.assertEqual(values.list, ["a", "b", "c"])
                with patch.object(lists.random, "randint", return_value=2) as draw:
                    self.assertEqual(values[keyword], "b")
                    draw.assert_called_once_with(1, 3)
                with patch.object(lists.random, "randint", return_value=2):
                    values[keyword] = "B"
                    values.delete(keyword)
                self.assertEqual(values.list, ["a", "c"])

    def test_invalid_indices_report_empty_and_do_not_modify_the_list(self):
        for index in (0, -1, 0.9, 4, "bad", "", "all", "LAST", math.inf, -math.inf, math.nan):
            with self.subTest(index=index):
                values = List(["a", "b"])
                self.assertEqual(values[index], "")
                values[index] = "changed"
                values.insert(index, "changed")
                if index != "all":
                    values.delete(index)
                self.assertEqual(values.list, ["a", "b"])

    def test_empty_lists_handle_special_indices_without_random_draws(self):
        for index in ("last", "random", "any", "first"):
            with self.subTest(index=index):
                values = List([])
                with patch.object(lists.random, "randint") as draw:
                    self.assertEqual(values.get(index), "")
                    values.set(index, "changed")
                    values.delete2(index)
                    draw.assert_not_called()
                self.assertEqual(values.list, [])

    def test_empty_lists_allow_insertion_at_special_indices(self):
        for index in (1, "1", "last", "random", "any"):
            with self.subTest(index=index):
                values = List([])
                values.insert(index, "first")
                self.assertEqual(values.list, ["first"])

    def test_legacy_first_alias_is_preserved(self):
        values = List(["a", "b"])
        self.assertEqual(values.get("first"), "a")
        values.set("first", "A")
        values.insert2("first", "new")
        values.delete2("first")
        self.assertEqual(values.list, ["A", "b"])

    def test_join_uses_no_separator_only_for_single_character_strings(self):
        for items, expected in (
            ([], ""),
            (["A", "b", "中"], "Ab中"),
            (["🙂", "A"], "🙂 A"),
            (["hello", "WORLD"], "hello WORLD"),
            ([1, 2], "1 2"),
            (["a", 2], "a 2"),
            ([1.0, 2.5, True, False], "1 2.5 true false"),
            ([math.inf, -math.inf, math.nan], "Infinity -Infinity NaN"),
        ):
            with self.subTest(items=items):
                self.assertEqual(List(items).join(), expected)


if __name__ == "__main__":
    unittest.main()
