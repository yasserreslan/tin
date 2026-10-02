#include "base/buf.h"

#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

/* Makes room for extra more bytes plus the terminating NUL. */
static void reserve(Buf *b, size_t extra)
{
	size_t need = size_add(size_add(b->n, extra), 1);
	if (need <= b->cap)
		return;
	size_t cap = b->cap ? b->cap : 256;
	while (cap < need)
		cap = size_mul(cap, 2);
	char *p = realloc(b->p, cap);
	if (p == NULL)
		oom();
	b->p = p;
	b->cap = cap;
}

void buf_write(Buf *b, const void *p, size_t n)
{
	reserve(b, n);
	if (n > 0)
		memcpy(b->p + b->n, p, n);
	b->n += n;
	b->p[b->n] = '\0';
}

void buf_putc(Buf *b, char c)
{
	buf_write(b, &c, 1);
}

void buf_puts(Buf *b, const char *s)
{
	buf_write(b, s, strlen(s));
}

void buf_vprintf(Buf *b, const char *fmt, va_list ap)
{
	va_list again;
	va_copy(again, ap);
	int n = vsnprintf(NULL, 0, fmt, again);
	va_end(again);
	if (n < 0)
		oom(); /* an encoding error in a format string is a bug in the caller */
	reserve(b, (size_t)n);
	vsnprintf(b->p + b->n, (size_t)n + 1, fmt, ap);
	b->n += (size_t)n;
}

void buf_printf(Buf *b, const char *fmt, ...)
{
	va_list ap;
	va_start(ap, fmt);
	buf_vprintf(b, fmt, ap);
	va_end(ap);
}

Str buf_str(const Buf *b)
{
	return (Str){b->p ? b->p : "", b->n};
}

const char *buf_cstr(const Buf *b)
{
	return b->p ? b->p : "";
}

void buf_free(Buf *b)
{
	free(b->p);
	*b = (Buf){0};
}
