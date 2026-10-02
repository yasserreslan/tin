#include <errno.h>
#include <string.h>

#include "base/proc.h"
#include "testkit/testkit.h"

static void captures_output_and_exit_code(void)
{
	char *argv[] = {"sh", "-c", "echo out; echo err >&2; exit 3", NULL};
	Buf out = {0};
	ProcStatus st;
	CHECK(proc_run(argv, &out, &st));
	CHECK(st.exited);
	CHECK_EQ_INT(st.exit_code, 3);
	CHECK(strstr(buf_cstr(&out), "out\n") != NULL);
	CHECK(strstr(buf_cstr(&out), "err\n") != NULL);
	buf_free(&out);
}

static void success_is_exit_zero(void)
{
	char *argv[] = {"true", NULL};
	ProcStatus st;
	CHECK(proc_run(argv, NULL, &st));
	CHECK(st.exited);
	CHECK_EQ_INT(st.exit_code, 0);
	CHECK_EQ_INT(st.signal, 0);
}

static void a_missing_program_is_not_a_failed_program(void)
{
	char *argv[] = {"definitely-not-a-real-program-xyz", NULL};
	Buf out = {0};
	ProcStatus st;
	CHECK(proc_run(argv, &out, &st));
	CHECK(!st.exited);
	CHECK_EQ_INT(st.exec_errno, ENOENT);
	buf_free(&out);
}

static void a_signal_is_reported(void)
{
	char *argv[] = {"sh", "-c", "kill -9 $$", NULL};
	ProcStatus st;
	CHECK(proc_run(argv, NULL, &st));
	CHECK(!st.exited);
	CHECK_EQ_INT(st.signal, 9);
}

static void large_output_does_not_deadlock(void)
{
	char *argv[] = {"sh", "-c",
			"i=0; while [ $i -lt 20000 ]; do echo "
			"0123456789012345678901234567890123456789; i=$((i+1)); done",
			NULL};
	Buf out = {0};
	ProcStatus st;
	CHECK(proc_run(argv, &out, &st));
	CHECK_EQ_INT(st.exit_code, 0);
	CHECK_EQ_INT(out.n, 20000 * 41);
	buf_free(&out);
}

static void arguments_are_not_interpreted_by_a_shell(void)
{
	char *argv[] = {"printf", "%s", "a b;$(echo no)", NULL};
	Buf out = {0};
	ProcStatus st;
	CHECK(proc_run(argv, &out, &st));
	CHECK_EQ_STR(out.p, "a b;$(echo no)");
	buf_free(&out);
}

static int add_one(void *arg)
{
	return *(int *)arg + 1;
}

static int deep(int n)
{
	volatile char pad[256];
	pad[0] = (char)n;
	return n == 0 ? pad[0] : deep(n - 1) + 1 + (pad[0] & 0);
}

static int deep_job(void *arg)
{
	return deep(*(int *)arg);
}

static void run_with_stack_returns_the_result(void)
{
	int arg = 41, result = 0;
	CHECK(run_with_stack((size_t)1 << 20, add_one, &arg, &result));
	CHECK_EQ_INT(result, 42);
}

static void run_with_stack_survives_recursion_the_default_stack_would_not(void)
{
	int depth = 200000, result = 0; /* ~50 MB of frames */
	CHECK(run_with_stack((size_t)256 << 20, deep_job, &depth, &result));
	CHECK_EQ_INT(result, 200000);
}

static const TestCase cases[] = {
	TEST_CASE(captures_output_and_exit_code),
	TEST_CASE(success_is_exit_zero),
	TEST_CASE(a_missing_program_is_not_a_failed_program),
	TEST_CASE(a_signal_is_reported),
	TEST_CASE(large_output_does_not_deadlock),
	TEST_CASE(arguments_are_not_interpreted_by_a_shell),
	TEST_CASE(run_with_stack_returns_the_result),
	TEST_CASE(run_with_stack_survives_recursion_the_default_stack_would_not),
};

TESTKIT_MAIN("base/proc", cases)
