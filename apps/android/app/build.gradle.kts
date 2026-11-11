plugins {
    alias(libs.plugins.android.application)
    alias(libs.plugins.kotlin.compose)
}

android {
    namespace = "io.github.felixclements.ownwave"
    compileSdk = 37

    defaultConfig {
        applicationId = "io.github.felixclements.ownwave"
        minSdk = 28
        targetSdk = 37
        // Release builds get both from the tag (see .github/workflows/android-release.yml).
        versionCode = providers.gradleProperty("ownwave.versionCode").orNull?.toInt() ?: 1
        versionName = providers.gradleProperty("ownwave.versionName").orNull ?: "0.1.0"
        testInstrumentationRunner = "androidx.test.runner.AndroidJUnitRunner"
    }

    // The release keystore comes from the environment (CI secrets) and is never in the repo.
    // Without OWNWAVE_KEYSTORE_FILE, assembleRelease builds an unsigned APK.
    val keystoreFile = providers.environmentVariable("OWNWAVE_KEYSTORE_FILE").orNull
    signingConfigs {
        if (keystoreFile != null) {
            create("release") {
                storeFile = file(keystoreFile)
                storePassword = providers.environmentVariable("OWNWAVE_KEYSTORE_PASSWORD").get()
                keyAlias = providers.environmentVariable("OWNWAVE_KEY_ALIAS").get()
                keyPassword = providers.environmentVariable("OWNWAVE_KEY_PASSWORD").get()
            }
        }
    }

    buildTypes {
        release {
            signingConfig = signingConfigs.findByName("release")
            isMinifyEnabled = true
            isShrinkResources = true
            proguardFiles(getDefaultProguardFile("proguard-android-optimize.txt"), "proguard-rules.pro")
        }
    }

    buildFeatures {
        compose = true
        buildConfig = true
    }

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_21
        targetCompatibility = JavaVersion.VERSION_21
    }

    testOptions {
        unitTests.isReturnDefaultValues = true
    }

    lint {
        abortOnError = true
        checkDependencies = false
    }
}

kotlin { jvmToolchain(21) }

dependencies {
    implementation(project(":api"))

    implementation(libs.androidx.core.ktx)
    implementation(libs.androidx.activity.compose)
    implementation(libs.androidx.lifecycle.runtime.compose)
    implementation(libs.androidx.lifecycle.viewmodel.compose)
    implementation(libs.androidx.datastore.preferences)
    implementation(libs.kotlinx.coroutines.android)
    implementation(libs.kotlinx.coroutines.guava)

    implementation(libs.media3.exoplayer)
    implementation(libs.media3.session)
    implementation(libs.media3.datasource.okhttp)

    implementation(platform(libs.compose.bom))
    implementation(libs.compose.ui)
    implementation(libs.compose.ui.tooling.preview)
    implementation(libs.compose.material3)
    implementation(libs.compose.material.icons)
    debugImplementation(libs.compose.ui.tooling)

    implementation(libs.coil.compose)
    implementation(libs.coil.network.okhttp)

    testImplementation(libs.junit)
    testImplementation(libs.okhttp.mockwebserver)
    testImplementation(libs.kotlinx.coroutines.test)

    // Manual smoke tests on a signed-in emulator (connectedDebugAndroidTest); CI does not run them.
    androidTestImplementation(libs.androidx.test.runner)
    androidTestImplementation(libs.androidx.test.ext.junit)
}
