plugins {
    id("com.android.application") version "9.1.1" apply false
    // AGP 9.0+ ships built-in Kotlin; only the Compose compiler plugin remains explicit.
    id("org.jetbrains.kotlin.plugin.compose") version "2.2.10" apply false
}
