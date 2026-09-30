"""
lists.py

Handles custom list data structures
"""

import math
import random

from ..operators import tonum

__all__ = ['List', 'StaticList']


class List:
    """
    Emulates the correct list behavior

    Attributes:
        list: The internal list
    """

    __slots__ = ('list',)

    def __init__(self, values):
        self.list = values

    def __getitem__(self, key):
        index = list_index(key, len(self.list))
        if index is not None:
            return self.list[index]
        return ""

    def get(self, key):
        """
        Gets an item, supporting legacy indices
        (first, last, random)
        """
        return self[1 if key == 'first' else key]

    def __setitem__(self, key, value):
        index = list_index(key, len(self.list))
        if index is not None:
            self.list[index] = value

    def set(self, key, item):
        """
        Sets an item, supporting legacy indices
        (first, last, random)
        """
        self[1 if key == 'first' else key] = item

    def append(self, value):
        """Add an item to list"""
        self.list.append(value)

    def insert(self, key, value):
        """Insert an item in list"""
        index = list_index(key, len(self.list) + 1)
        if index is not None:
            self.list.insert(index, value)

    def insert2(self, key, item):
        """
        Inserts an item, supporting legacy indices
        (first, last random)
        """
        self.insert(1 if key == 'first' else key, item)

    def delete(self, key):
        """Remove an item from list"""
        if key == 'all':
            self.delete_all()
            return
        index = list_index(key, len(self.list))
        if index is not None:
            del self.list[index]

    def delete2(self, key):
        """
        Deletes an item, supporting legacy indices
        (first, last, random, all)
        """
        self.delete(1 if key == 'first' else key)

    def delete_all(self):
        """Deletes all items in list"""
        self.list = []

    def __contains__(self, item):
        item = search_str(item)
        return any(item == search_str(value) for value in self.list)

    def join(self):
        """Joins the list"""
        separator = '' if all(isinstance(item, str) and len(item) == 1 and ord(item) <= 0xffff
                              for item in self.list) else ' '
        return separator.join(item_text(item) for item in self.list)

    def __len__(self):
        return self.list.__len__()

    # TODO Variable/list reporters
    def show(self):
        """Print list"""
        print(self.list)

    def hide(self):
        """Do nothing"""

    def index(self, item):
        """Gets the position of item in list"""
        item = search_str(item)
        for i, value in enumerate(self.list):
            if item == search_str(value):
                return i + 1
        return 0

    def copy(self):
        """Return a copy of this List"""
        return self.__class__(self.list.copy())


class StaticList(List):
    """
    A list that doesn't change

    Attributes:
        list: Inherited from List, the internal list

        dict: Used to test if an item is contained in the list and to
            determine the index of items in the list.
    """

    __slots__ = ('dict',)

    def __init__(self, values):  # pylint: disable=super-init-not-called
        self.list = tuple(values)

        self.dict = {}
        for i, item in enumerate(values):
            self.dict.setdefault(search_str(item), i+1)

    def index(self, item):
        """Gets the position of item in list"""
        return self.dict.get(search_str(item), 0)

    def __contains__(self, item):
        return search_str(item) in self.dict

    def copy(self):
        """Returns self; this list is static"""
        return self


def list_index(key, length):
    """Resolve a Scratch index to a zero-based position, or None."""
    if key == 'last':
        return length - 1 if length else None
    if key in ('random', 'any'):
        return random.randint(1, length) - 1 if length else None
    number = tonum(key)
    if not math.isfinite(number):
        return None
    index = math.floor(number) - 1
    return index if 0 <= index < length else None


def item_text(value):
    """Preserve Scratch text when reporting mixed list contents."""
    if isinstance(value, bool):
        return 'true' if value else 'false'
    if isinstance(value, float):
        if math.isnan(value):
            return 'NaN'
        if math.isinf(value):
            return 'Infinity' if value > 0 else '-Infinity'
        if value.is_integer():
            return str(int(value))
    return str(value)


def search_str(value):
    """
    Gets a lowercase str for searching
    Also handles integer floats.
    """
    if isinstance(value, float) and value.is_integer():
        return str(int(value))
    return str(value).lower()
