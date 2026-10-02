/* Files and temporary directories. Failures are reported as text in a Buf, worded like Go's
 * ("open x.tin: no such file or directory"), so tools print the same diagnostics as before. */
#ifndef BASE_FS_H
#define BASE_FS_H

#include <stdbool.h>

#include "base/buf.h"

/* Appends the whole contents of path to out. On failure appends the reason to err and returns
 * false. */
bool fs_read_file(const char *path, Buf *out, Buf *err);

/* Writes data to path (created or truncated, mode 0644). */
bool fs_write_file(const char *path, const void *data, size_t n, Buf *err);

/* Creates a private directory under $TMPDIR (or /tmp) named <prefix>XXXXXX and stores its path
 * in out. */
bool fs_tmpdir_make(const char *prefix, Buf *out, Buf *err);

/* Removes the files directly inside dir, then dir. It does not descend into subdirectories. */
void fs_tmpdir_remove(const char *dir);

#endif
