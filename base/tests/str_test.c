#include "base/str.h"
#include "testkit/testkit.h"

static void equality_compares_length_and_bytes(void)
{
	CHECK(str_eq(STR_LIT("abc"), STR_LIT("abc")));
	CHECK(!str_eq(STR_LIT("abc"), STR_LIT("abd")));
	CHECK(!str_eq(STR_LIT("abc"), STR_LIT("ab")));
	CHECK(str_eq(STR_LIT(""), (Str){NULL, 0}));
}

static void equality_sees_past_a_nul(void)
{
	Str a = {"a\0b", 3}, b = {"a\0c", 3};
	CHECK(!str_eq(a, b));
	CHECK(str_eq(a, a));
}

static void cstr_comparisons(void)
{
	CHECK(str_eq_cstr(STR_LIT("fn"), "fn"));
	CHECK(!str_eq_cstr(STR_LIT("fn"), "fnx"));
	CHECK(!str_eq_cstr(STR_LIT("fnx"), "fn"));
	CHECK(str_eq_cstr(str_from_cstr("hello"), "hello"));
}

static void dup_is_terminated_and_independent(void)
{
	Arena arena = {0};
	char src[] = "abc";
	Str d = str_dup(&arena, src, 3);
	src[0] = 'X';
	CHECK_EQ_STR(d.p, "abc");
	CHECK_EQ_INT(d.n, 3);
	Str e = str_dup(&arena, "", 0);
	CHECK_EQ_STR(e.p, "");
	arena_free(&arena);
}

static void lit_has_the_right_length(void)
{
	CHECK_EQ_INT(STR_LIT("hello").n, 5);
	CHECK_EQ_INT(STR_LIT("").n, 0);
}

static const TestCase cases[] = {
	TEST_CASE(equality_compares_length_and_bytes),
	TEST_CASE(equality_sees_past_a_nul),
	TEST_CASE(cstr_comparisons),
	TEST_CASE(dup_is_terminated_and_independent),
	TEST_CASE(lit_has_the_right_length),
};

TESTKIT_MAIN("base/str", cases)
