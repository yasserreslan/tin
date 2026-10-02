/* Diag: source positions and the diagnostics a tool reports against them.
 *
 * Passes never print. They add diagnostics to a Diag, and the driver renders them once, in the
 * order they were added. Keeping diagnostics as data (position, severity, message) is what lets a
 * test assert on them, and later lets a tool emit them as JSON or to an editor. */
#ifndef BASE_DIAG_H
#define BASE_DIAG_H

#include <stdarg.h>
#include <stdbool.h>
#include <stddef.h>

#include "base/arena.h"
#include "base/buf.h"
#include "base/util.h"
#include "base/vec.h"

/* A location in a source file. file == NULL or "" means "no position". */
typedef struct {
	const char *file;
	int line, col;
} Pos;

bool pos_eq(Pos a, Pos b);

/* Appends "file:line:col" to out. */
void pos_format(Pos p, Buf *out);

typedef enum { DIAG_ERROR } DiagSeverity;

typedef struct {
	DiagSeverity severity;
	Pos pos;
	Str msg;
} DiagItem;

typedef struct {
	Arena *arena; /* owns the messages; must outlive the Diag */
	VEC(DiagItem) items;
} Diag;

void diag_init(Diag *d, Arena *arena);

void diag_error(Diag *d, Pos pos, const char *fmt, ...) PRINTF_LIKE(3, 4);
void diag_verror(Diag *d, Pos pos, const char *fmt, va_list ap) VPRINTF_LIKE(3);

size_t diag_count(const Diag *d);

/* Appends every diagnostic to out, one per line: "file:line:col: error: msg", or "error: msg"
 * when it has no position. */
void diag_render(const Diag *d, Buf *out);

#endif
