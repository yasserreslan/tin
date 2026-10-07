import org.jetbrains.intellij.platform.gradle.TestFrameworkType

plugins {
    id("java")
    id("org.jetbrains.kotlin.jvm") version "2.0.21"
    id("org.jetbrains.intellij.platform") version "2.2.1"
}

group = "org.tin"
version = providers.fileContents(layout.projectDirectory.file("../../../VERSION")).asText.map { it.trim() }.get()

repositories {
    mavenCentral()
    intellijPlatform {
        defaultRepositories()
    }
}

dependencies {
    intellijPlatform {
        intellijIdeaCommunity("2024.3")
        instrumentationTools()
        testFramework(TestFrameworkType.Platform)
    }
    testImplementation("junit:junit:4.13.2")
}

kotlin {
    jvmToolchain(17)
}

intellijPlatform {
    pluginConfiguration {
        id = "org.tin.tinland"
        name = "Tinland"
        version = project.version.toString()
        description = "Tin language support for IntelliJ IDEA: syntax highlighting, a file type and the compiler's diagnostics. Built on the IntelliJ Platform."
        vendor {
            name = "Tin"
        }
        ideaVersion {
            sinceBuild = "243"
        }
    }
}

tasks {
    test {
        useJUnit()
    }
}
