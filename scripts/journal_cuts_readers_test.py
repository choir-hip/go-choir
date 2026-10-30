#!/usr/bin/env python3
"""Readers of the run events journal cuts R1 and R3 changed.

docs/archive/problems/paper-dolt-journal-grows-1-6-gib-per-10-minutes-2026-10-22.md,
"Build: journal cuts R1 to R4". No network, no host: it imports the two
scripts and feeds them event lists.

  python3 -I scripts/journal_cuts_readers_test.py

Ways the readers can fail, written first:
  - the latency probe finds no model call once the request-side progress line
    is bus-only, because it paired two lines and one is gone;
  - an old tape (two lines per call) stops pairing, or counts a call twice
    when both the pairing and the new field are present;
  - the retrieval extractor reads no cell once tool.invoked is bus-only, or
    reads a cell twice from a tape that holds the invoked row and the result.
"""
import importlib.util
import os
import unittest

HERE = os.path.dirname(os.path.abspath(__file__))


def load(name):
    spec = importlib.util.spec_from_file_location(name, os.path.join(HERE, name + ".py"))
    mod = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(mod)
    return mod


def progress(ts, payload):
    return {"kind": "loop.progress", "ts": ts, "payload": payload}


class ModelCalls(unittest.TestCase):
    def setUp(self):
        self.probe = load("management_latency_probe")

    def test_new_shape_reads_the_latency_off_the_response_line(self):
        events = [
            progress("2026-10-22T19:00:03Z", {"iteration": 1, "phase": "tool_loop", "stop_reason": "tool_use", "input_tokens": 100, "model": "m1", "provider_call_ms": 2500}),
            progress("2026-10-22T19:00:09Z", {"iteration": 2, "phase": "tool_loop", "stop_reason": "end_turn", "input_tokens": 120, "llm_model": "m1", "provider_call_ms": 400}),
        ]
        calls = self.probe.model_calls(events)
        self.assertEqual([(c["iteration"], c["s"], c["input_tokens"], c["model"]) for c in calls],
                         [(1, 2.5, 100, "m1"), (2, 0.4, 120, "m1")])

    def test_old_tape_still_pairs_the_request_line_with_the_response(self):
        events = [
            progress("2026-10-22T19:00:00Z", {"iteration": 1, "phase": "provider_call_started", "messages": 1, "message_roles": ["user"]}),
            progress("2026-10-22T19:00:04Z", {"iteration": 1, "phase": "tool_loop", "stop_reason": "end_turn", "input_tokens": 50, "model": "m1"}),
        ]
        calls = self.probe.model_calls(events)
        self.assertEqual([(c["iteration"], c["s"]) for c in calls], [(1, 4.0)])

    def test_a_response_with_the_field_is_not_counted_twice_when_the_request_line_is_present(self):
        events = [
            progress("2026-10-22T19:00:00Z", {"iteration": 1, "phase": "provider_call_started", "messages": 1}),
            progress("2026-10-22T19:00:04Z", {"iteration": 1, "phase": "tool_loop", "stop_reason": "end_turn", "input_tokens": 50, "model": "m1", "provider_call_ms": 3900}),
        ]
        calls = self.probe.model_calls(events)
        self.assertEqual([(c["iteration"], c["s"]) for c in calls], [(1, 3.9)])


class CellSources(unittest.TestCase):
    def setUp(self):
        self.extract = load("retrieval_eval_extract")

    def test_cells_come_from_results_when_invoked_is_bus_only(self):
        evs = [
            {"seq": 2, "kind": "tool.result", "payload": {"tool": "desk_go_eval", "call_id": "b", "arguments": {"source": "second"}}},
            {"seq": 1, "kind": "tool.result", "payload": {"tool": "desk_go_eval", "call_id": "a", "arguments": {"source": "first"}}},
        ]
        self.assertEqual(self.extract.cell_sources(evs), ["first", "second"])

    def test_old_tape_cells_come_from_invoked_and_are_not_doubled_by_their_results(self):
        evs = [
            {"seq": 1, "kind": "tool.invoked", "payload": {"tool": "desk_go_eval", "call_id": "a", "arguments": {"source": "first"}}},
            {"seq": 2, "kind": "tool.result", "payload": {"tool": "desk_go_eval", "call_id": "a", "output": "{}"}},
            {"seq": 3, "kind": "tool.invoked", "payload": {"tool": "desk_go_eval", "call_id": "b", "arguments": {"source": "second"}}},
        ]
        self.assertEqual(self.extract.cell_sources(evs), ["first", "second"])

    def test_a_tape_that_crosses_the_cutover_counts_each_call_once(self):
        evs = [
            {"seq": 1, "kind": "tool.invoked", "payload": {"tool": "desk_go_eval", "call_id": "a", "arguments": {"source": "old"}}},
            {"seq": 2, "kind": "tool.result", "payload": {"tool": "desk_go_eval", "call_id": "a", "arguments": {"source": "old"}}},
            {"seq": 3, "kind": "tool.result", "payload": {"tool": "desk_go_eval", "call_id": "n", "arguments": {"source": "new"}}},
        ]
        self.assertEqual(self.extract.cell_sources(evs), ["old", "new"])


if __name__ == "__main__":
    unittest.main()
