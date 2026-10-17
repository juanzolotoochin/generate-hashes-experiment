"""Greeting helpers."""

import six


def greeting(name):
    """Return a greeting for name."""
    return six.text_type("hello, {}").format(name)
