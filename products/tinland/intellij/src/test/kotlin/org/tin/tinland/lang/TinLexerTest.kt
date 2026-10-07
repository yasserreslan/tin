package org.tin.tinland.lang

import com.intellij.lexer.Lexer
import com.intellij.psi.TokenType
import com.intellij.psi.tree.IElementType
import org.junit.Assert.assertEquals
import org.junit.Test

class TinLexerTest {
    private fun tokens(text: String): List<Pair<IElementType, String>> {
        val lexer: Lexer = TinLexer()
        lexer.start(text)
        val out = mutableListOf<Pair<IElementType, String>>()
        while (lexer.tokenType != null) {
            val type = lexer.tokenType!!
            if (type != TokenType.WHITE_SPACE) out += type to text.substring(lexer.tokenStart, lexer.tokenEnd)
            lexer.advance()
        }
        return out
    }

    @Test
    fun declarationAndCall() {
        assertEquals(
            listOf(
                TinTokenTypes.KEYWORD to "fn",
                TinTokenTypes.IDENTIFIER to "main",
                TinTokenTypes.PUNCTUATION to "(",
                TinTokenTypes.PUNCTUATION to ")",
                TinTokenTypes.PUNCTUATION to "{",
                TinTokenTypes.KEYWORD to "let",
                TinTokenTypes.IDENTIFIER to "x",
                TinTokenTypes.OPERATOR to ":=",
                TinTokenTypes.NUMBER to "0x2a",
                TinTokenTypes.PUNCTUATION to "}",
            ),
            tokens("fn main() {\n\tlet x := 0x2a\n}"),
        )
    }

    @Test
    fun literalsAndComments() {
        assertEquals(
            listOf(
                TinTokenTypes.CONSTANT to "true",
                TinTokenTypes.STRING to "\"a \\\" b\"",
                TinTokenTypes.NUMBER to "2.5",
                TinTokenTypes.NUMBER to "200ms",
                TinTokenTypes.RAW_STRING to "`raw`",
                TinTokenTypes.CHAR to "'x'",
                TinTokenTypes.COMMENT to "// note",
            ),
            tokens("true \"a \\\" b\" 2.5 200ms `raw` 'x' // note"),
        )
    }

    @Test
    fun contextualWordsAreNames() {
        assertEquals(
            listOf(TinTokenTypes.IDENTIFIER to "in", TinTokenTypes.IDENTIFIER to "scope"),
            tokens("in scope"),
        )
    }

    @Test
    fun rangeIsAnOperatorNotANumber() {
        assertEquals(
            listOf(TinTokenTypes.NUMBER to "1", TinTokenTypes.OPERATOR to "..", TinTokenTypes.NUMBER to "3"),
            tokens("1..3"),
        )
    }

    @Test
    fun unterminatedStringEndsAtLineBreak() {
        assertEquals(
            listOf(TinTokenTypes.STRING to "\"open", TinTokenTypes.IDENTIFIER to "x"),
            tokens("\"open\nx"),
        )
    }
}
