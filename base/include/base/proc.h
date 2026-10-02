/* Child processes and big-stack execution. */
#ifndef BASE_PROC_H
#define BASE_PROC_H

#include <stdbool.h>
#include <stddef.h>

#include "base/buf.h"

typedef struct {
	bool exited; /* the process ran and exited normally; exit_code is valid */
	int exit_code;
	int signal;	/* nonzero when a signal ended it */
	int exec_errno; /* nonzero when the program could not be started (errno from execvp) */
} ProcStatus;

/* Runs argv[0] (searched in PATH) with argv, waits for it and fills *st.
 *
 * If output is not NULL, the child's stdout and stderr are both captured into it; otherwise the
 * child inherits ours. Returns false only when the machinery itself failed (pipe, fork, wait);
 * a program that cannot be started is a true return with st->exec_errno set. */
bool proc_run(char *const argv[], Buf *output, ProcStatus *st);

/* Calls fn(arg) on a new thread with a stack of stack_bytes and stores its result in *result.
 * Deep recursion (long expression chains, nested blocks) needs more than the default stack.
 * Returns false when the thread could not be created; the caller can then call fn itself. */
bool run_with_stack(size_t stack_bytes, int (*fn)(void *), void *arg, int *result);

#endif
