# testkit

The unit-test framework for the native packages. Each `<pkg>/tests/<name>_test.c` is its own
program: a list of cases and `TESTKIT_MAIN`. See the header comment in
`include/testkit/testkit.h` for the macros, and [docs/NATIVE.md](../docs/NATIVE.md) for what a
test should cover.

```c
static void adds(void) { CHECK_EQ_INT(1 + 1, 2); }
static const TestCase cases[] = { TEST_CASE(adds) };
TESTKIT_MAIN("math", cases)
```

`TESTKIT_VERBOSE=1` lists passing cases too.
