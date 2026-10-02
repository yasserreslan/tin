#include <stdint.h>
#include <string.h>

#include "base/arena.h"
#include "testkit/testkit.h"

static void memory_is_zeroed_and_aligned(void)
{
	Arena a = {0};
	for (size_t n = 1; n < 200; n += 7) {
		unsigned char *p = arena_alloc(&a, n);
		CHECK(((uintptr_t)p & 15) == 0);
		for (size_t i = 0; i < n; i++)
			CHECK_EQ_INT(p[i], 0);
		memset(p, 0xAB, n); /* dirty it: the next allocation must still read zero */
	}
	arena_free(&a);
}

static void allocations_do_not_overlap(void)
{
	Arena a = {0};
	char *x = arena_alloc(&a, 24);
	char *y = arena_alloc(&a, 24);
	memset(x, 'x', 24);
	memset(y, 'y', 24);
	for (int i = 0; i < 24; i++) {
		CHECK_EQ_INT(x[i], 'x');
		CHECK_EQ_INT(y[i], 'y');
	}
	arena_free(&a);
}

static void large_allocation_gets_its_own_chunk(void)
{
	Arena a = {0};
	size_t big = (size_t)4 << 20;
	char *p = arena_alloc(&a, big);
	p[0] = 1;
	p[big - 1] = 2;
	char *q = arena_alloc(&a, 16);
	CHECK(q != NULL);
	CHECK_EQ_INT(p[0], 1);
	CHECK_EQ_INT(p[big - 1], 2);
	arena_free(&a);
}

static void many_small_allocations_cross_chunks(void)
{
	Arena a = {0};
	int **ptrs = arena_alloc_array(&a, 100000, sizeof(int *));
	for (int i = 0; i < 100000; i++) {
		ptrs[i] = ARENA_NEW(&a, int);
		*ptrs[i] = i;
	}
	for (int i = 0; i < 100000; i++)
		CHECK_EQ_INT(*ptrs[i], i);
	arena_free(&a);
}

static void dup_copies_bytes(void)
{
	Arena a = {0};
	const char src[] = {'a', 0, 'b', 'c'};
	char *d = arena_dup(&a, src, sizeof src);
	CHECK(d != src);
	CHECK(memcmp(d, src, sizeof src) == 0);
	CHECK(arena_dup(&a, src, 0) != NULL);
	arena_free(&a);
}

static void free_resets_the_arena(void)
{
	Arena a = {0};
	arena_alloc(&a, 100);
	arena_free(&a);
	CHECK(a.head == NULL);
	CHECK(arena_alloc(&a, 100) != NULL); /* usable again */
	arena_free(&a);
}

static const TestCase cases[] = {
	TEST_CASE(memory_is_zeroed_and_aligned),
	TEST_CASE(allocations_do_not_overlap),
	TEST_CASE(large_allocation_gets_its_own_chunk),
	TEST_CASE(many_small_allocations_cross_chunks),
	TEST_CASE(dup_copies_bytes),
	TEST_CASE(free_resets_the_arena),
};

TESTKIT_MAIN("base/arena", cases)
