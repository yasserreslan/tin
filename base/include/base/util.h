/* Compiler attributes, allocation-failure policy and small checked helpers shared by every
 * package. Nothing here allocates. */
#ifndef BASE_UTIL_H
#define BASE_UTIL_H

#include <stddef.h>

#if defined(__GNUC__) || defined(__clang__)
#define PRINTF_LIKE(fmt_index, first_arg) __attribute__((format(printf, fmt_index, first_arg)))
/* For a function that takes a va_list instead of "...": the format is argument fmt_index. */
#define VPRINTF_LIKE(fmt_index) __attribute__((format(printf, fmt_index, 0)))
#else
#define PRINTF_LIKE(fmt_index, first_arg)
#define VPRINTF_LIKE(fmt_index)
#endif

/* Allocation policy: functions in this tree never return NULL for out-of-memory. They call
 * oom(), which prints a message and aborts, so a compiler that runs out of memory fails loudly
 * (a crash) and is never mistaken for one that rejected its input. */
_Noreturn void oom(void);

/* a + b and a * b, calling oom() when the result does not fit in a size_t. */
size_t size_add(size_t a, size_t b);
size_t size_mul(size_t a, size_t b);

/* Writes the text of errno value err to out (n bytes, always NUL-terminated), the way Go prints
 * it: the first letter lowercased ("no such file or directory"). */
void errno_text(int err, char *out, size_t n);

#endif
