# Native packages: code built with the host C toolchain (see docs/NATIVE.md).
#
# A package is a top-level directory with a package.mk and this shape:
#
#   include/<pkg>/*.h    public headers, included as "<pkg>/name.h"
#   src/*.c              the library, built into $(NATIVE_OUT)/lib<pkg>.a
#   cmd/<prog>/*.c       one program per directory, linked with the package and its dependencies
#   tests/*_test.c       one test program per file, linked with testkit
#
# package.mk sets <pkg>_DEPS (packages it links against) and <pkg>_PROGRAMS (names under cmd/).
# Adding a package is adding that directory; nothing in this file or the Makefile changes.
#
#   make native          build every library and program (programs land in bin/)
#   make native-test     build and run every test program
#   make native-asan     the tests again under AddressSanitizer and UBSan
#   make native-format   apply .clang-format to every native source
#   NATIVE_WERROR=-Werror  on any of these makes warnings errors (CI does)
#   make native-clean    remove the native build output

CC ?= cc
NATIVE_CC ?= $(CC)
NATIVE_OUT ?= bin/native
NATIVE_BIN ?= bin
NATIVE_OPT ?= -O2 -g
NATIVE_SAN ?=
# POSIX 2008 on every host; _DEFAULT_SOURCE and _DARWIN_C_SOURCE keep glibc and macOS from hiding it.
NATIVE_STD = -std=c11 -D_POSIX_C_SOURCE=200809L -D_DEFAULT_SOURCE -D_DARWIN_C_SOURCE
NATIVE_WARN ?= -Wall -Wextra -Wpedantic -Wshadow -Wstrict-prototypes -Wmissing-prototypes \
               -Wvla -Wformat=2 -Wundef
# CI passes NATIVE_WERROR=-Werror; a newer compiler's new warning should not break a user's build.
NATIVE_WERROR ?=
NATIVE_INCLUDES = $(addprefix -I,$(wildcard */include))
NATIVE_CFLAGS = $(NATIVE_STD) $(NATIVE_OPT) $(NATIVE_WARN) $(NATIVE_WERROR) $(NATIVE_SAN) $(NATIVE_INCLUDES)
NATIVE_LDFLAGS = $(NATIVE_SAN) -pthread

NATIVE_PKGS := $(sort $(patsubst %/package.mk,%,$(wildcard */package.mk)))
include $(wildcard */package.mk)

# The packages p depends on, directly and indirectly (p itself not included).
native_deps = $(foreach d,$($(1)_DEPS),$(d) $(call native_deps,$(d)))
# The archives a program or test of package p links, dependents before dependencies.
native_libs = $(foreach l,$(1) $(call native_deps,$(1)),$(NATIVE_OUT)/lib$(l).a)

define native_package
$(1)_SRCS := $$(wildcard $(1)/src/*.c)
$(1)_OBJS := $$(patsubst $(1)/src/%.c,$$(NATIVE_OUT)/obj/$(1)/%.o,$$($(1)_SRCS))
$(1)_TESTBINS := $$(patsubst $(1)/tests/%.c,$$(NATIVE_OUT)/test/$(1)/%,$$(wildcard $(1)/tests/*_test.c))
NATIVE_LIBS += $$(NATIVE_OUT)/lib$(1).a
NATIVE_TESTBINS += $$($(1)_TESTBINS)

$$(NATIVE_OUT)/obj/$(1)/%.o: $(1)/src/%.c
	@mkdir -p $$(dir $$@)
	$$(NATIVE_CC) $$(NATIVE_CFLAGS) -MMD -MP -c -o $$@ $$<

$$(NATIVE_OUT)/lib$(1).a: $$($(1)_OBJS)
	@rm -f $$@
	$$(AR) rcs $$@ $$^

$$(NATIVE_OUT)/test/$(1)/%: $(1)/tests/%.c $$(call native_libs,$(1)) $$(NATIVE_OUT)/libtestkit.a
	@mkdir -p $$(dir $$@)
	$$(NATIVE_CC) $$(NATIVE_CFLAGS) -MMD -MP -MF $$@.d -o $$@ $$< \
		$$(call native_libs,$(1)) $$(NATIVE_OUT)/libtestkit.a $$(NATIVE_LDFLAGS)

$(foreach p,$($(1)_PROGRAMS),$(call native_program,$(1),$(p)))
endef

# native_program(pkg, prog): bin/<prog> from <pkg>/cmd/<prog>/*.c.
define native_program
$(1)_$(2)_OBJS := $$(patsubst $(1)/cmd/$(2)/%.c,$$(NATIVE_OUT)/obj/$(1)/cmd/$(2)/%.o,$$(wildcard $(1)/cmd/$(2)/*.c))
NATIVE_PROGS += $$(NATIVE_BIN)/$(2)

$$(NATIVE_OUT)/obj/$(1)/cmd/$(2)/%.o: $(1)/cmd/$(2)/%.c
	@mkdir -p $$(dir $$@)
	$$(NATIVE_CC) $$(NATIVE_CFLAGS) -MMD -MP -c -o $$@ $$<

$$(NATIVE_BIN)/$(2): $$($(1)_$(2)_OBJS) $$(call native_libs,$(1))
	@mkdir -p $$(dir $$@)
	$$(NATIVE_CC) -o $$@ $$^ $$(NATIVE_LDFLAGS)
endef

$(foreach p,$(NATIVE_PKGS),$(eval $(call native_package,$(p))))

.PHONY: native native-test native-asan native-clean native-format

native: $(NATIVE_LIBS) $(NATIVE_PROGS)

native-test: $(NATIVE_TESTBINS)
	@status=0; for t in $(NATIVE_TESTBINS); do $$t || status=1; done; exit $$status

native-asan:
	$(MAKE) NATIVE_OUT=bin/native-asan NATIVE_BIN=bin/native-asan NATIVE_OPT="-O1 -g" \
		NATIVE_SAN="-fsanitize=address,undefined -fno-sanitize-recover=undefined -fno-omit-frame-pointer" \
		native-test

NATIVE_SOURCES = $(foreach p,$(NATIVE_PKGS),$(shell find $(p) \( -name '*.c' -o -name '*.h' \)))

native-format:
	clang-format -i $(NATIVE_SOURCES)

native-clean:
	rm -rf bin/native bin/native-asan $(NATIVE_PROGS)

-include $(shell find $(NATIVE_OUT) -name '*.d' 2>/dev/null)
