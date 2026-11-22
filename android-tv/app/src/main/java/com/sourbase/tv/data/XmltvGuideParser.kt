package com.sourbase.tv.data

import org.w3c.dom.Element
import org.w3c.dom.Node
import org.xml.sax.InputSource
import java.io.StringReader
import java.time.LocalDateTime
import java.time.ZoneOffset
import java.time.format.DateTimeFormatter
import java.util.Locale
import java.util.UUID
import javax.xml.XMLConstants
import javax.xml.parsers.DocumentBuilderFactory

/**
 * Secure, fixture-testable XMLTV parser.
 *
 * XMLTV date-times with an explicit offset are converted to UTC epoch millis.
 * Date-times without an offset are interpreted as UTC rather than depending
 * on the device's current timezone.
 */
object XmltvGuideParser {
    private const val MAX_XML_CHARS = 64 * 1024 * 1024
    private val dateFormatter = DateTimeFormatter.ofPattern("uuuuMMddHHmmss", Locale.ROOT)
    private val datePattern = Regex("""^(\d{14})(?:\s*([+-]\d{4}|Z))?.*$""")

    fun parse(content: String): XmltvGuide {
        require(content.length <= MAX_XML_CHARS) {
            "XMLTV guide is too large (maximum 64 MiB of text)."
        }

        val document = try {
            val factory = secureDocumentBuilderFactory()
            val builder = factory.newDocumentBuilder()
            builder.setEntityResolver { _, _ -> InputSource(StringReader("")) }
            builder.parse(InputSource(StringReader(content)))
        } catch (error: Exception) {
            throw IllegalArgumentException("XMLTV document is malformed or unsafe.", error)
        }

        val root = document.documentElement
        require(root != null && root.tagName.equals("tv", ignoreCase = true)) {
            "XMLTV root element must be <tv>."
        }

        val warnings = mutableListOf<String>()
        val channels = root.directChildren("channel").mapNotNull { element ->
            val id = element.getAttribute("id").trim()
            if (id.isEmpty()) {
                warnings += "XMLTV channel without an id was skipped."
                null
            } else {
                val names = element.directChildren("display-name")
                    .map { it.textContent.trim() }
                    .filter(String::isNotEmpty)
                val iconUrl = element.directChildren("icon")
                    .firstNotNullOfOrNull { it.getAttribute("src").trim().takeIf(String::isNotEmpty) }
                    ?.takeIf(::isHttpUrl)

                XmltvChannel(id = id, displayNames = names, logoUrl = iconUrl)
            }
        }

        val programmes = root.directChildren("programme").mapNotNull { element ->
            parseProgramme(element, warnings)
        }

        return XmltvGuide(
            channels = channels,
            programmes = programmes.sortedBy(EpgProgramme::startEpochMs),
            warnings = warnings
        )
    }

    private fun parseProgramme(element: Element, warnings: MutableList<String>): EpgProgramme? {
        val channelId = element.getAttribute("channel").trim()
        val start = parseXmltvTimestamp(element.getAttribute("start"))
        val end = parseXmltvTimestamp(element.getAttribute("stop"))
        val titleElement = element.directChildren("title").firstOrNull()
        val title = titleElement?.textContent?.trim().orEmpty()

        if (channelId.isEmpty() || start == null || end == null || title.isEmpty() || end <= start) {
            warnings += "Programme with missing channel, title or invalid time range was skipped."
            return null
        }

        val description = element.directChildren("desc")
            .firstOrNull()?.textContent?.trim()?.takeIf(String::isNotEmpty)
        val categories = element.directChildren("category")
            .map { it.textContent.trim() }
            .filter(String::isNotEmpty)
        val language = titleElement?.let {
            it.getAttributeNS(XMLConstants.XML_NS_URI, "lang")
                .ifBlank { it.getAttribute("lang") }
                .takeIf(String::isNotBlank)
        }
        val identity = "$channelId|$start|$end|$title"
        val id = UUID.nameUUIDFromBytes(identity.toByteArray(Charsets.UTF_8)).toString()

        return EpgProgramme(
            id = id,
            channelId = channelId,
            title = title,
            description = description,
            startEpochMs = start,
            endEpochMs = end,
            categories = categories,
            language = language
        )
    }

    private fun parseXmltvTimestamp(value: String): Long? {
        val match = datePattern.matchEntire(value.trim()) ?: return null
        return try {
            val localDateTime = LocalDateTime.parse(match.groupValues[1], dateFormatter)
            val offsetValue = match.groupValues.getOrNull(2).orEmpty()
            val offset = when {
                offsetValue.isBlank() || offsetValue == "Z" -> ZoneOffset.UTC
                else -> {
                    val sign = if (offsetValue[0] == '-') -1 else 1
                    val hours = offsetValue.substring(1, 3).toInt()
                    val minutes = offsetValue.substring(3, 5).toInt()
                    ZoneOffset.ofHoursMinutes(sign * hours, sign * minutes)
                }
            }
            localDateTime.toInstant(offset).toEpochMilli()
        } catch (_: Exception) {
            null
        }
    }

    private fun secureDocumentBuilderFactory(): DocumentBuilderFactory =
        DocumentBuilderFactory.newInstance().apply {
            isNamespaceAware = true
            isXIncludeAware = false
            isExpandEntityReferences = false
            setFeature("http://xml.org/sax/features/external-general-entities", false)
            setFeature("http://xml.org/sax/features/external-parameter-entities", false)
            setFeature("http://apache.org/xml/features/nonvalidating/load-external-dtd", false)
            // Android's XMLConstants API surface omits the Java SE constant fields.
            // The JAXP property names are standard strings; unsupported implementations
            // are still protected by the disabled external-entity features and resolver.
            runCatching {
                setAttribute("http://javax.xml.XMLConstants/property/accessExternalDTD", "")
            }
            runCatching {
                setAttribute("http://javax.xml.XMLConstants/property/accessExternalSchema", "")
            }
        }

    private fun Element.directChildren(tagName: String): List<Element> {
        val result = mutableListOf<Element>()
        val nodes = childNodes
        for (index in 0 until nodes.length) {
            val node = nodes.item(index)
            if (node.nodeType == Node.ELEMENT_NODE && node.localNameOrTagName() == tagName) {
                result += node as Element
            }
        }
        return result
    }

    private fun Node.localNameOrTagName(): String = localName ?: nodeName

    private fun isHttpUrl(value: String): Boolean = try {
        val uri = java.net.URI(value)
        (uri.scheme.equals("http", true) || uri.scheme.equals("https", true)) &&
            !uri.host.isNullOrBlank() &&
            uri.rawUserInfo == null
    } catch (_: Exception) {
        false
    }
}
