#include "base/vec.h"

#include <string.h>

#include "base/util.h"

void *vec_grow(Arena *a, void *old, size_t n, size_t *cap, size_t elem_size)
{
	size_t ncap = *cap ? size_mul(*cap, 2) : 8;
	void *v = arena_alloc_array(a, ncap, elem_size);
	if (n > 0)
		memcpy(v, old, n * elem_size);
	*cap = ncap;
	return v;
}
