/* Vec: typed growable arrays whose storage lives in an Arena.
 *
 *     typedef VEC(Token) TokenVec;
 *     TokenVec toks = {0};
 *     vec_push(&arena, &toks, tok);        // toks.v[0 .. toks.n)
 *
 * Growth allocates a new array and copies; the old one stays in the arena until it is freed, so
 * build large vectors once and pass them on. A zero Vec is empty and valid. */
#ifndef BASE_VEC_H
#define BASE_VEC_H

#include <stddef.h>

#include "base/arena.h"

#define VEC(T)                 \
	struct {               \
		T *v;          \
		size_t n, cap; \
	}

/* Appends x to vec, growing it from arena when full. */
#define vec_push(arena, vec, x)                                                       \
	do {                                                                          \
		if ((vec)->n == (vec)->cap)                                           \
			(vec)->v = vec_grow((arena), (vec)->v, (vec)->n, &(vec)->cap, \
					    sizeof *(vec)->v);                        \
		(vec)->v[(vec)->n++] = (x);                                           \
	} while (0)

/* Implementation of vec_push: a larger copy of the first n elements; updates *cap. */
void *vec_grow(Arena *a, void *old, size_t n, size_t *cap, size_t elem_size);

#endif
