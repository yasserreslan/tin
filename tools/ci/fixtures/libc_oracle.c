/* The C library's answers the Tin runtime must match exactly: strerror, gmtime_r with strftime, gethostname, realpath and
   isatty. tools/ci/helpers_check.tin and compiler_paths_check.tin compile this with cc and compare. */
#define _GNU_SOURCE
#include <fcntl.h>
#include <limits.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <time.h>
#include <unistd.h>

static void quoted(const char *s) {
	putchar('"');
	for (; *s; s++) {
		switch (*s) {
		case '"': fputs("\\\"", stdout); break;
		case '\\': fputs("\\\\", stdout); break;
		case '\n': fputs("\\n", stdout); break;
		case '\r': fputs("\\r", stdout); break;
		case '\t': fputs("\\t", stdout); break;
		default:
			if ((unsigned char)*s < 32) printf("\\u%04x", *s); else putchar(*s);
		}
	}
	putchar('"');
}

static int open_fds(int *fds) {
	int master = posix_openpt(O_RDWR | O_NOCTTY);
	if (master < 0 || grantpt(master) < 0 || unlockpt(master) < 0) return -1;
	char name[256];
	if (ptsname_r(master, name, sizeof name) != 0) return -1;
	int slave = open(name, O_RDWR | O_NOCTTY);
	int p[2];
	if (slave < 0 || pipe(p) < 0) return -1;
	FILE *f = tmpfile();
	if (!f) return -1;
	fds[0] = slave; fds[1] = master; fds[2] = p[0]; fds[3] = p[1]; fds[4] = fileno(f);
	for (int i = 0; i < 5; i++) fcntl(fds[i], F_SETFD, 0);
	return 0;
}

int main(int argc, char **argv) {
	if (argc < 2) return 2;
	if (!strcmp(argv[1], "errors")) {
		for (int n = -2; n < 140; n++) {
			printf("%d ", n);
			quoted(strerror(n));
			putchar('\n');
		}
	} else if (!strcmp(argv[1], "calendar")) {
		long long seconds;
		while (scanf("%lld", &seconds) == 1) {
			time_t t = (time_t)seconds;
			struct tm tm;
			char buf[128];
			if (!gmtime_r(&t, &tm)) return 1;
			if (!strftime(buf, sizeof buf, "Date: %a, %d %b %Y %H:%M:%S GMT\r\n", &tm)) return 1;
			printf("%d %d %d %d %d %d %d ", tm.tm_year + 1900, tm.tm_mon + 1, tm.tm_mday, tm.tm_hour, tm.tm_min, tm.tm_sec, tm.tm_wday);
			quoted(buf);
			putchar('\n');
		}
	} else if (!strcmp(argv[1], "hostname")) {
		char buf[256];
		if (gethostname(buf, sizeof buf)) return 1;
		quoted(buf);
		putchar('\n');
	} else if (!strcmp(argv[1], "realpath")) {
		char cwd[PATH_MAX], full[2 * PATH_MAX], out[PATH_MAX];
		if (!getcwd(cwd, sizeof cwd)) return 1;
		for (int i = 2; i < argc; i++) {
			if (argv[i][0] == 0) full[0] = 0;
			else if (argv[i][0] == '/') snprintf(full, sizeof full, "%s", argv[i]);
			else snprintf(full, sizeof full, "%s/%s", cwd, argv[i]);
			puts(realpath(full, out) ? out : "<nil>");
		}
	} else if (!strcmp(argv[1], "tty") || !strcmp(argv[1], "tty-run")) {
		int fds[6];
		if (open_fds(fds)) return 1;
		fds[5] = -1;
		if (!strcmp(argv[1], "tty")) {
			for (int i = 0; i < 6; i++) puts(isatty(fds[i]) ? "true" : "false");
		} else {
			char *args[10];
			char numbers[6][16];
			args[0] = argv[2];
			args[1] = "tty";
			for (int i = 0; i < 6; i++) {
				snprintf(numbers[i], sizeof numbers[i], "%d", fds[i]);
				args[2 + i] = numbers[i];
			}
			args[8] = NULL;
			execv(argv[2], args);
			return 1;
		}
	} else {
		return 2;
	}
	return 0;
}
