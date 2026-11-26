plugins {
    kotlin("jvm") version "2.4.10"
}

repositories { mavenCentral() }

dependencies {
    testImplementation("junit:junit:4.13.2")
}

// Host-JVM tests only. Android, Exact and ONNX classes stay in the APK's source set;
// these pure execution rules are compiled without an Android plugin or device.
sourceSets {
    main {
        kotlin.setSrcDirs(listOf("../../mobile/exact-package/android"))
        kotlin.include("NativeModelCatalog.kt", "ModelDownloadManager.kt", "SileroState.kt")
    }
}

kotlin { jvmToolchain(21) }
