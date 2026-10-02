#include "testkit/testkit.h"

#include <stdarg.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

static int case_failed;
static const char *current_suite;
static const char *current_case;

void testkit_fail(const char *file, int line, const char *fmt, ...)
{
	if (!case_failed)
		printf("FAIL %s.%s\n", current_suite, current_case);
	case_failed = 1;
	printf("  %s:%d: ", file, line);
	va_list ap;
	va_start(ap, fmt);
	vprintf(fmt, ap);
	va_end(ap);
	putchar('\n');
}

int testkit_str_eq(const char *got, const char *want)
{
	if (got == NULL || want == NULL)
		return got == want;
	return strcmp(got, want) == 0;
}

int testkit_run(const char *suite, const TestCase *cases, size_t n)
{
	int failures = 0;
	int verbose = getenv("TESTKIT_VERBOSE") != NULL;
	current_suite = suite;
	for (size_t i = 0; i < n; i++) {
		current_case = cases[i].name;
		case_failed = 0;
		cases[i].fn();
		if (case_failed)
			failures++;
		else if (verbose)
			printf("PASS %s.%s\n", suite, cases[i].name);
		fflush(stdout);
	}
	if (failures > 0)
		printf("FAIL %s: %d of %zu cases failed\n", suite, failures, n);
	else
		printf("ok   %s (%zu cases)\n", suite, n);
	return failures > 0 ? 1 : 0;
}
