package org.tin.tinland.lang

import com.intellij.lexer.LexerBase
import com.intellij.psi.tree.IElementType

/**
 * A lexer for edition 1 source. It classifies tokens for highlighting; the compiler's lexer remains the
 * authority on what is valid (`tinc -tokens` is the contract, see tracking issue #392).
 */
class TinLexer : LexerBase() {
    private var buffer: CharSequence = ""
    private var bufferEnd = 0
    private var tokenStart = 0
    private var tokenEnd = 0
    private var tokenType: IElementType? = null

    override fun start(buffer: CharSequence, startOffset: Int, endOffset: Int, initialState: Int) {
        this.buffer = buffer
        bufferEnd = endOffset
        tokenStart = startOffset
        tokenEnd = startOffset
        tokenType = null
        advance()
    }

    override fun getState(): Int = 0
    override fun getTokenType(): IElementType? = tokenType
    override fun getTokenStart(): Int = tokenStart
    override fun getTokenEnd(): Int = tokenEnd
    override fun getBufferSequence(): CharSequence = buffer
    override fun getBufferEnd(): Int = bufferEnd

    override fun advance() {
        tokenStart = tokenEnd
        if (tokenStart >= bufferEnd) {
            tokenType = null
            return
        }
        val c = buffer[tokenStart]
        tokenType = when {
            c == ' ' || c == '\t' || c == '\n' || c == '\r' -> {
                tokenEnd = scanWhile(tokenStart) { it == ' ' || it == '\t' || it == '\n' || it == '\r' }
                WHITE_SPACE
            }
            c == '/' && peek(1) == '/' -> {
                tokenEnd = scanWhile(tokenStart) { it != '\n' && it != '\r' }
                TinTokenTypes.COMMENT
            }
            c == '"' -> {
                tokenEnd = scanQuoted(tokenStart, '"')
                TinTokenTypes.STRING
            }
            c == '`' -> {
                tokenEnd = scanQuoted(tokenStart, '`', escapes = false)
                TinTokenTypes.RAW_STRING
            }
            c == '\'' -> {
                tokenEnd = scanQuoted(tokenStart, '\'')
                TinTokenTypes.CHAR
            }
            c.isDigit() || (c == '.' && peek(1).isDigit()) -> {
                tokenEnd = scanNumber(tokenStart)
                TinTokenTypes.NUMBER
            }
            c.isLetter() || c == '_' -> {
                tokenEnd = scanWhile(tokenStart) { it.isLetterOrDigit() || it == '_' }
                val word = buffer.subSequence(tokenStart, tokenEnd).toString()
                when {
                    word in TinKeywords.CONSTANTS -> TinTokenTypes.CONSTANT
                    word in TinKeywords.RESERVED -> TinTokenTypes.KEYWORD
                    else -> TinTokenTypes.IDENTIFIER
                }
            }
            c in "()[]{},;" -> {
                tokenEnd = tokenStart + 1
                TinTokenTypes.PUNCTUATION
            }
            c in OPERATOR_CHARS || c == '.' || c == ':' -> {
                val multi = multiCharOperatorLength(tokenStart)
                if (multi > 0) {
                    tokenEnd = tokenStart + multi
                    TinTokenTypes.OPERATOR
                } else {
                    tokenEnd = tokenStart + 1
                    if (c == '.' || c == ':') TinTokenTypes.PUNCTUATION else TinTokenTypes.OPERATOR
                }
            }
            else -> {
                tokenEnd = tokenStart + 1
                TinTokenTypes.BAD_CHARACTER
            }
        }
    }

    private fun peek(offset: Int): Char {
        val i = tokenStart + offset
        return if (i < bufferEnd) buffer[i] else '\u0000'
    }

    private inline fun scanWhile(from: Int, accept: (Char) -> Boolean): Int {
        var i = from
        while (i < bufferEnd && accept(buffer[i])) i++
        return i
    }

    /** A string, rune or raw string; an unterminated one ends at the line break. */
    private fun scanQuoted(from: Int, quote: Char, escapes: Boolean = true): Int {
        var i = from + 1
        while (i < bufferEnd) {
            val c = buffer[i]
            if (c == '\n' || c == '\r') return i
            if (escapes && c == '\\' && i + 1 < bufferEnd) {
                i += 2
                continue
            }
            i++
            if (c == quote) return i
        }
        return i
    }

    /** Digits, letters (for 0x, units such as ms), underscores, and a dot only before a digit. */
    private fun scanNumber(from: Int): Int {
        var i = from
        while (i < bufferEnd) {
            val c = buffer[i]
            val exponentSign = (c == '+' || c == '-') && i > from && (buffer[i - 1] == 'e' || buffer[i - 1] == 'E') &&
                i + 1 < bufferEnd && buffer[i + 1].isDigit()
            when {
                c.isLetterOrDigit() || c == '_' || exponentSign -> i++
                c == '.' && i + 1 < bufferEnd && buffer[i + 1].isDigit() -> i++
                else -> return i
            }
        }
        return i
    }

    /** The length of the multi-character operator starting at from, or 0. */
    private fun multiCharOperatorLength(from: Int): Int {
        for (op in MULTI_CHAR_OPERATORS) {
            if (from + op.length <= bufferEnd && buffer.startsWith(op, from)) return op.length
        }
        return 0
    }

    companion object {
        private val WHITE_SPACE: IElementType = com.intellij.psi.TokenType.WHITE_SPACE
        private const val OPERATOR_CHARS = "+-*/%&|^!~<>=?"
        private val MULTI_CHAR_OPERATORS = listOf(
            "...", ":=", "==", "!=", "<=", ">=", "&&", "||", "<<", ">>", "+=", "-=", "*=", "/=", "%=", "=>",
            "..", "++", "--", "+%", "-%", "*%",
        )
    }
}
