import copy
import pathlib
import sys
import unittest

import yaml

sys.path.insert(0, str(pathlib.Path(__file__).parent))
import gen_rules as g  # noqa: E402


def spec():
    return yaml.safe_load(g.DEFAULT_SPEC.read_text())


class GenRulesTest(unittest.TestCase):
    def test_budget(self):
        self.assertEqual(g.budget(99.9), 0.001)
        self.assertEqual(g.budget(99.0), 0.01)
        with self.assertRaises(g.SpecError):
            g.budget(100)

    def test_rule_counts_and_parse(self):
        doc = yaml.safe_load(g.generate(spec()))
        rules = [r for grp in doc["groups"] for r in grp["rules"]]
        self.assertEqual(sum("alert" in r for r in rules), 8)  # 2 SLOs x 4 burn rows
        self.assertEqual(sum("record" in r for r in rules), 20)  # 2 x (8 windows + budget + remaining)

    def test_thresholds_are_burn_times_budget(self):
        out = g.generate(spec())
        self.assertIn("> (14.4 * 0.001)", out)  # 99.9% availability
        self.assertIn("> (14.4 * 0.01)", out)  # 99% latency

    def test_latency_threshold_must_be_a_bucket(self):
        s = copy.deepcopy(spec())
        s["slos"][1]["sli"]["threshold_seconds"] = 0.35
        with self.assertRaises(g.SpecError):
            g.generate(s)

    def test_short_window_must_be_shorter(self):
        s = copy.deepcopy(spec())
        s["burn_policy"][0]["short"] = "2h"
        with self.assertRaises(g.SpecError):
            g.generate(s)

    def test_committed_rules_are_not_stale(self):
        self.assertEqual(g.DEFAULT_OUT.read_text(), g.generate(spec()))


if __name__ == "__main__":
    unittest.main()
