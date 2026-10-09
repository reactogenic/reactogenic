// For TestTimeoutClock (isolation_test.go), on macOS: a wall clock that
// steps a minute forward every time it is read — what a machine's sleep, or
// a clock that is set, does to a process: the wall clock moves, and the time
// the process ran does not. Loaded with DYLD_INSERT_LIBRARIES.
#include <time.h>

static long reads;

static int stepping(clockid_t clock, struct timespec *now) {
  int result = clock_gettime(clock, now);
  if (clock == CLOCK_REALTIME) now->tv_sec += 60 * ++reads;
  return result;
}

__attribute__((used, section("__DATA,__interpose"))) static struct {
  const void *with, *replaced;
} interpose[] = {{(const void *)stepping, (const void *)clock_gettime}};
