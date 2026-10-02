#include "base/fs.h"

#include <dirent.h>
#include <errno.h>
#include <fcntl.h>
#include <stdlib.h>
#include <string.h>
#include <sys/stat.h>
#include <unistd.h>

/* Appends "<what> <path>: <reason>" to err, as Go words its file errors. */
static void fail(Buf *err, const char *what, const char *path, int e)
{
	char text[256];
	errno_text(e, text, sizeof text);
	buf_printf(err, "%s %s: %s", what, path, text);
}

bool fs_read_file(const char *path, Buf *out, Buf *err)
{
	int fd = open(path, O_RDONLY);
	if (fd < 0) {
		fail(err, "open", path, errno);
		return false;
	}
	struct stat st;
	if (fstat(fd, &st) == 0 && S_ISDIR(st.st_mode)) {
		fail(err, "read", path, EISDIR);
		close(fd);
		return false;
	}
	char chunk[65536];
	for (;;) {
		ssize_t r = read(fd, chunk, sizeof chunk);
		if (r < 0) {
			if (errno == EINTR)
				continue;
			fail(err, "read", path, errno);
			close(fd);
			return false;
		}
		if (r == 0)
			break;
		buf_write(out, chunk, (size_t)r);
	}
	close(fd);
	buf_write(out, "", 0); /* an empty file still leaves a valid, NUL-terminated Buf */
	return true;
}

bool fs_write_file(const char *path, const void *data, size_t n, Buf *err)
{
	int fd = open(path, O_WRONLY | O_CREAT | O_TRUNC, 0644);
	if (fd < 0) {
		fail(err, "open", path, errno);
		return false;
	}
	const char *p = data;
	while (n > 0) {
		ssize_t w = write(fd, p, n);
		if (w < 0) {
			if (errno == EINTR)
				continue;
			fail(err, "write", path, errno);
			close(fd);
			return false;
		}
		p += w;
		n -= (size_t)w;
	}
	if (close(fd) != 0) {
		fail(err, "close", path, errno);
		return false;
	}
	return true;
}

bool fs_tmpdir_make(const char *prefix, Buf *out, Buf *err)
{
	const char *base = getenv("TMPDIR");
	if (base == NULL || base[0] == '\0')
		base = "/tmp";
	size_t len = strlen(base);
	while (len > 1 && base[len - 1] == '/')
		len--;
	Buf tmpl = {0};
	buf_printf(&tmpl, "%.*s/%sXXXXXX", (int)len, base, prefix);
	if (mkdtemp(tmpl.p) == NULL) {
		fail(err, "mkdtemp", buf_cstr(&tmpl), errno);
		buf_free(&tmpl);
		return false;
	}
	buf_puts(out, tmpl.p);
	buf_free(&tmpl);
	return true;
}

void fs_tmpdir_remove(const char *dir)
{
	DIR *d = opendir(dir);
	if (d != NULL) {
		struct dirent *ent;
		while ((ent = readdir(d)) != NULL) {
			if (strcmp(ent->d_name, ".") == 0 || strcmp(ent->d_name, "..") == 0)
				continue;
			Buf path = {0};
			buf_printf(&path, "%s/%s", dir, ent->d_name);
			unlink(path.p);
			buf_free(&path);
		}
		closedir(d);
	}
	rmdir(dir);
}
