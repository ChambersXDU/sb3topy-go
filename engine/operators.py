"""
operators.py

Contains functions primarily used by project.py to ensure maximum
compatibility.
"""

__all__ = [
    'tonum', 'toint', 'tobool', 'tostr', 'string_length', 'letter_of', 'pick_rand',
    'gt', 'lt', 'eq', 'div', 'sqrt'
]

import math
import random


def tonum(value):
    """Attempt to cast a value to a number"""
    try:
        value = float(value)
        if value.is_integer():
            return int(value)
        if math.isnan(value):
            return 0
        return value
    except ValueError:
        return 0


def toint(value):
    """Round repeat counts as Scratch does, with ties toward positive infinity."""
    try:
        return math.floor(tonum(value) + 0.5)
    except OverflowError:
        return 0


def tobool(value):
    """Convert conditions using Scratch's string and numeric rules."""
    if isinstance(value, str):
        return value != '' and value != '0' and value.lower() != 'false'
    if isinstance(value, float) and math.isnan(value):
        return False
    return bool(value)


def tostr(value):
    """Format values for Scratch's text reporters."""
    if isinstance(value, bool):
        return 'true' if value else 'false'
    if value is None:
        return 'null'
    if isinstance(value, float):
        if math.isnan(value):
            return 'NaN'
        if math.isinf(value):
            return 'Infinity' if value > 0 else '-Infinity'
        if value.is_integer():
            return str(int(value))
    return str(value)


def string_length(value):
    """Count UTF-16 units, matching Scratch's character indexing."""
    return len(tostr(value).encode('utf-16-le', errors='surrogatepass')) // 2


def letter_of(text, index):
    """Read a one-based UTF-16 character using Scratch's bounds and truncation."""
    text = tostr(text)
    units = text.encode('utf-16-le', errors='surrogatepass')
    offset = tonum(index) - 1
    if offset < 0 or offset >= len(units) // 2:
        return ""
    start = math.floor(offset) * 2
    return units[start:start + 2].decode('utf-16-le', errors='surrogatepass')


def pick_rand(number1, number2):
    """Rand int or float depending on values"""
    number1, number2 = min(number1, number2), max(number1, number2)
    if isinstance(number1, float) or isinstance(number2, float):
        return random.random() * abs(number2-number1) + number1
    return random.randint(number1, number2)


def gt(value1, value2):  # pylint: disable=invalid-name
    """Either numerical or string comparison"""
    try:
        return float(value1) > float(value2)
    except ValueError:
        return tostr(value1).lower() > tostr(value2).lower()


def lt(value1, value2):  # pylint: disable=invalid-name
    """Either numerical or string comparison"""
    try:
        return float(value1) < float(value2)
    except ValueError:
        return tostr(value1).lower() < tostr(value2).lower()


def eq(value1, value2):  # pylint: disable=invalid-name
    """Either numerical or string comparison"""
    try:
        return float(value1) == float(value2)
    except ValueError:
        return tostr(value1).lower() == tostr(value2).lower()


def div(value1, value2):
    """Divide handling division by zero"""
    try:
        return tonum(value1) / tonum(value2)
    except ZeroDivisionError:
        return float('infinity')


def sqrt(value):
    """Gets the square root handling negative values"""
    try:
        return math.sqrt(value)
    except ValueError:
        return float('nan')
