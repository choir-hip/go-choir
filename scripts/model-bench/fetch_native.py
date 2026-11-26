#!/usr/bin/env python3
"""Fetch and verify the pinned native Android benchmark artifacts."""
import argparse
import os
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from native import CACHE, fetch_all, load_spec


def main():
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("--cache", default=CACHE)
    args = ap.parse_args()
    fetch_all(load_spec(), args.cache)
    print("native Android benchmark artifacts verified in", os.path.abspath(args.cache))


if __name__ == "__main__":
    main()
