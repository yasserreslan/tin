/* testkit: the unit-test framework for the native packages.
 *
 * Each `<package>/tests/<name>_test.c` is its own program: a list of cases and TESTKIT_MAIN.
 * Cases are void functions. CHECK* stop the case at the first failure (they `return`, so use them
 * directly in the case function); EXPECT* record the failure and keep going. A failing case prints
 * `FAIL suite.case` with the reasons; the program ends with one `ok` or `FAIL` line for the suite
 * and exits 1 if any case failed. TESTKIT_VERBOSE=1 also lists passing cases.
 *
 *     static void adds(void) { CHECK_EQ_INT(1 + 1, 2); }
 *     static const TestCase cases[] = { TEST_CASE(adds) };
 *     TESTKIT_MAIN("math", cases)
 */
#ifndef TESTKIT_TESTKIT_H
#define TESTKIT_TESTKIT_H

#include <stddef.h>

typedef struct {
	const char *name;
	void (*fn)(void);
} TestCase;

#define TEST_CASE(fn) {#fn, fn}

/* Runs every case and returns the process exit status. */
int testkit_run(const char *suite, const TestCase *cases, size_t n);

/* Records a failure of the running case; the macros below call this. */
void testkit_fail(const char *file, int line, const char *fmt, ...)
#if defined(__GNUC__) || defined(__clang__)
	__attribute__((format(printf, 3, 4)))
#endif
	;

/* Compares two C strings, treating NULL as different from every string. */
int testkit_str_eq(const char *got, const char *want);

#define EXPECT(cond)                                                             \
	do {                                                                     \
		if (!(cond))                                                     \
			testkit_fail(__FILE__, __LINE__, "expected: %s", #cond); \
	} while (0)

#define CHECK(cond)                                                              \
	do {                                                                     \
		if (!(cond)) {                                                   \
			testkit_fail(__FILE__, __LINE__, "expected: %s", #cond); \
			return;                                                  \
		}                                                                \
	} while (0)

#define EXPECT_EQ_INT(got, want)                                                                \
	do {                                                                                    \
		long long got_ = (long long)(got), want_ = (long long)(want);                   \
		if (got_ != want_)                                                              \
			testkit_fail(__FILE__, __LINE__, "%s: got %lld, want %lld", #got, got_, \
				     want_);                                                    \
	} while (0)

#define CHECK_EQ_INT(got, want)                                                                 \
	do {                                                                                    \
		long long got_ = (long long)(got), want_ = (long long)(want);                   \
		if (got_ != want_) {                                                            \
			testkit_fail(__FILE__, __LINE__, "%s: got %lld, want %lld", #got, got_, \
				     want_);                                                    \
			return;                                                                 \
		}                                                                               \
	} while (0)

#define EXPECT_EQ_STR(got, want)                                                                \
	do {                                                                                    \
		const char *got_ = (got), *want_ = (want);                                      \
		if (!testkit_str_eq(got_, want_))                                               \
			testkit_fail(__FILE__, __LINE__, "%s:\n  got:  \"%s\"\n  want: \"%s\"", \
				     #got, got_ ? got_ : "(null)", want_ ? want_ : "(null)");   \
	} while (0)

#define CHECK_EQ_STR(got, want)                                                                 \
	do {                                                                                    \
		const char *got_ = (got), *want_ = (want);                                      \
		if (!testkit_str_eq(got_, want_)) {                                             \
			testkit_fail(__FILE__, __LINE__, "%s:\n  got:  \"%s\"\n  want: \"%s\"", \
				     #got, got_ ? got_ : "(null)", want_ ? want_ : "(null)");   \
			return;                                                                 \
		}                                                                               \
	} while (0)

#define TESTKIT_MAIN(suite, cases)                                                    \
	int main(void)                                                                \
	{                                                                             \
		return testkit_run(suite, cases, sizeof(cases) / sizeof((cases)[0])); \
	}

#endif
