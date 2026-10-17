import unittest

from py.extra import extra


class ExtraTest(unittest.TestCase):
    def test_ascii_host(self):
        self.assertEqual(extra.ascii_host("bücher.example"), "xn--bcher-kva.example")


if __name__ == "__main__":
    unittest.main()
