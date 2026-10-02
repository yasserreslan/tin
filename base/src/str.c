#include "base/str.h"

#include <string.h>

Str str_from_cstr(const char *s)
{
	return (Str){s, strlen(s)};
}

Str str_dup(Arena *a, const char *p, size_t n)
{
	char *d = arena_alloc(a, n + 1); /* zeroed, so the copy is NUL-terminated */
	if (n > 0)
		memcpy(d, p, n);
	return (Str){d, n};
}

bool str_eq(Str a, Str b)
{
	return a.n == b.n && (a.n == 0 || memcmp(a.p, b.p, a.n) == 0);
}

bool str_eq_cstr(Str a, const char *s)
{
	return a.n == strlen(s) && (a.n == 0 || memcmp(a.p, s, a.n) == 0);
}
