/* Str: a length-delimited byte string, not necessarily NUL-terminated and allowed to contain NULs
 * (Tin string literals can). A Str never owns its bytes; whoever created it decides the lifetime.
 * Copies made by str_dup live in an Arena and are also NUL-terminated, so they can be passed to C
 * functions. */
#ifndef BASE_STR_H
#define BASE_STR_H

#include <stdbool.h>
#include <stddef.h>

#include "base/arena.h"

typedef struct {
	const char *p;
	size_t n;
} Str;

/* A Str for a string literal (no strlen). */
#define STR_LIT(lit) ((Str){"" lit, sizeof(lit) - 1})

/* For printf: printf("name " STR_FMT, STR_ARG(s)). The argument is evaluated twice. */
#define STR_FMT "%.*s"
#define STR_ARG(s) (int)(s).n, (s).p

Str str_from_cstr(const char *s);
Str str_dup(Arena *a, const char *p, size_t n);
bool str_eq(Str a, Str b);
bool str_eq_cstr(Str a, const char *s);

#endif
