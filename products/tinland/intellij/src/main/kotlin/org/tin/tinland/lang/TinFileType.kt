package org.tin.tinland.lang

import com.intellij.icons.AllIcons
import com.intellij.openapi.fileTypes.LanguageFileType
import javax.swing.Icon

/** The `.tin` file type. */
object TinFileType : LanguageFileType(TinLanguage) {
    override fun getName(): String = "Tin"
    override fun getDescription(): String = "Tin source file"
    override fun getDefaultExtension(): String = "tin"
    override fun getIcon(): Icon = AllIcons.FileTypes.Any_type
}
