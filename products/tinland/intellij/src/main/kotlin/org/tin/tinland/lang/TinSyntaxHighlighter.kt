package org.tin.tinland.lang

import com.intellij.lexer.Lexer
import com.intellij.openapi.editor.DefaultLanguageHighlighterColors
import com.intellij.openapi.editor.HighlighterColors
import com.intellij.openapi.editor.colors.TextAttributesKey
import com.intellij.openapi.editor.colors.TextAttributesKey.createTextAttributesKey
import com.intellij.openapi.fileTypes.SyntaxHighlighterBase
import com.intellij.psi.tree.IElementType

class TinSyntaxHighlighter : SyntaxHighlighterBase() {
    override fun getHighlightingLexer(): Lexer = TinLexer()

    override fun getTokenHighlights(tokenType: IElementType?): Array<TextAttributesKey> = when (tokenType) {
        TinTokenTypes.KEYWORD -> KEYWORD
        TinTokenTypes.CONSTANT -> CONSTANT
        TinTokenTypes.NUMBER -> NUMBER
        TinTokenTypes.STRING, TinTokenTypes.RAW_STRING, TinTokenTypes.CHAR -> STRING
        TinTokenTypes.COMMENT -> COMMENT
        TinTokenTypes.OPERATOR -> OPERATOR
        TinTokenTypes.PUNCTUATION -> PUNCTUATION
        TinTokenTypes.BAD_CHARACTER -> BAD_CHARACTER
        else -> NO_HIGHLIGHT
    }

    companion object {
        private fun attributes(name: String, fallback: TextAttributesKey): Array<TextAttributesKey> =
            arrayOf(createTextAttributesKey("TIN_$name", fallback))

        private val KEYWORD = attributes("KEYWORD", DefaultLanguageHighlighterColors.KEYWORD)
        private val CONSTANT = attributes("CONSTANT", DefaultLanguageHighlighterColors.CONSTANT)
        private val NUMBER = attributes("NUMBER", DefaultLanguageHighlighterColors.NUMBER)
        private val STRING = attributes("STRING", DefaultLanguageHighlighterColors.STRING)
        private val COMMENT = attributes("COMMENT", DefaultLanguageHighlighterColors.LINE_COMMENT)
        private val OPERATOR = attributes("OPERATOR", DefaultLanguageHighlighterColors.OPERATION_SIGN)
        private val PUNCTUATION = attributes("PUNCTUATION", DefaultLanguageHighlighterColors.SEMICOLON)
        private val BAD_CHARACTER = attributes("BAD_CHARACTER", HighlighterColors.BAD_CHARACTER)
        private val NO_HIGHLIGHT = emptyArray<TextAttributesKey>()
    }
}
