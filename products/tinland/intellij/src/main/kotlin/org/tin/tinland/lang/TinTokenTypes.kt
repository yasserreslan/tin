package org.tin.tinland.lang

import com.intellij.psi.tree.IElementType

/** The token types the Tin lexer produces. Names follow the compiler's lexer (toolchain/compiler/lex.tin). */
object TinTokenTypes {
    val KEYWORD = IElementType("KEYWORD", TinLanguage)
    val CONSTANT = IElementType("CONSTANT", TinLanguage)
    val IDENTIFIER = IElementType("IDENTIFIER", TinLanguage)
    val NUMBER = IElementType("NUMBER", TinLanguage)
    val STRING = IElementType("STRING", TinLanguage)
    val RAW_STRING = IElementType("RAW_STRING", TinLanguage)
    val CHAR = IElementType("CHAR", TinLanguage)
    val COMMENT = IElementType("COMMENT", TinLanguage)
    val OPERATOR = IElementType("OPERATOR", TinLanguage)
    val PUNCTUATION = IElementType("PUNCTUATION", TinLanguage)
    val BAD_CHARACTER = IElementType("BAD_CHARACTER", TinLanguage)
}
