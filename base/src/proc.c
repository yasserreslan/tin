#include "base/proc.h"

#include <errno.h>
#include <fcntl.h>
#include <pthread.h>
#include <sys/wait.h>
#include <unistd.h>

static void close_fd(int *fd)
{
	if (*fd >= 0) {
		close(*fd);
		*fd = -1;
	}
}

/* Reads fd to end of file, appending to out when it is not NULL. */
static void drain(int fd, Buf *out)
{
	char chunk[4096];
	for (;;) {
		ssize_t r = read(fd, chunk, sizeof chunk);
		if (r < 0 && errno == EINTR)
			continue;
		if (r <= 0)
			return;
		if (out != NULL)
			buf_write(out, chunk, (size_t)r);
	}
}

bool proc_run(char *const argv[], Buf *output, ProcStatus *st)
{
	*st = (ProcStatus){0};
	int out_pipe[2] = {-1, -1}; /* the child's stdout and stderr */
	/* Carries errno back if exec fails; a successful exec closes it (FD_CLOEXEC) first. */
	int exec_pipe[2] = {-1, -1};

	if (output != NULL && pipe(out_pipe) != 0)
		return false;
	if (pipe(exec_pipe) != 0 || fcntl(exec_pipe[1], F_SETFD, FD_CLOEXEC) != 0) {
		close_fd(&out_pipe[0]);
		close_fd(&out_pipe[1]);
		close_fd(&exec_pipe[0]);
		close_fd(&exec_pipe[1]);
		return false;
	}

	pid_t pid = fork();
	if (pid < 0) {
		close_fd(&out_pipe[0]);
		close_fd(&out_pipe[1]);
		close_fd(&exec_pipe[0]);
		close_fd(&exec_pipe[1]);
		return false;
	}
	if (pid == 0) {
		close(exec_pipe[0]);
		if (output != NULL) {
			close(out_pipe[0]);
			dup2(out_pipe[1], STDOUT_FILENO);
			dup2(out_pipe[1], STDERR_FILENO);
			close(out_pipe[1]);
		}
		execvp(argv[0], argv);
		int e = errno;
		ssize_t ignored = write(exec_pipe[1], &e, sizeof e);
		(void)ignored;
		_exit(127);
	}

	close_fd(&out_pipe[1]);
	close_fd(&exec_pipe[1]);
	if (output != NULL)
		drain(out_pipe[0], output);
	close_fd(&out_pipe[0]);

	int exec_errno = 0;
	ssize_t got;
	do {
		got = read(exec_pipe[0], &exec_errno, sizeof exec_errno);
	} while (got < 0 && errno == EINTR);
	close_fd(&exec_pipe[0]);

	int status = 0;
	while (waitpid(pid, &status, 0) < 0) {
		if (errno != EINTR)
			return false;
	}
	if (got == (ssize_t)sizeof exec_errno) {
		st->exec_errno = exec_errno ? exec_errno : ENOENT;
		return true;
	}
	if (WIFEXITED(status)) {
		st->exited = true;
		st->exit_code = WEXITSTATUS(status);
	} else if (WIFSIGNALED(status)) {
		st->signal = WTERMSIG(status);
	}
	return true;
}

typedef struct {
	int (*fn)(void *);
	void *arg;
	int result;
} StackJob;

static void *stack_main(void *p)
{
	StackJob *job = p;
	job->result = job->fn(job->arg);
	return NULL;
}

bool run_with_stack(size_t stack_bytes, int (*fn)(void *), void *arg, int *result)
{
	pthread_attr_t attr;
	if (pthread_attr_init(&attr) != 0)
		return false;
	StackJob job = {fn, arg, 0};
	pthread_t thread;
	bool ok = pthread_attr_setstacksize(&attr, stack_bytes) == 0 &&
		  pthread_create(&thread, &attr, stack_main, &job) == 0;
	pthread_attr_destroy(&attr);
	if (!ok)
		return false;
	pthread_join(thread, NULL);
	*result = job.result;
	return true;
}
