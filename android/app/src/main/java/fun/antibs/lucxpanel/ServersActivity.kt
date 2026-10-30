package `fun`.antibs.lucxpanel

import android.app.Activity
import android.app.AlertDialog
import android.content.Intent
import android.net.Uri
import android.os.Bundle
import android.view.Menu
import android.view.MenuItem
import android.widget.ArrayAdapter
import android.widget.CheckBox
import android.widget.EditText
import android.widget.LinearLayout
import android.widget.ListView
import android.widget.TextView
import android.widget.Toast

class ServersActivity : Activity() {
    private lateinit var listView: ListView
    private lateinit var empty: TextView

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        title = getString(R.string.servers)
        val root = LinearLayout(this).apply { orientation = LinearLayout.VERTICAL }
        empty = TextView(this).apply { setText(R.string.no_servers); setPadding(48, 48, 48, 48) }
        listView = ListView(this)
        root.addView(empty)
        root.addView(listView, LinearLayout.LayoutParams(-1, -1))
        setContentView(root)
        listView.setOnItemClickListener { _, _, pos, _ -> open(Servers.list(this)[pos]) }
        listView.setOnItemLongClickListener { _, _, pos, _ ->
            val s = Servers.list(this)[pos]
            AlertDialog.Builder(this).setTitle(s.name)
                .setItems(arrayOf(getString(R.string.open), getString(R.string.edit), getString(R.string.delete))) { _, w ->
                    when (w) {
                        0 -> open(s)
                        1 -> editDialog(s)
                        2 -> AlertDialog.Builder(this).setMessage(R.string.delete_confirm)
                            .setPositiveButton(R.string.delete) { _, _ ->
                                Servers.delete(this, s.id)
                                refresh()
                            }.setNegativeButton(R.string.cancel, null).show()
                    }
                }.show()
            true
        }
        refresh()
        if (Servers.list(this).isEmpty()) editDialog(null)
    }

    private fun refresh() {
        val l = Servers.list(this)
        empty.visibility = if (l.isEmpty()) TextView.VISIBLE else TextView.GONE
        val cur = Servers.currentId(this)
        listView.adapter = object : ArrayAdapter<Server>(this, android.R.layout.simple_list_item_2, android.R.id.text1, l) {
            override fun getView(position: Int, convertView: android.view.View?, parent: android.view.ViewGroup): android.view.View {
                val v = super.getView(position, convertView, parent)
                val s = l[position]
                v.findViewById<TextView>(android.R.id.text1).text = (if (s.id == cur) "● " else "") + s.name
                v.findViewById<TextView>(android.R.id.text2).text = s.url + if (s.allowInsecure) "  ⚠ self-signed" else ""
                return v
            }
        }
    }

    private fun open(s: Server) {
        Servers.setCurrent(this, s.id)
        startActivity(Intent(this, MainActivity::class.java).addFlags(Intent.FLAG_ACTIVITY_CLEAR_TOP))
        finish()
    }

    private fun editDialog(existing: Server?) {
        val pad = 48
        val box = LinearLayout(this).apply { orientation = LinearLayout.VERTICAL; setPadding(pad, pad / 2, pad, 0) }
        val name = EditText(this).apply { setHint(R.string.name); setSingleLine(); setText(existing?.name ?: "") }
        val url = EditText(this).apply {
            setHint(R.string.url_hint); setSingleLine()
            inputType = android.text.InputType.TYPE_CLASS_TEXT or android.text.InputType.TYPE_TEXT_VARIATION_URI
            setText(existing?.url ?: "https://")
        }
        val insecure = CheckBox(this).apply { setText(R.string.allow_insecure); isChecked = existing?.allowInsecure ?: false }
        box.addView(name); box.addView(url); box.addView(insecure)
        val d = AlertDialog.Builder(this).setTitle(if (existing == null) R.string.add_server else R.string.edit_server)
            .setView(box).setPositiveButton(R.string.save, null).setNegativeButton(R.string.cancel, null).create()
        d.setOnShowListener {
            d.getButton(AlertDialog.BUTTON_POSITIVE).setOnClickListener {
                val u = url.text.toString().trim()
                val parsed = Uri.parse(u)
                if ((parsed.scheme != "http" && parsed.scheme != "https") || parsed.host.isNullOrEmpty()) {
                    Toast.makeText(this, R.string.invalid_url, Toast.LENGTH_LONG).show(); return@setOnClickListener
                }
                val n = name.text.toString().trim().ifEmpty { parsed.host!! }
                val s = Server(existing?.id ?: Servers.newId(), n, u, insecure.isChecked)
                Servers.upsert(this, s)
                if (Servers.current(this) == null) Servers.setCurrent(this, s.id)
                d.dismiss(); refresh()
            }
        }
        d.show()
    }

    override fun onCreateOptionsMenu(menu: Menu): Boolean {
        menu.add(0, 1, 0, R.string.add_server).setShowAsAction(MenuItem.SHOW_AS_ACTION_ALWAYS)
        return true
    }

    override fun onOptionsItemSelected(item: MenuItem): Boolean {
        if (item.itemId == 1) { editDialog(null); return true }
        return super.onOptionsItemSelected(item)
    }
}

