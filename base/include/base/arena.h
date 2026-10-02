/* Arena: bump allocation for data that all dies together.
 *
 * A compiler builds a graph of small objects (tokens, nodes, symbols) and drops all of it at once,
 * so each object is carved from a chunk and the whole arena is released in one call. Memory is
 * zeroed and 16-byte aligned. An Arena value must be zero-initialized (`Arena a = {0};`) or passed
 * to arena_init; arena_free returns it to that state.
 */
#ifndef BASE_ARENA_H
#define BASE_ARENA_H

#include <stddef.h>

typedef struct ArenaChunk ArenaChunk;

typedef struct {
	ArenaChunk *head;
} Arena;

void arena_init(Arena *a);

/* n zeroed bytes that stay valid until arena_free. Never returns NULL. */
void *arena_alloc(Arena *a, size_t n);

/* count * size zeroed bytes, with the multiplication checked. */
void *arena_alloc_array(Arena *a, size_t count, size_t size);

/* A copy of p[0..n) in the arena. */
void *arena_dup(Arena *a, const void *p, size_t n);

/* Releases every allocation made from a. */
void arena_free(Arena *a);

#define ARENA_NEW(arena, T) ((T *)arena_alloc((arena), sizeof(T)))
#define ARENA_NEW_ARRAY(arena, T, count) ((T *)arena_alloc_array((arena), (count), sizeof(T)))

#endif
