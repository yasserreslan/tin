"""Fast contract tests for the compiler fuzzing generator and runner."""
import importlib.util
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]
GEN = ROOT / 'tools/fuzz/gen.py'
RUNNER = ROOT / 'tools/fuzz/run.py'
REDUCER = ROOT / 'tools/fuzz/reduce.py'
MUTATOR = ROOT / 'tools/fuzz/mutate.py'
COMPILER = Path(os.environ.get('TIN_COMPILER', ROOT / 'bin/tinc'))


def load_runner():
    spec = importlib.util.spec_from_file_location('tin_fuzz_runner', RUNNER)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


class FuzzRunnerTests(unittest.TestCase):
    def test_first_difference_is_stable(self):
        runner = load_runner()
        self.assertEqual(runner.first_difference('one\ntwo\n', 'one\nchanged\n'),
                         "line 2: Tin='two'; oracle='changed'")
        self.assertEqual(runner.first_difference('one\n', 'one\ntwo\n'),
                         "line 2: Tin='<EOF>'; oracle='two'")

    @unittest.skipUnless(GEN.is_file(), 'gen.py is supplied by the generator PR')
    def test_generator_is_deterministic(self):
        with tempfile.TemporaryDirectory(prefix='tin-fuzz-test-') as td:
            root = Path(td)
            for lang in ('tin', 'go'):
                a, b = root / f'a.{lang}', root / f'b.{lang}'
                subprocess.run([sys.executable, str(GEN), '--seed', '604', '--tin', str(a), '--go', str(root / 'a.go')], check=True, timeout=10)
                subprocess.run([sys.executable, str(GEN), '--seed', '604', '--tin', str(root / 'b.tin'), '--go', str(b)], check=True, timeout=10)
                self.assertEqual(a.read_bytes() if lang == 'tin' else b.read_bytes(),
                                 (root / f'b.{lang}').read_bytes() if lang == 'tin' else (root / f'a.{lang}').read_bytes())

    @unittest.skipUnless(GEN.is_file() and COMPILER.is_file(), 'generator or local Tin compiler is unavailable')
    def test_fifty_generated_pairs_compile_run_and_match(self):
        go = shutil.which('go')
        if not go:
            self.skipTest('Go is not installed; Go twin comparisons require Go')
        with tempfile.TemporaryDirectory(prefix='tin-fuzz-pairs-') as td:
            root = Path(td)
            for seed in range(50):
                work = root / str(seed)
                work.mkdir()
                tin, gofile, exe = work / 'prog.tin', work / 'prog.go', work / 'prog'
                subprocess.run([sys.executable, str(GEN), '--seed', str(seed), '--tin', str(tin), '--go', str(gofile)], check=True, timeout=10)
                subprocess.run([str(COMPILER), '-o', str(exe), str(tin)], cwd=ROOT, check=True, timeout=20, capture_output=True, text=True)
                actual = subprocess.run([str(exe)], cwd=ROOT, check=True, timeout=20, capture_output=True, text=True)
                expected = subprocess.run([go, 'run', str(gofile)], cwd=ROOT, check=True, timeout=30, capture_output=True, text=True)
                self.assertEqual(actual.stdout, expected.stdout, f'seed {seed}')

    @unittest.skipUnless(GEN.is_file() and COMPILER.is_file(), 'generator or local Tin compiler is unavailable')
    def test_runner_short_smoke(self):
        with tempfile.TemporaryDirectory(prefix='tin-fuzz-runner-') as td:
            p = subprocess.run([sys.executable, str(RUNNER), '--oracle', 'go', '--seconds', '1', '--jobs', '2',
                                '--out', str(Path(td) / 'out'), '--seed', '8100'], cwd=ROOT,
                               timeout=90, capture_output=True, text=True)
            self.assertEqual(p.returncode, 0, p.stdout + p.stderr)
            self.assertIn('completed ', p.stdout)

    @unittest.skipUnless(REDUCER.is_file(), 'reduce.py is supplied by another toolkit task')
    def test_reducer_planted_failure(self):
        self.skipTest('reducer contract test is owned by the reducer task')

    @unittest.skipUnless(MUTATOR.is_file(), 'mutate.py is supplied by another toolkit task')
    def test_mutator_small_corpus(self):
        self.skipTest('mutator contract test is owned by the mutator task')


if __name__ == '__main__':
    unittest.main()
