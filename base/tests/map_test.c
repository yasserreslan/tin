#include <stdio.h>

#include "base/map.h"
#include "testkit/testkit.h"

static void missing_key_reads_null(void)
{
	Arena a = {0};
	Map *m = map_new(&a);
	CHECK(map_get(m, STR_LIT("x")) == NULL);
	CHECK_EQ_INT(map_len(m), 0);
	arena_free(&a);
}

static void set_then_get(void)
{
	Arena a = {0};
	Map *m = map_new(&a);
	int one = 1, two = 2;
	map_set(m, STR_LIT("one"), &one);
	map_set(m, STR_LIT("two"), &two);
	CHECK(map_get(m, STR_LIT("one")) == &one);
	CHECK(map_get(m, STR_LIT("two")) == &two);
	CHECK(map_get(m, STR_LIT("three")) == NULL);
	CHECK_EQ_INT(map_len(m), 2);
	arena_free(&a);
}

static void set_replaces_without_growing_the_count(void)
{
	Arena a = {0};
	Map *m = map_new(&a);
	int x = 1, y = 2;
	map_set(m, STR_LIT("k"), &x);
	map_set(m, STR_LIT("k"), &y);
	CHECK(map_get(m, STR_LIT("k")) == &y);
	CHECK_EQ_INT(map_len(m), 1);
	arena_free(&a);
}

static void empty_key_is_a_key(void)
{
	Arena a = {0};
	Map *m = map_new(&a);
	int x = 1;
	map_set(m, STR_LIT(""), &x);
	CHECK(map_get(m, STR_LIT("")) == &x);
	arena_free(&a);
}

static void keys_differing_after_a_nul_are_distinct(void)
{
	Arena a = {0};
	Map *m = map_new(&a);
	int x = 1, y = 2;
	map_set(m, (Str){"a\0b", 3}, &x);
	map_set(m, (Str){"a\0c", 3}, &y);
	CHECK(map_get(m, (Str){"a\0b", 3}) == &x);
	CHECK(map_get(m, (Str){"a\0c", 3}) == &y);
	arena_free(&a);
}

static void many_keys_survive_rehashing(void)
{
	Arena a = {0};
	Map *m = map_new(&a);
	enum { N = 50000 };
	static int values[N];
	for (int i = 0; i < N; i++) {
		char *key = arena_alloc(&a, 24);
		snprintf(key, 24, "key%d", i);
		values[i] = i;
		map_set(m, str_from_cstr(key), &values[i]);
	}
	CHECK_EQ_INT(map_len(m), N);
	for (int i = 0; i < N; i++) {
		char key[24];
		snprintf(key, sizeof key, "key%d", i);
		int *got = map_get(m, str_from_cstr(key));
		CHECK(got == &values[i]);
	}
	CHECK(map_get(m, STR_LIT("key50000")) == NULL);
	arena_free(&a);
}

static const TestCase cases[] = {
	TEST_CASE(missing_key_reads_null),
	TEST_CASE(set_then_get),
	TEST_CASE(set_replaces_without_growing_the_count),
	TEST_CASE(empty_key_is_a_key),
	TEST_CASE(keys_differing_after_a_nul_are_distinct),
	TEST_CASE(many_keys_survive_rehashing),
};

TESTKIT_MAIN("base/map", cases)
