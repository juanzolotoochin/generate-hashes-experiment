import unittest

from py.lib import greet


class GreetTest(unittest.TestCase):
    def test_greeting(self):
        self.assertEqual(greet.greeting("bazel"), "hello, bazel")


if __name__ == "__main__":
    unittest.main()
