#include "base/vec.h"
#include "testkit/testkit.h"

typedef VEC(int) IntVec;
typedef struct {
	long a;
	char b;
} Pair;
typedef VEC(Pair) PairVec;

static void zero_vec_is_empty(void)
{
	IntVec v = {0};
	CHECK_EQ_INT(v.n, 0);
	CHECK(v.v == NULL);
}

static void push_keeps_every_element_across_growth(void)
{
	Arena a = {0};
	IntVec v = {0};
	for (int i = 0; i < 10000; i++)
		vec_push(&a, &v, i * 3);
	CHECK_EQ_INT(v.n, 10000);
	CHECK(v.cap >= v.n);
	for (int i = 0; i < 10000; i++)
		CHECK_EQ_INT(v.v[i], i * 3);
	arena_free(&a);
}

static void struct_elements(void)
{
	Arena a = {0};
	PairVec v = {0};
	for (int i = 0; i < 100; i++)
		vec_push(&a, &v, ((Pair){i, (char)('a' + i % 26)}));
	CHECK_EQ_INT(v.v[99].a, 99);
	CHECK_EQ_INT(v.v[27].b, 'b');
	arena_free(&a);
}

static void capacity_doubles(void)
{
	Arena a = {0};
	IntVec v = {0};
	vec_push(&a, &v, 1);
	CHECK_EQ_INT(v.cap, 8);
	for (int i = 0; i < 8; i++)
		vec_push(&a, &v, i);
	CHECK_EQ_INT(v.cap, 16);
	arena_free(&a);
}

static const TestCase cases[] = {
	TEST_CASE(zero_vec_is_empty),
	TEST_CASE(push_keeps_every_element_across_growth),
	TEST_CASE(struct_elements),
	TEST_CASE(capacity_doubles),
};

TESTKIT_MAIN("base/vec", cases)
