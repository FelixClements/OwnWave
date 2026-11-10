// Kotlin client generated from the server's OpenAPI spec on every build.
// Nothing generated is checked in: a spec change that breaks the app breaks the compile.
plugins {
    alias(libs.plugins.kotlin.jvm)
    alias(libs.plugins.kotlin.serialization)
    alias(libs.plugins.openapi.generator)
}

val spec = rootProject.layout.projectDirectory.file("../../services/go-api-server/openapi.yaml")
val generatedDir = layout.buildDirectory.dir("generated/openapi")

openApiGenerate {
    generatorName.set("kotlin")
    library.set("jvm-retrofit2")
    inputSpec.set(spec.asFile.absolutePath)
    outputDir.set(generatedDir)
    packageName.set("io.github.felixclements.ownwave.api")
    generateApiTests.set(false)
    generateModelTests.set(false)
    generateApiDocumentation.set(false)
    generateModelDocumentation.set(false)
    configOptions.putAll(
        mapOf(
            "serializationLibrary" to "kotlinx_serialization",
            "useCoroutines" to "true",
            "dateLibrary" to "java8",
            "enumPropertyNaming" to "UPPERCASE",
            "omitGradleWrapper" to "true",
            "sourceFolder" to "src/main/kotlin",
        ),
    )
}

sourceSets.main {
    kotlin.srcDir(generatedDir.map { it.dir("src/main/kotlin") })
}

tasks.named("compileKotlin") { dependsOn(tasks.named("openApiGenerate")) }
tasks.openApiGenerate { inputs.file(spec) }

kotlin { jvmToolchain(21) }

dependencies {
    api(libs.retrofit)
    api(libs.okhttp)
    api(libs.kotlinx.serialization.json)
    api(libs.retrofit.kotlinx.serialization)
    api(libs.retrofit.scalars)
    implementation(libs.okhttp.logging)
}
