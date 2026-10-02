# Design: `dyn` and multi-word values

This note is the implementation design for the `dyn` sub-item of #141 (notes/roadmap.md,
"`dyn` fat reference (data pointer plus table) for open sets, with region rules"). The
decision that `dyn S` exists and what it means is in notes/design_foundations.md section 2;
this note fixes the representation the compiler will build, because the acceptance test —
`[]dyn Writer` of different types, conversion allocates nothing, and storing a `dyn` of
request memory in a global is a region error — cannot be met by any one-word encoding.

## 1. Why two words, and why not a box

A `dyn S` value is an object plus the method table for the shape `S` at that object's type.
The table depends on the pair (concrete type, shape) and is static; the object is a runtime
value. A one-word representation would have to be a pointer to a record holding both, which
means an allocation at every conversion (or a per-object header, which changes the layout of
every struct and every existing value in the language). Both were rejected in
design_foundations section 2; the acceptance test ("conversion allocates nothing") rejects
the first outright.

The representation is therefore a **two-word value**: `[object, table]`, where

- `object` is the concrete value as it is today (a struct is already a reference, so one
  word; a `dyn` of a slice, map, func or str carries that reference);
- `table` is the address of a static table in the constant segment, one per (concrete type,
  shape) pair, emitted where the conversion happens and deduplicated per program.

A `dyn S` is never nil; `?dyn S` is the same two words with `object == 0` meaning absent, so
the existing nil check (`object == 0`) works unchanged. There is no downcast and no runtime
type information beyond the table: the table holds only the method addresses the shape names,
in the shape's declaration order.

## 2. Where two-word values appear

`dyn` is the first two-word type; the same machinery is what value arrays (`[N]T` with copy
semantics), `complex128` and SIMD vectors need, so it is built as a general "value of size
16" and not as a `dyn` special case:

| context | today (one word) | with two words |
|---|---|---|
| local | one home slot | two adjacent slots; a local of size 16 occupies both |
| parameter / result | one register (x0..x7, v0..v7) | a pair of registers; a 16-byte argument that does not fit goes on the stack in two slots |
| struct field | 8-byte slot | 16-byte slot, 8-byte aligned; `layout_struct` already computes offsets, the size class is new |
| slice element | scale 1/2/4/8 in the load/store | two loads/stores at +0 and +8; `slice_of` records the element size 16 |
| map value | one word | two words in the value slot |
| `?dyn` | nil check | `object == 0` |

Scalar sizes stay as they are; the compiler's rule "every value is one word" becomes "every
value is one or two words, and a type's `TY_SIZE` decides".

## 3. Conversions and calls

- A conversion from a concrete type `T` to `dyn S` is checked where it is written (the
  existing structural satisfaction check, with `dyn` allowed in parameters, results, fields,
  slice elements and map values). It lowers to a pair: the object unchanged, the table's
  address materialized as a constant.
- A method call on a `dyn S` value loads the method address from the table at the method's
  index (the shape's declaration order is the table order, so the index is a compile-time
  constant) and calls it indirectly with the object as the receiver. No lookup by name at run
  time, no hashing, no allocation.
- The table is emitted in the constant segment with one relocation per method. `macho.tin`,
  `elf.tin` and `elf_x64.tin` already emit data with relocations for other tables (the string
  table, function descriptors), so this reuses their paths.
- A `dyn` value is not comparable with `==` (as `func` values are not); identity is a method
  on the shape (`Kind()`), as design_foundations section 2 requires.

## 4. Regions

The region bits of a `dyn S` value are those of its `object`; the table is static and carries
no region. Storing a `dyn` built from request memory into a global, a map reachable from one,
or a `keep`ed structure is therefore an error exactly as storing the object itself is, with
the same message and `keep` suggestion. `keep(dyn)` deep-copies the object (and copies the
table pointer, which is static). `region.tin` needs one new case: a two-word value's bits come
from the value expression's bits, and a conversion is a use of the object.

## 5. Staging

Each step keeps `make bootstrap` a fixed point and adds its own tests:

1. **This note** and the representation decision (no compiler change).
2. **Size-16 values in the type system**: `TY_SIZE` of a `dyn` type, `K_DYN`, interning by
   (shape, bindings), `[]dyn`/`?dyn`, conversion checking and region bits, with code
   generation still refusing to emit a program that uses one (a clear diagnostic, so the
   checker cannot accept a program the backend cannot build). Negative tests only.
3. **Two-word values in lowering and codegen on arm64**: homes, parameters, results, struct
   fields and method calls through a table; positive tests that build and run.
4. **The amd64 backend** and `[]dyn`/map values; cross-target tests.
5. **The region and lifetime tests**, the `dyn` twin, and the acceptance test of the box.
6. **The library ports that need `dyn`** (`io.ReaderFrom`/`WriterTo`, `MultiReader`,
   `database/sql/driver` shapes), which is the next sub-item.

## 6. Rejected alternatives

- **Boxed one-word record**: allocates at every conversion, adds an indirection to every
  method call, and contradicts the acceptance test. Rejected.
- **Per-object type header**: makes every struct two words (or every reference carry a
  header), changes the layout and cost of all existing code, and leaks runtime type
  information into values that do not need it. Rejected.
- **Table chosen by the call site**: impossible for an open set; the concrete type is not
  known where the method is called. Rejected.
- **One table per shape with an index per type**: a global type registry needs runtime type
  information and a lock-free lookup; the per-pair static table has neither. Rejected.

## 7. What this unlocks

The same size-16 value machinery is what value arrays (`[N]T` with copy semantics, for UUIDs
and hashes as map keys), `complex64`/`complex128` and SIMD vector types need. Building `dyn`
first gives the language one reviewed representation for "more than one word" instead of three
ad-hoc ones.
