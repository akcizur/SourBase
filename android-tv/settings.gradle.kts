pluginManagement {
    repositories {
        google()
        // Fallback mirror for Maven Central. Keep Maven Central below it so hosted
        // runners can still resolve dependencies when repo.maven.apache.org is rate-limited.
        maven {
            url = uri("https://maven-central.storage-download.googleapis.com/maven2/")
        }
        mavenCentral()
        gradlePluginPortal()
    }
}

dependencyResolutionManagement {
    repositoriesMode.set(RepositoriesMode.FAIL_ON_PROJECT_REPOS)
    repositories {
        google()
        // Google Cloud-hosted Maven Central mirror; Maven Central remains the fallback.
        maven {
            url = uri("https://maven-central.storage-download.googleapis.com/maven2/")
        }
        mavenCentral()
    }
}

rootProject.name = "SourbaseAndroidTV"
include(":app")
