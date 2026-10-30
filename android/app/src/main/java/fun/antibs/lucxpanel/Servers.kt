package `fun`.antibs.lucxpanel

import android.content.Context
import org.json.JSONArray
import org.json.JSONObject
import java.util.UUID

data class Server(
    val id: String,
    val name: String,
    val url: String,
    val allowInsecure: Boolean,
    val flag: String = "",
    val lastOpened: Long = 0L,
)

object Servers {
    private const val PREFS = "servers"

    private fun prefs(c: Context) = c.getSharedPreferences(PREFS, Context.MODE_PRIVATE)

    fun list(c: Context): MutableList<Server> {
        val arr = JSONArray(prefs(c).getString("list", "[]"))
        return MutableList(arr.length()) {
            val o = arr.getJSONObject(it)
            Server(o.getString("id"), o.getString("name"), o.getString("url"), o.optBoolean("insecure", false),
                o.optString("flag", ""), o.optLong("lastOpened", 0L))
        }
    }

    fun save(c: Context, list: List<Server>) {
        val arr = JSONArray()
        list.forEach {
            arr.put(JSONObject().put("id", it.id).put("name", it.name).put("url", it.url).put("insecure", it.allowInsecure)
                .put("flag", it.flag).put("lastOpened", it.lastOpened))
        }
        prefs(c).edit().putString("list", arr.toString()).apply()
    }

    fun upsert(c: Context, s: Server) {
        val l = list(c)
        val i = l.indexOfFirst { it.id == s.id }
        if (i >= 0) l[i] = s else l.add(s)
        save(c, l)
    }

    fun insertAt(c: Context, index: Int, s: Server) {
        val l = list(c)
        l.add(index.coerceIn(0, l.size), s)
        save(c, l)
    }

    fun delete(c: Context, id: String) {
        save(c, list(c).filter { it.id != id })
        if (currentId(c) == id) setCurrent(c, null)
    }

    fun markOpened(c: Context, id: String) {
        list(c).firstOrNull { it.id == id }?.let { upsert(c, it.copy(lastOpened = System.currentTimeMillis())) }
    }

    fun newId() = UUID.randomUUID().toString()

    fun currentId(c: Context): String? = prefs(c).getString("current", null)
    fun setCurrent(c: Context, id: String?) = prefs(c).edit().putString("current", id).apply()
    fun current(c: Context): Server? = currentId(c)?.let { id -> list(c).firstOrNull { it.id == id } }
}

object Flags {
    val list = listOf(
        "" to "—",
        "🇷🇺" to "RU", "🇩🇪" to "DE", "🇳🇱" to "NL", "🇫🇮" to "FI", "🇫🇷" to "FR", "🇬🇧" to "GB", "🇺🇸" to "US",
        "🇰🇿" to "KZ", "🇹🇷" to "TR", "🇵🇱" to "PL", "🇸🇪" to "SE", "🇨🇭" to "CH", "🇦🇹" to "AT", "🇪🇪" to "EE",
        "🇱🇻" to "LV", "🇱🇹" to "LT", "🇺🇦" to "UA", "🇧🇾" to "BY", "🇬🇪" to "GE", "🇦🇲" to "AM", "🇦🇪" to "AE",
        "🇮🇱" to "IL", "🇯🇵" to "JP", "🇸🇬" to "SG", "🇭🇰" to "HK", "🇰🇷" to "KR", "🇨🇦" to "CA", "🇧🇬" to "BG",
        "🇷🇴" to "RO", "🇨🇿" to "CZ", "🇪🇸" to "ES", "🇮🇹" to "IT", "🇳🇴" to "NO", "🇮🇳" to "IN", "🇧🇷" to "BR",
        "🌐" to "Other",
    )

    /** Best-effort guess from the host's country-code TLD; empty if unknown (.com/.fun etc). */
    fun guess(host: String?): String {
        val tld = host?.substringAfterLast('.', "")?.uppercase() ?: return ""
        val code = if (tld == "UK") "GB" else tld
        return list.firstOrNull { it.second == code }?.first ?: ""
    }

    fun display(flag: String) = flag.ifEmpty { "🖥" }
}
