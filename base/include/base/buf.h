/* Buf: a growable byte buffer on the heap, for text that is built up and then written or
 * inspected (generated assembly, diagnostics, a child process's output).
 *
 * Unlike an Arena, a Buf owns its memory: call buf_free when done. A zero Buf (`Buf b = {0};`) is
 * empty and valid. After any write, b.p is NUL-terminated (b.n does not count the NUL). */
#ifndef BASE_BUF_H
#define BASE_BUF_H

#include <stdarg.h>
#include <stddef.h>

#include "base/str.h"
#include "base/util.h"

typedef struct {
	char *p;
	size_t n, cap;
} Buf;

void buf_putc(Buf *b, char c);
void buf_write(Buf *b, const void *p, size_t n);
void buf_puts(Buf *b, const char *s);
void buf_printf(Buf *b, const char *fmt, ...) PRINTF_LIKE(2, 3);
void buf_vprintf(Buf *b, const char *fmt, va_list ap) VPRINTF_LIKE(2);

/* The contents as a Str (valid until the next write or buf_free). */
Str buf_str(const Buf *b);

/* The contents as a C string; "" for an empty Buf. */
const char *buf_cstr(const Buf *b);

void buf_free(Buf *b);

#endif
