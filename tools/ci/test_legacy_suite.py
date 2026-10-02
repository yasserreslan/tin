"""Test the legacy suite runner itself: a green run must mean the programs really passed."""
import contextlib
import io
from pathlib import Path
import sys
import tempfile
import unittest
from unittest.mock import patch
import legacy_suite


class ExpectationTests(unittest.TestCase):
    def test_expect_lines_become_stdout(self):
        out, status = legacy_suite.expectations('x();   // expect: 7\ny();\n// expect: a b\n')
        self.assertEqual(out, '7\na b\n')
        self.assertEqual(status, 0)

    def test_only_one_leading_space_is_trimmed(self):
        out, _ = legacy_suite.expectations('// expect:  two\n// expect:none\n')
        self.assertEqual(out, ' two\nnone\n')

    def test_exit_status(self):
        self.assertEqual(legacy_suite.expectations('// exit: 3\n')[1], 3)
        self.assertEqual(legacy_suite.expectations('// exit: x\n')[1], 0)

    def test_no_expectations_means_empty_output_and_success(self):
        self.assertEqual(legacy_suite.expectations('fn main() {}\n'), ('', 0))


class CaseTests(unittest.TestCase):
    def setUp(self):
        self.dir = tempfile.TemporaryDirectory()
        self.root = Path(self.dir.name)
        (self.root / 'tests/errors').mkdir(parents=True)
        self.work = self.root / 'work'
        self.work.mkdir()

    def tearDown(self):
        self.dir.cleanup()

    def program(self, text='// expect: hi\nfn main() { return 0; }\n'):
        path = self.root / 'tests/p.tin'
        path.write_text(text)
        return path

    def error_case(self, text='// error: boom\nfn main() { }\n'):
        path = self.root / 'tests/errors/e.tin'
        path.write_text(text)
        return path

    def check(self, fn, path, outcomes):
        """Runs fn with execute() replaced by canned outcomes, in call order."""
        with patch.object(legacy_suite, 'execute', side_effect=outcomes):
            return fn(Path('/compiler'), 'native', path, self.work, self.root)

    def test_program_passes(self):
        self.assertIsNone(self.check(legacy_suite.run_program, self.program(),
                                     [(0, b'', b''), (0, b'hi\n', b'')]))

    def test_wrong_stdout_fails(self):
        problem = self.check(legacy_suite.run_program, self.program(), [(0, b'', b''), (0, b'bye\n', b'')])
        self.assertIn('stdout differs', problem)

    def test_wrong_exit_status_fails(self):
        problem = self.check(legacy_suite.run_program, self.program(), [(0, b'', b''), (1, b'hi\n', b'')])
        self.assertIn('exit status 1, want 0', problem)

    def test_expected_nonzero_exit_passes(self):
        path = self.program('// expect: x\n// exit: 4\n')
        self.assertIsNone(self.check(legacy_suite.run_program, path, [(0, b'', b''), (4, b'x\n', b'')]))

    def test_output_then_crash_is_failure(self):
        problem = self.check(legacy_suite.run_program, self.program(), [(0, b'', b''), (-11, b'hi\n', b'')])
        self.assertIsNotNone(problem)

    def test_timeout_is_failure_even_with_expected_output(self):
        problem = self.check(legacy_suite.run_program, self.program(), [(0, b'', b''), (124, b'hi\n', b'')])
        self.assertIsNotNone(problem)

    def test_build_failure_never_runs_a_stale_binary(self):
        with patch.object(legacy_suite, 'execute', side_effect=[(1, b'', b'no')]) as run:
            problem = legacy_suite.run_program(Path('/compiler'), 'native', self.program(), self.work, self.root)
        self.assertIn('build failed', problem)
        self.assertEqual(run.call_count, 1)

    def test_error_case_passes_when_rejected_with_the_message(self):
        self.assertIsNone(self.check(legacy_suite.run_error_case, self.error_case(),
                                     [(1, b'', b'f.tin:1:1: error: boom here\n')]))

    def test_error_case_fails_when_the_program_compiles(self):
        self.assertIn('compiled successfully',
                      self.check(legacy_suite.run_error_case, self.error_case(), [(0, b'', b'')]))

    def test_error_case_fails_on_a_compiler_crash(self):
        for status in (-11, 2, 124):
            problem = self.check(legacy_suite.run_error_case, self.error_case(), [(status, b'', b'boom')])
            self.assertIn('did not reject cleanly', problem)

    def test_error_case_fails_on_the_wrong_message(self):
        self.assertIn('does not contain',
                      self.check(legacy_suite.run_error_case, self.error_case(), [(1, b'', b'other\n')]))

    def test_error_case_needs_its_promise_on_line_one(self):
        self.assertIn('first line', self.check(legacy_suite.run_error_case, self.error_case('fn main() {}\n'), []))


class RunTests(unittest.TestCase):
    def test_no_programs_is_failure(self):
        with tempfile.TemporaryDirectory() as directory, contextlib.redirect_stdout(io.StringIO()):
            (Path(directory) / 'tests').mkdir()
            self.assertFalse(legacy_suite.run(Path('/compiler'), root=Path(directory)))

    def test_one_failing_case_fails_the_run(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / 'tests/errors').mkdir(parents=True)
            (root / 'tests/good.tin').write_text('// expect: ok\n')
            (root / 'tests/bad.tin').write_text('// expect: ok\n')

            def fake(command, **kwargs):
                if command[0] == '/compiler':
                    return 0, b'', b''
                return 0, (b'ok\n' if command[0].endswith('good') else b'wrong\n'), b''

            output = io.StringIO()
            with patch.object(legacy_suite, 'execute', side_effect=fake), contextlib.redirect_stdout(output):
                self.assertFalse(legacy_suite.run(Path('/compiler'), root=root))
            self.assertIn('PASS good.tin', output.getvalue())
            self.assertIn('FAIL bad.tin', output.getvalue())

    def test_all_passing_cases_pass_the_run(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / 'tests/errors').mkdir(parents=True)
            (root / 'tests/good.tin').write_text('// expect: ok\n')
            (root / 'tests/errors/e.tin').write_text('// error: nope\n')

            def fake(command, **kwargs):
                if command[0] == '/compiler':
                    return (1, b'', b'nope') if command[-1].endswith('e.tin') else (0, b'', b'')
                return 0, b'ok\n', b''

            with patch.object(legacy_suite, 'execute', side_effect=fake), contextlib.redirect_stdout(io.StringIO()):
                self.assertTrue(legacy_suite.run(Path('/compiler'), root=root))


class ExecuteTests(unittest.TestCase):
    def test_real_process_timeout(self):
        result = legacy_suite.execute([sys.executable, '-c', 'import time; time.sleep(10)'], timeout=.05)
        self.assertEqual(result[0], 124)


if __name__ == '__main__':
    unittest.main()
