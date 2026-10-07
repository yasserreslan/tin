package org.tin.tinland.lang

/**
 * The reserved words of edition 1 (toolchain/docs/LANGUAGE.md, "Reserved words"). The contextual words
 * (in, max, on, use, with, once, select, scope, arena, shape, dyn, wrap) are names everywhere except where a
 * construct starts, so the lexer leaves them as identifiers.
 */
object TinKeywords {
    val RESERVED: Set<String> = setOf(
        "break", "case", "catch", "const", "continue", "default", "defer", "detach", "else", "enum",
        "extern", "fail", "fn", "for", "go", "guard", "if", "import", "keep", "let", "limit", "map",
        "match", "mut", "package", "parallel", "range", "return", "secret", "shared", "struct", "try",
        "type", "within",
        // Reserved so that the edition 0 forms produce their diagnostic.
        "func", "var", "switch", "while",
    )
    val CONSTANTS: Set<String> = setOf("true", "false", "nil")
}
