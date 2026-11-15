package io.github.felixclements.ownwave.ui.admin

import java.time.OffsetDateTime
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonPrimitive
import kotlinx.serialization.json.contentOrNull
import kotlinx.serialization.json.doubleOrNull
import kotlinx.serialization.json.intOrNull
import kotlinx.serialization.json.jsonArray
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive

/** A scan job from /api/admin/scans (the spec types it as a free-form object). */
data class ScanSummary(
    val id: String,
    val status: String,
    val error: String?,
    val startedAt: OffsetDateTime?,
    val foldersTotal: Int,
    val foldersDone: Int,
    val foldersFailed: Int,
    val imported: Int,
    val modelSuccess: Int,
    val failedFiles: Int,
) {
    val finished get() = status == "completed" || status == "failed"
}

data class ScanPhase(val label: String, val progress: Double?)

/** Where a scan is: folder imports drive the progress (apps/web/src/lib/scan.ts). */
fun ScanSummary.phase(): ScanPhase {
    val finished = foldersDone + foldersFailed
    val total = foldersTotal
    return when (status) {
        "pending" -> ScanPhase("Waiting for a worker…", null)
        "completed" -> ScanPhase("Completed", 1.0)
        "failed" -> ScanPhase(error?.takeIf { it.isNotBlank() } ?: "Failed", if (total > 0) finished.toDouble() / total else null)
        else -> when {
            total == 0 -> ScanPhase("Looking for music folders…", null)
            finished < total -> ScanPhase("Importing folders: $finished of $total", finished.toDouble() / total)
            else -> ScanPhase("Finishing scan…", 1.0)
        }
    }
}

data class Confusion(val tag: String, val audio: String?, val tracks: Int)

data class GenreReport(val audioAgreement: Double?, val tracks: Int, val confusions: List<Confusion>)

data class Invite(val url: String, val expiresAt: OffsetDateTime?)

/** Reads the admin endpoints the generated client types as `Any`. */
object AdminJson {
    private fun JsonObject.str(key: String) = (this[key] as? JsonPrimitive)?.contentOrNull
    private fun JsonObject.int(key: String) = (this[key] as? JsonPrimitive)?.intOrNull ?: 0
    private fun JsonObject.obj(key: String) = this[key] as? JsonObject ?: JsonObject(emptyMap())
    private fun JsonObject.time(key: String) = str(key)?.let { runCatching { OffsetDateTime.parse(it) }.getOrNull() }

    /** Component name to status ("ok" or an error), without the version. */
    fun health(json: JsonElement): List<Pair<String, String>> =
        json.jsonObject.filterKeys { it != "version" }.map { (k, v) -> k to ((v as? JsonPrimitive)?.contentOrNull ?: v.toString()) }

    fun scans(json: JsonElement): List<ScanSummary> {
        val jobs = (json as? JsonObject)?.get("jobs") ?: return emptyList()
        return jobs.jsonArray.map { it.jsonObject.toScan() }
    }

    private fun JsonObject.toScan(): ScanSummary {
        val folders = obj("folders")
        val stats = obj("stats")
        return ScanSummary(
            id = str("id").orEmpty(),
            status = str("status").orEmpty(),
            error = str("error"),
            startedAt = time("started_at"),
            foldersTotal = folders.int("total"),
            foldersDone = folders.int("done"),
            foldersFailed = folders.int("failed"),
            imported = stats.int("imported"),
            modelSuccess = stats.int("model_success"),
            failedFiles = stats.int("failed_files"),
        )
    }

    fun report(json: JsonElement): GenreReport {
        val o = json.jsonObject
        val confusions = (o["top_confusions"] as? kotlinx.serialization.json.JsonArray).orEmpty().map {
            val c = it.jsonObject
            Confusion(c.str("tag").orEmpty(), c.str("audio"), c.int("tracks"))
        }
        return GenreReport((o["audio_agreement"] as? JsonPrimitive)?.doubleOrNull, o.int("tracks"), confusions)
    }

    fun invite(json: JsonElement): Invite {
        val o = json.jsonObject
        return Invite(o["invite_url"]?.jsonPrimitive?.contentOrNull ?: error("No invite link in the response."), o.time("expires_at"))
    }
}
