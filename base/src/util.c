#include "base/util.h"

#include <ctype.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

void oom(void)
{
	fputs("fatal: out of memory\n", stderr);
	abort();
}

size_t size_add(size_t a, size_t b)
{
	if (a > SIZE_MAX - b)
		oom();
	return a + b;
}

size_t size_mul(size_t a, size_t b)
{
	if (b != 0 && a > SIZE_MAX / b)
		oom();
	return a * b;
}

void errno_text(int err, char *out, size_t n)
{
	if (n == 0)
		return;
	snprintf(out, n, "%s", strerror(err));
	if (out[0] != '\0')
		out[0] = (char)tolower((unsigned char)out[0]);
}
