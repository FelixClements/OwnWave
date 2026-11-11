package io.github.felixclements.ownwave.artwork

import android.content.ContentProvider
import android.content.ContentValues
import android.database.Cursor
import android.graphics.Bitmap
import android.graphics.Canvas
import android.net.Uri
import android.os.ParcelFileDescriptor
import androidx.core.content.ContextCompat
import androidx.core.graphics.createBitmap
import io.github.felixclements.ownwave.R
import io.github.felixclements.ownwave.container
import okhttp3.Request
import java.io.ByteArrayOutputStream
import java.io.FileNotFoundException
import java.io.IOException
import java.util.concurrent.Executors

/**
 * Serves cover images as `content://` URIs (see [ArtworkUris]) for Android Auto and the notification,
 * which load artwork themselves and cannot send the Bearer token.
 *
 * Read-only and images only. Covers are fetched through the app's OkHttp client (its 50 MB disk cache
 * and auth interceptor), at most [MAX_PARALLEL] at a time so a grid of covers does not hit the server
 * in a burst. A track or station with no cover (204 or 404) gets a placeholder; other failures fail the
 * read, so the caller may try again later.
 */
class ArtworkProvider : ContentProvider() {

    private val fetches = Executors.newFixedThreadPool(MAX_PARALLEL)

    private val placeholder: ByteArray by lazy {
        val bitmap = createBitmap(PLACEHOLDER_SIZE, PLACEHOLDER_SIZE)
        val canvas = Canvas(bitmap)
        canvas.drawColor(PLACEHOLDER_BACKGROUND)
        ContextCompat.getDrawable(appContext(), R.drawable.ic_launcher_foreground)?.run {
            setBounds(0, 0, PLACEHOLDER_SIZE, PLACEHOLDER_SIZE)
            draw(canvas)
        }
        ByteArrayOutputStream().also { bitmap.compress(Bitmap.CompressFormat.PNG, 100, it) }.toByteArray()
    }

    override fun onCreate(): Boolean = true

    /** `requireContext()` needs API 30; the context is set before any call reaches the provider. */
    private fun appContext() = checkNotNull(context) { "Provider not attached" }

    override fun getType(uri: Uri): String? =
        if (ArtworkUris.serverPath(uri.pathSegments) != null) "image/*" else null

    override fun openFile(uri: Uri, mode: String): ParcelFileDescriptor {
        if (mode != "r") throw SecurityException("Artwork is read-only")
        val path = ArtworkUris.serverPath(uri.pathSegments) ?: throw FileNotFoundException("Not artwork: $uri")
        val app = appContext().container
        val session = app.sessionStore.current ?: throw FileNotFoundException("Not signed in")
        val url = session.server.newBuilder().addPathSegments(path).build()

        val (read, write) = ParcelFileDescriptor.createReliablePipe()
        fetches.execute {
            try {
                app.httpClient.newCall(Request.Builder().url(url).build()).execute().use { response ->
                    val type = response.body.contentType()
                    when {
                        response.code == 200 && type?.type == "image" ->
                            ParcelFileDescriptor.AutoCloseOutputStream(write).use { response.body.byteStream().copyTo(it) }
                        response.code == 204 || response.code == 404 ->
                            ParcelFileDescriptor.AutoCloseOutputStream(write).use { it.write(placeholder) }
                        else -> write.closeWithError("HTTP ${response.code} ${type ?: ""}".trim())
                    }
                }
            } catch (e: IOException) {
                runCatching { write.closeWithError(e.message ?: "Fetch failed") }
            }
        }
        return read
    }

    override fun query(
        uri: Uri,
        projection: Array<out String>?,
        selection: String?,
        selectionArgs: Array<out String>?,
        sortOrder: String?,
    ): Cursor? = null

    override fun insert(uri: Uri, values: ContentValues?): Uri? = throw UnsupportedOperationException("Read-only")

    override fun update(uri: Uri, values: ContentValues?, selection: String?, selectionArgs: Array<out String>?): Int =
        throw UnsupportedOperationException("Read-only")

    override fun delete(uri: Uri, selection: String?, selectionArgs: Array<out String>?): Int =
        throw UnsupportedOperationException("Read-only")

    private companion object {
        const val MAX_PARALLEL = 4
        const val PLACEHOLDER_SIZE = 256
        const val PLACEHOLDER_BACKGROUND = 0xFF1C1B1F.toInt()
    }
}
