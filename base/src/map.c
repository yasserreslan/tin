#include "base/map.h"

#include <stdint.h>

#include "base/util.h"

typedef struct {
	Str key;
	void *val;
	bool used;
} Entry;

struct Map {
	Arena *arena;
	Entry *e;
	size_t cap, n; /* cap is zero or a power of two; n counts used entries */
};

Map *map_new(Arena *a)
{
	Map *m = ARENA_NEW(a, Map);
	m->arena = a;
	return m;
}

/* FNV-1a. Keys come from source text the tool is compiling, not from a network peer, so a
 * keyed hash would only cost time. */
static uint64_t hash(Str s)
{
	uint64_t h = UINT64_C(1469598103934665603);
	for (size_t i = 0; i < s.n; i++) {
		h ^= (unsigned char)s.p[i];
		h *= UINT64_C(1099511628211);
	}
	return h;
}

void *map_get(const Map *m, Str key)
{
	if (m->cap == 0)
		return NULL;
	size_t mask = m->cap - 1;
	for (size_t i = hash(key) & mask; m->e[i].used; i = (i + 1) & mask)
		if (str_eq(m->e[i].key, key))
			return m->e[i].val;
	return NULL;
}

/* Stores into a table known to have a free slot. Returns true if key was new. */
static bool insert(Entry *e, size_t cap, Str key, void *val)
{
	size_t mask = cap - 1;
	size_t i = hash(key) & mask;
	for (; e[i].used; i = (i + 1) & mask) {
		if (str_eq(e[i].key, key)) {
			e[i].val = val;
			return false;
		}
	}
	e[i] = (Entry){key, val, true};
	return true;
}

void map_set(Map *m, Str key, void *val)
{
	if (size_mul(size_add(m->n, 1), 2) > m->cap) {
		size_t cap = m->cap ? size_mul(m->cap, 2) : 16;
		Entry *e = ARENA_NEW_ARRAY(m->arena, Entry, cap);
		for (size_t i = 0; i < m->cap; i++)
			if (m->e[i].used)
				insert(e, cap, m->e[i].key, m->e[i].val);
		m->e = e;
		m->cap = cap;
	}
	if (insert(m->e, m->cap, key, val))
		m->n++;
}

size_t map_len(const Map *m)
{
	return m->n;
}
