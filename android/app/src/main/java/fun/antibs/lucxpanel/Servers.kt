package `fun`.antibs.lucxpanel

import android.content.Context
import org.json.JSONArray
import org.json.JSONObject
import java.util.UUID

data class Server(val id: String, val name: String, val url: String, val allowInsecure: Boolean)

object Servers {
    private const val PREFS = "servers"

    private fun prefs(c: Context) = c.getSharedPreferences(PREFS, Context.MODE_PRIVATE)

    fun list(c: Context): MutableList<Server> {
        val arr = JSONArray(prefs(c).getString("list", "[]"))
        return MutableList(arr.length()) {
            val o = arr.getJSONObject(it)
            Server(o.getString("id"), o.getString("name"), o.getString("url"), o.optBoolean("insecure", false))
        }
    }

    fun save(c: Context, list: List<Server>) {
        val arr = JSONArray()
        list.forEach {
            arr.put(JSONObject().put("id", it.id).put("name", it.name).put("url", it.url).put("insecure", it.allowInsecure))
        }
        prefs(c).edit().putString("list", arr.toString()).apply()
    }

    fun upsert(c: Context, s: Server) {
        val l = list(c)
        val i = l.indexOfFirst { it.id == s.id }
        if (i >= 0) l[i] = s else l.add(s)
        save(c, l)
    }

    fun delete(c: Context, id: String) {
        save(c, list(c).filter { it.id != id })
        if (currentId(c) == id) setCurrent(c, null)
    }

    fun newId() = UUID.randomUUID().toString()

    fun currentId(c: Context): String? = prefs(c).getString("current", null)
    fun setCurrent(c: Context, id: String?) = prefs(c).edit().putString("current", id).apply()
    fun current(c: Context): Server? = currentId(c)?.let { id -> list(c).firstOrNull { it.id == id } }
}
