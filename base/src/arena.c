#include "base/arena.h"

#include <stdlib.h>
#include <string.h>

#include "base/util.h"

#define CHUNK_BYTES ((size_t)1 << 20)
#define ALIGN 16

struct ArenaChunk {
	ArenaChunk *next;
	size_t used, cap;
	_Alignas(ALIGN) char data[];
};

void arena_init(Arena *a)
{
	a->head = NULL;
}

void *arena_alloc(Arena *a, size_t n)
{
	n = size_add(n, ALIGN - 1) & ~(size_t)(ALIGN - 1);
	ArenaChunk *c = a->head;
	if (c == NULL || c->cap - c->used < n) {
		size_t cap = n > CHUNK_BYTES ? n : CHUNK_BYTES;
		c = calloc(1, size_add(sizeof(ArenaChunk), cap));
		if (c == NULL)
			oom();
		c->cap = cap;
		c->next = a->head;
		a->head = c;
	}
	void *p = c->data + c->used;
	c->used += n;
	return p;
}

void *arena_alloc_array(Arena *a, size_t count, size_t size)
{
	return arena_alloc(a, size_mul(count, size));
}

void *arena_dup(Arena *a, const void *p, size_t n)
{
	void *d = arena_alloc(a, n);
	if (n > 0)
		memcpy(d, p, n);
	return d;
}

void arena_free(Arena *a)
{
	ArenaChunk *c = a->head;
	while (c != NULL) {
		ArenaChunk *next = c->next;
		free(c);
		c = next;
	}
	a->head = NULL;
}
