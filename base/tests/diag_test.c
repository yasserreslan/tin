#include "base/diag.h"
#include "testkit/testkit.h"

static void renders_position_and_message(void)
{
	Arena a = {0};
	Diag d;
	diag_init(&d, &a);
	diag_error(&d, (Pos){"a.tin", 3, 7}, "undefined: %s", "x");
	Buf out = {0};
	diag_render(&d, &out);
	CHECK_EQ_STR(out.p, "a.tin:3:7: error: undefined: x\n");
	buf_free(&out);
	arena_free(&a);
}

static void a_diagnostic_without_a_file_has_no_position(void)
{
	Arena a = {0};
	Diag d;
	diag_init(&d, &a);
	diag_error(&d, (Pos){0}, "no main function");
	diag_error(&d, (Pos){"", 1, 1}, "also none");
	Buf out = {0};
	diag_render(&d, &out);
	CHECK_EQ_STR(out.p, "error: no main function\nerror: also none\n");
	buf_free(&out);
	arena_free(&a);
}

static void keeps_order_and_counts(void)
{
	Arena a = {0};
	Diag d;
	diag_init(&d, &a);
	CHECK_EQ_INT(diag_count(&d), 0);
	for (int i = 0; i < 100; i++)
		diag_error(&d, (Pos){"f", i + 1, 1}, "e%d", i);
	CHECK_EQ_INT(diag_count(&d), 100);
	CHECK_EQ_STR(d.items.v[0].msg.p, "e0");
	CHECK_EQ_STR(d.items.v[99].msg.p, "e99");
	Buf out = {0};
	diag_render(&d, &out);
	CHECK(out.n > 100 * 10);
	buf_free(&out);
	arena_free(&a);
}

static void long_messages_are_not_truncated(void)
{
	Arena a = {0};
	Diag d;
	diag_init(&d, &a);
	char big[3000];
	for (int i = 0; i < 2999; i++)
		big[i] = 'z';
	big[2999] = '\0';
	diag_error(&d, (Pos){"f", 1, 1}, "%s", big);
	CHECK_EQ_INT(d.items.v[0].msg.n, 2999);
	arena_free(&a);
}

static void pos_equality_compares_file_by_value(void)
{
	char a1[] = "x.tin", a2[] = "x.tin";
	CHECK(pos_eq((Pos){a1, 1, 2}, (Pos){a2, 1, 2}));
	CHECK(!pos_eq((Pos){a1, 1, 2}, (Pos){a2, 1, 3}));
	CHECK(!pos_eq((Pos){a1, 1, 2}, (Pos){"y.tin", 1, 2}));
	CHECK(pos_eq((Pos){0}, (Pos){"", 0, 0}));
}

static void pos_format_matches_the_go_output(void)
{
	Buf b = {0};
	pos_format((Pos){"a.tin", 3, 7}, &b);
	CHECK_EQ_STR(b.p, "a.tin:3:7");
	buf_free(&b);
}

static const TestCase cases[] = {
	TEST_CASE(renders_position_and_message),
	TEST_CASE(a_diagnostic_without_a_file_has_no_position),
	TEST_CASE(keeps_order_and_counts),
	TEST_CASE(long_messages_are_not_truncated),
	TEST_CASE(pos_equality_compares_file_by_value),
	TEST_CASE(pos_format_matches_the_go_output),
};

TESTKIT_MAIN("base/diag", cases)
