plugins {
    kotlin("jvm") version "2.4.10"
}

repositories { mavenCentral() }

dependencies {
    implementation("org.jetbrains.kotlinx:kotlinx-coroutines-core:1.10.2")
    testImplementation("junit:junit:4.13.2")
    testImplementation("org.jetbrains.kotlinx:kotlinx-coroutines-test:1.10.2")
}

// Host-JVM tests for the deck's rules (journal, controller over a fake engine). The Android
// engine, Media3 and the media session stay in the APK's source set and are proved on the phone.
sourceSets {
    main {
        kotlin.setSrcDirs(listOf("../../mobile/exact-package/android"))
        kotlin.include("DeckContract.kt", "DeckEngine.kt", "DeckJournal.kt", "DeckState.kt", "DeckController.kt")
    }
    test {
        kotlin.setSrcDirs(listOf("kotlin"))
    }
}

kotlin { jvmToolchain(21) }
