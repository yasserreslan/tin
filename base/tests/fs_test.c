#include <stdlib.h>
#include <string.h>
#include <sys/stat.h>
#include <unistd.h>

#include "base/fs.h"
#include "testkit/testkit.h"

static void tmpdir_roundtrip(void)
{
	Buf dir = {0}, err = {0};
	CHECK(fs_tmpdir_make("fstest-", &dir, &err));
	CHECK(strstr(dir.p, "fstest-") != NULL);

	Buf path = {0};
	buf_printf(&path, "%s/data.bin", dir.p);
	const char data[] = {'a', 0, 'b', '\n', (char)0xFF};
	CHECK(fs_write_file(path.p, data, sizeof data, &err));

	Buf got = {0};
	CHECK(fs_read_file(path.p, &got, &err));
	CHECK_EQ_INT(got.n, sizeof data);
	CHECK(memcmp(got.p, data, sizeof data) == 0);

	fs_tmpdir_remove(dir.p);
	struct stat st;
	CHECK(stat(dir.p, &st) != 0);

	buf_free(&got);
	buf_free(&path);
	buf_free(&dir);
	buf_free(&err);
}

static void reading_a_missing_file_says_so_like_go(void)
{
	Buf out = {0}, err = {0};
	CHECK(!fs_read_file("/nonexistent/dir/x.tin", &out, &err));
	CHECK_EQ_STR(err.p, "open /nonexistent/dir/x.tin: no such file or directory");
	buf_free(&out);
	buf_free(&err);
}

static void reading_a_directory_fails(void)
{
	Buf out = {0}, err = {0};
	CHECK(!fs_read_file("/", &out, &err));
	CHECK_EQ_STR(err.p, "read /: is a directory");
	buf_free(&out);
	buf_free(&err);
}

static void an_empty_file_reads_as_a_valid_empty_buf(void)
{
	Buf dir = {0}, err = {0}, path = {0}, got = {0};
	CHECK(fs_tmpdir_make("fstest-", &dir, &err));
	buf_printf(&path, "%s/empty", dir.p);
	CHECK(fs_write_file(path.p, "", 0, &err));
	CHECK(fs_read_file(path.p, &got, &err));
	CHECK_EQ_INT(got.n, 0);
	CHECK(got.p != NULL);
	fs_tmpdir_remove(dir.p);
	buf_free(&dir);
	buf_free(&err);
	buf_free(&path);
	buf_free(&got);
}

static void a_large_file_reads_completely(void)
{
	Buf dir = {0}, err = {0}, path = {0}, src = {0}, got = {0};
	CHECK(fs_tmpdir_make("fstest-", &dir, &err));
	buf_printf(&path, "%s/big", dir.p);
	for (int i = 0; i < 100000; i++)
		buf_printf(&src, "%d\n", i);
	CHECK(fs_write_file(path.p, src.p, src.n, &err));
	CHECK(fs_read_file(path.p, &got, &err));
	CHECK_EQ_INT(got.n, src.n);
	CHECK(memcmp(got.p, src.p, src.n) == 0);
	fs_tmpdir_remove(dir.p);
	buf_free(&dir);
	buf_free(&err);
	buf_free(&path);
	buf_free(&src);
	buf_free(&got);
}

static void writing_into_a_missing_directory_fails(void)
{
	Buf err = {0};
	CHECK(!fs_write_file("/nonexistent/dir/out", "x", 1, &err));
	CHECK_EQ_STR(err.p, "open /nonexistent/dir/out: no such file or directory");
	buf_free(&err);
}

static void tmpdir_honors_tmpdir_and_strips_trailing_slashes(void)
{
	Buf base = {0}, err = {0}, dir = {0};
	CHECK(fs_tmpdir_make("fstest-", &base, &err));
	Buf with_slash = {0};
	buf_printf(&with_slash, "%s//", base.p);
	setenv("TMPDIR", with_slash.p, 1);
	CHECK(fs_tmpdir_make("inner-", &dir, &err));
	CHECK(strncmp(dir.p, base.p, base.n) == 0);
	CHECK_EQ_INT(dir.p[base.n], '/');
	CHECK(dir.p[base.n + 1] != '/');
	unsetenv("TMPDIR");
	fs_tmpdir_remove(dir.p);
	fs_tmpdir_remove(base.p);
	buf_free(&base);
	buf_free(&err);
	buf_free(&dir);
	buf_free(&with_slash);
}

static const TestCase cases[] = {
	TEST_CASE(tmpdir_roundtrip),
	TEST_CASE(reading_a_missing_file_says_so_like_go),
	TEST_CASE(reading_a_directory_fails),
	TEST_CASE(an_empty_file_reads_as_a_valid_empty_buf),
	TEST_CASE(a_large_file_reads_completely),
	TEST_CASE(writing_into_a_missing_directory_fails),
	TEST_CASE(tmpdir_honors_tmpdir_and_strips_trailing_slashes),
};

TESTKIT_MAIN("base/fs", cases)
