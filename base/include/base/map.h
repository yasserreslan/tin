/* Map: a hash map from Str keys to pointers.
 *
 * Open addressing with a load factor of one half; storage comes from the Arena given to map_new.
 * Keys are stored by reference, so their bytes must outlive the map. A missing key reads as NULL,
 * so store non-NULL values. */
#ifndef BASE_MAP_H
#define BASE_MAP_H

#include <stddef.h>

#include "base/arena.h"
#include "base/str.h"

typedef struct Map Map;

Map *map_new(Arena *a);

/* The value stored under key, or NULL. */
void *map_get(const Map *m, Str key);

/* Stores val under key, replacing any previous value. */
void map_set(Map *m, Str key, void *val);

size_t map_len(const Map *m);

#endif
