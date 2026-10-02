#include <string.h>

#include "base/buf.h"
#include "testkit/testkit.h"

static void zero_buf_is_empty_and_valid(void)
{
	Buf b = {0};
	CHECK_EQ_STR(buf_cstr(&b), "");
	CHECK_EQ_INT(buf_str(&b).n, 0);
	buf_free(&b);
}

static void writes_append_and_stay_terminated(void)
{
	Buf b = {0};
	buf_puts(&b, "ab");
	buf_putc(&b, 'c');
	buf_write(&b, "def", 2);
	CHECK_EQ_STR(b.p, "abcde");
	CHECK_EQ_INT(b.n, 5);
	buf_free(&b);
}

static void zero_length_write_terminates(void)
{
	Buf b = {0};
	buf_write(&b, "", 0);
	CHECK(b.p != NULL);
	CHECK_EQ_INT(b.p[0], 0);
	buf_free(&b);
}

static void printf_formats_and_appends(void)
{
	Buf b = {0};
	buf_printf(&b, "%d-%s", 42, "x");
	buf_printf(&b, "|%05.1f", 3.14159);
	CHECK_EQ_STR(b.p, "42-x|003.1");
	buf_free(&b);
}

static void printf_longer_than_the_buffer_grows_it(void)
{
	Buf b = {0};
	for (int i = 0; i < 1000; i++)
		buf_printf(&b, "line %d of a long output\n", i);
	CHECK(b.n > 20000);
	CHECK(b.cap > b.n);
	CHECK(strncmp(b.p, "line 0 of", 9) == 0);
	CHECK(strstr(b.p, "line 999 of a long output\n") != NULL);
	buf_free(&b);
}

static void embedded_nul_is_preserved(void)
{
	Buf b = {0};
	buf_write(&b, "a\0b", 3);
	CHECK_EQ_INT(b.n, 3);
	CHECK_EQ_INT(buf_str(&b).n, 3);
	CHECK_EQ_INT(buf_str(&b).p[2], 'b');
	buf_free(&b);
}

static void free_resets(void)
{
	Buf b = {0};
	buf_puts(&b, "x");
	buf_free(&b);
	CHECK(b.p == NULL);
	CHECK_EQ_INT(b.n, 0);
	buf_puts(&b, "y"); /* reusable */
	CHECK_EQ_STR(b.p, "y");
	buf_free(&b);
}

static const TestCase cases[] = {
	TEST_CASE(zero_buf_is_empty_and_valid),
	TEST_CASE(writes_append_and_stay_terminated),
	TEST_CASE(zero_length_write_terminates),
	TEST_CASE(printf_formats_and_appends),
	TEST_CASE(printf_longer_than_the_buffer_grows_it),
	TEST_CASE(embedded_nul_is_preserved),
	TEST_CASE(free_resets),
};

TESTKIT_MAIN("base/buf", cases)
