package com.sourbase.tv.data

import android.content.Context
import android.content.Intent
import android.util.Base64
import org.json.JSONObject
import java.io.IOException
import java.nio.charset.StandardCharsets
import java.security.KeyStore
import javax.crypto.Cipher
import javax.crypto.KeyGenerator
import javax.crypto.SecretKey
import javax.crypto.spec.GCMParameterSpec
import android.security.keystore.KeyGenParameterSpec
import android.security.keystore.KeyProperties

/**
 * Stores provider playlist/EPG URLs encrypted with an AES-GCM key held by Android Keystore.
 *
 * SharedPreferences contains only Base64-encoded IV and ciphertext. The AES key is
 * non-exportable from Android Keystore, and the encrypted values are kept in the
 * app-private preferences file. Auto Backup is disabled in the application manifest.
 *
 * Never log URLs, plaintext JSON, IV/ciphertext records, or cryptographic exceptions.
 */
class SecureSourceStore(context: Context) {
    private val appContext = context.applicationContext
    private val preferences = appContext.getSharedPreferences(PREFERENCES_FILE, Context.MODE_PRIVATE)

    @Synchronized
    fun load(): StoredSourceUrls {
        val record = preferences.getString(ENCRYPTED_SOURCES_KEY, null)
            ?: return StoredSourceUrls()

        return try {
            val plaintext = decrypt(record)
            val json = JSONObject(String(plaintext, StandardCharsets.UTF_8))
            StoredSourceUrls(
                playlistUrl = json.optString(PLAYLIST_URL_FIELD).takeIf(String::isNotBlank),
                xmltvUrl = json.optString(XMLTV_URL_FIELD).takeIf(String::isNotBlank),
                playlistUri = json.optString(PLAYLIST_URI_FIELD).takeIf(String::isNotBlank),
                xmltvUri = json.optString(XMLTV_URI_FIELD).takeIf(String::isNotBlank)
            )
        } catch (_: Exception) {
            // Do not silently discard or overwrite data that can no longer be decrypted.
            throw IllegalStateException(
                "Uložené zdroje nelze bezpečně dešifrovat. Zachoval jsem je beze změny."
            )
        }
    }

    @Synchronized
    @Throws(IOException::class)
    fun save(
        playlistUrl: String?,
        xmltvUrl: String?,
        playlistUri: String? = null,
        xmltvUri: String? = null
    ) {
        val playlist = playlistUrl?.trim()?.takeIf(String::isNotEmpty)
        val guide = xmltvUrl?.trim()?.takeIf(String::isNotEmpty)
        val playlistDocument = playlistUri?.trim()?.takeIf(String::isNotEmpty)
        val guideDocument = xmltvUri?.trim()?.takeIf(String::isNotEmpty)

        if (playlist == null && guide == null && playlistDocument == null && guideDocument == null) {
            clear()
            return
        }

        val plaintext = JSONObject()
            .put(PLAYLIST_URL_FIELD, playlist.orEmpty())
            .put(XMLTV_URL_FIELD, guide.orEmpty())
            .put(PLAYLIST_URI_FIELD, playlistDocument.orEmpty())
            .put(XMLTV_URI_FIELD, guideDocument.orEmpty())
            .toString()
            .toByteArray(StandardCharsets.UTF_8)

        val cipher = Cipher.getInstance(TRANSFORMATION)
        cipher.init(Cipher.ENCRYPT_MODE, getOrCreateKey())
        cipher.updateAAD(appContext.packageName.toByteArray(StandardCharsets.UTF_8))

        val encrypted = cipher.doFinal(plaintext)
        val iv = cipher.iv
        check(iv.size == GCM_IV_LENGTH_BYTES) { "Unexpected AES-GCM IV length." }

        val record = buildString {
            append(Base64.encodeToString(iv, Base64.NO_WRAP))
            append(RECORD_SEPARATOR)
            append(Base64.encodeToString(encrypted, Base64.NO_WRAP))
        }

        if (!preferences.edit().putString(ENCRYPTED_SOURCES_KEY, record).commit()) {
            throw IOException("Šifrované zdroje se nepodařilo uložit.")
        }
    }

    @Synchronized
    @Throws(IOException::class)
    fun clear() {
        if (!preferences.edit().remove(ENCRYPTED_SOURCES_KEY).commit()) {
            throw IOException("Šifrované zdroje se nepodařilo vymazat.")
        }

        // This store owns the SAF grants used for M3U/XMLTV document imports.
        // Releasing all app-persisted read grants also handles records that were
        // too damaged to decrypt and therefore no longer expose their document URIs.
        appContext.contentResolver.persistedUriPermissions
            .filter { it.isReadPermission }
            .forEach { permission ->
                runCatching {
                    appContext.contentResolver.releasePersistableUriPermission(
                        permission.uri,
                        Intent.FLAG_GRANT_READ_URI_PERMISSION
                    )
                }
            }

        // Also remove a possibly invalidated key so a fresh source setup can recover.
        val keyStore = KeyStore.getInstance(ANDROID_KEY_STORE).apply { load(null) }
        if (keyStore.containsAlias(KEY_ALIAS)) {
            keyStore.deleteEntry(KEY_ALIAS)
        }
    }

    private fun decrypt(record: String): ByteArray {
        val separatorIndex = record.indexOf(RECORD_SEPARATOR)
        require(separatorIndex > 0 && separatorIndex < record.lastIndex) {
            "Invalid encrypted source record."
        }

        val iv = Base64.decode(record.substring(0, separatorIndex), Base64.NO_WRAP)
        val encrypted = Base64.decode(record.substring(separatorIndex + 1), Base64.NO_WRAP)
        require(iv.size == GCM_IV_LENGTH_BYTES && encrypted.size >= GCM_TAG_LENGTH_BITS / 8) {
            "Invalid encrypted source record."
        }

        val cipher = Cipher.getInstance(TRANSFORMATION)
        cipher.init(Cipher.DECRYPT_MODE, getOrCreateKey(), GCMParameterSpec(GCM_TAG_LENGTH_BITS, iv))
        cipher.updateAAD(appContext.packageName.toByteArray(StandardCharsets.UTF_8))
        return cipher.doFinal(encrypted)
    }

    private fun getOrCreateKey(): SecretKey {
        val keyStore = KeyStore.getInstance(ANDROID_KEY_STORE).apply { load(null) }
        val existingKey = keyStore.getKey(KEY_ALIAS, null) as? SecretKey
        if (existingKey != null) return existingKey

        val generator = KeyGenerator.getInstance(KeyProperties.KEY_ALGORITHM_AES, ANDROID_KEY_STORE)
        val keySpec = KeyGenParameterSpec.Builder(
            KEY_ALIAS,
            KeyProperties.PURPOSE_ENCRYPT or KeyProperties.PURPOSE_DECRYPT
        )
            .setBlockModes(KeyProperties.BLOCK_MODE_GCM)
            .setEncryptionPaddings(KeyProperties.ENCRYPTION_PADDING_NONE)
            .setKeySize(AES_KEY_SIZE_BITS)
            .setRandomizedEncryptionRequired(true)
            .setUserAuthenticationRequired(false)
            .build()

        generator.init(keySpec)
        return generator.generateKey()
    }

    private companion object {
        const val ANDROID_KEY_STORE = "AndroidKeyStore"
        const val KEY_ALIAS = "sourbase.source-urls.aes-gcm.v1"
        const val PREFERENCES_FILE = "sourbase.secure-source-settings.v1"
        const val ENCRYPTED_SOURCES_KEY = "encrypted_source_urls"
        const val PLAYLIST_URL_FIELD = "playlistUrl"
        const val XMLTV_URL_FIELD = "xmltvUrl"
        const val PLAYLIST_URI_FIELD = "playlistUri"
        const val XMLTV_URI_FIELD = "xmltvUri"
        const val TRANSFORMATION = "AES/GCM/NoPadding"
        const val AES_KEY_SIZE_BITS = 256
        const val GCM_IV_LENGTH_BYTES = 12
        const val GCM_TAG_LENGTH_BITS = 128
        const val RECORD_SEPARATOR = '.'
    }
}

data class StoredSourceUrls(
    val playlistUrl: String? = null,
    val xmltvUrl: String? = null,
    val playlistUri: String? = null,
    val xmltvUri: String? = null
)
