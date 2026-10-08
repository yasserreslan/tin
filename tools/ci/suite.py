"""DEPRECATED: the strict suite is tools/ci/suite.tin (run it with tools/ci/tin.sh suite). Only the two helpers the Python checks
that are still waiting to be rewritten in Tin import are left here; this file goes when the last of them does."""
from pathlib import Path
import subprocess

ROOT = Path(__file__).resolve().parents[2]


def execute(command, *, cwd=ROOT, timeout=60, env=None):
    try:
        result = subprocess.run(command, cwd=cwd, env=env, capture_output=True, timeout=timeout)
        return result.returncode, result.stdout, result.stderr
    except subprocess.TimeoutExpired as exc:
        return 124, exc.stdout or b"", (exc.stderr or b"") + b"\nCI timeout\n"
