#include "base/diag.h"

#include <stdio.h>
#include <string.h>

bool pos_eq(Pos a, Pos b)
{
	const char *fa = a.file ? a.file : "";
	const char *fb = b.file ? b.file : "";
	return a.line == b.line && a.col == b.col && strcmp(fa, fb) == 0;
}

void pos_format(Pos p, Buf *out)
{
	buf_printf(out, "%s:%d:%d", p.file ? p.file : "", p.line, p.col);
}

void diag_init(Diag *d, Arena *arena)
{
	*d = (Diag){.arena = arena};
}

void diag_verror(Diag *d, Pos pos, const char *fmt, va_list ap)
{
	va_list again;
	va_copy(again, ap);
	int n = vsnprintf(NULL, 0, fmt, again);
	va_end(again);
	if (n < 0)
		oom();
	char *msg = arena_alloc(d->arena, (size_t)n + 1);
	vsnprintf(msg, (size_t)n + 1, fmt, ap);
	DiagItem item = {DIAG_ERROR, pos, {msg, (size_t)n}};
	vec_push(d->arena, &d->items, item);
}

void diag_error(Diag *d, Pos pos, const char *fmt, ...)
{
	va_list ap;
	va_start(ap, fmt);
	diag_verror(d, pos, fmt, ap);
	va_end(ap);
}

size_t diag_count(const Diag *d)
{
	return d->items.n;
}

void diag_render(const Diag *d, Buf *out)
{
	for (size_t i = 0; i < d->items.n; i++) {
		const DiagItem *it = &d->items.v[i];
		if (it->pos.file != NULL && it->pos.file[0] != '\0') {
			pos_format(it->pos, out);
			buf_puts(out, ": ");
		}
		buf_puts(out, "error: ");
		buf_write(out, it->msg.p, it->msg.n);
		buf_putc(out, '\n');
	}
}
