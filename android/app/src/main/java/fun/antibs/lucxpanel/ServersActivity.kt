package `fun`.antibs.lucxpanel

import android.content.Intent
import android.content.res.ColorStateList
import android.graphics.Canvas
import android.graphics.Color
import android.net.Uri
import android.os.Bundle
import android.text.format.DateUtils
import android.view.LayoutInflater
import android.view.Menu
import android.view.MenuItem
import android.view.View
import android.view.ViewGroup
import android.widget.ArrayAdapter
import android.widget.CheckBox
import android.widget.EditText
import android.widget.LinearLayout
import android.widget.Spinner
import android.widget.TextView
import android.widget.Toast
import androidx.appcompat.app.AppCompatActivity
import androidx.recyclerview.widget.DividerItemDecoration
import androidx.recyclerview.widget.GridLayoutManager
import androidx.recyclerview.widget.ItemTouchHelper
import androidx.recyclerview.widget.LinearLayoutManager
import androidx.recyclerview.widget.RecyclerView
import androidx.swiperefreshlayout.widget.SwipeRefreshLayout
import com.google.android.material.appbar.MaterialToolbar
import com.google.android.material.dialog.MaterialAlertDialogBuilder
import com.google.android.material.floatingactionbutton.FloatingActionButton
import com.google.android.material.snackbar.Snackbar

class ServersActivity : AppCompatActivity() {
    private lateinit var list: RecyclerView
    private lateinit var swipe: SwipeRefreshLayout
    private lateinit var fab: FloatingActionButton
    private lateinit var empty: TextView
    private lateinit var header: View
    private lateinit var toolbar: MaterialToolbar
    private var style = Prefs.STYLE_CARDS
    private var servers: List<Server> = emptyList()
    private val status = HashMap<String, Long>() // id -> ms, -1 offline; absent = checking
    private var touchHelper: ItemTouchHelper? = null
    private var divider: RecyclerView.ItemDecoration? = null

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContentView(R.layout.activity_servers)
        toolbar = findViewById(R.id.toolbar)
        setSupportActionBar(toolbar)
        setTitle(R.string.servers)
        list = findViewById(R.id.list)
        swipe = findViewById(R.id.swipe)
        fab = findViewById(R.id.fab)
        empty = findViewById(R.id.empty)
        header = findViewById(R.id.header)
        fab.setOnClickListener { editDialog(null) }
        swipe.setOnRefreshListener { pingAll() }
        if (Servers.list(this).isEmpty()) editDialog(null)
    }

    override fun onResume() {
        super.onResume()
        applyStyle()
        reload()
        pingAll()
    }

    private fun applyStyle() {
        style = Prefs.style(this)
        val root = findViewById<View>(R.id.root)
        val cards = style == Prefs.STYLE_CARDS
        val navy = getColor(R.color.navy)
        if (cards) {
            root.setBackgroundColor(navy)
            toolbar.setBackgroundColor(navy)
            toolbar.setTitleTextColor(Color.WHITE)
            empty.setTextColor(Color.WHITE)
        } else {
            val tv = android.util.TypedValue()
            theme.resolveAttribute(android.R.attr.colorBackground, tv, true)
            root.setBackgroundColor(tv.data)
            toolbar.setBackgroundColor(if (style == Prefs.STYLE_GRID) getColor(R.color.accent) else tv.data)
            val onSurface = com.google.android.material.color.MaterialColors.getColor(toolbar, com.google.android.material.R.attr.colorOnSurface)
            toolbar.setTitleTextColor(if (style == Prefs.STYLE_GRID) Color.WHITE else onSurface)
            empty.setTextColor(onSurface)
        }
        header.visibility = if (style == Prefs.STYLE_GRID) View.VISIBLE else View.GONE
        fab.visibility = if (style == Prefs.STYLE_GRID) View.GONE else View.VISIBLE
        list.layoutManager = if (style == Prefs.STYLE_GRID) GridLayoutManager(this, 2) else LinearLayoutManager(this)
        val pad = if (style == Prefs.STYLE_GRID) dp(8) else 0
        list.setPadding(pad, pad, pad, dp(96))
        divider?.let { list.removeItemDecoration(it) }; divider = null
        touchHelper?.attachToRecyclerView(null); touchHelper = null
        if (style == Prefs.STYLE_MINIMAL) {
            divider = DividerItemDecoration(this, DividerItemDecoration.VERTICAL).also { list.addItemDecoration(it) }
            touchHelper = ItemTouchHelper(SwipeCallback()).also { it.attachToRecyclerView(list) }
        }
        list.adapter = Adapter()
        invalidateOptionsMenu()
    }

    private fun dp(v: Int) = (v * resources.displayMetrics.density).toInt()

    private fun reload() {
        servers = Servers.list(this)
        empty.visibility = if (servers.isEmpty() && style != Prefs.STYLE_GRID) View.VISIBLE else View.GONE
        findViewById<TextView>(R.id.subtitle).text =
            getString(R.string.dashboard_subtitle) + " · " + getString(R.string.servers_count, servers.size)
        list.adapter?.notifyDataSetChanged()
    }

    private fun pingAll() {
        status.clear()
        list.adapter?.notifyDataSetChanged()
        if (servers.isEmpty()) { swipe.isRefreshing = false; return }
        var left = servers.size
        servers.forEach { s ->
            Ping.check(s) { ms ->
                status[s.id] = ms
                val i = servers.indexOfFirst { it.id == s.id }
                if (i >= 0) list.adapter?.notifyItemChanged(i)
                if (--left <= 0) swipe.isRefreshing = false
            }
        }
    }

    private fun open(s: Server) {
        Servers.setCurrent(this, s.id)
        Servers.markOpened(this, s.id)
        startActivity(Intent(this, MainActivity::class.java).addFlags(Intent.FLAG_ACTIVITY_CLEAR_TOP))
        finish()
    }

    private fun itemMenu(s: Server) {
        MaterialAlertDialogBuilder(this).setTitle(s.name)
            .setItems(arrayOf(getString(R.string.open), getString(R.string.edit), getString(R.string.delete))) { _, w ->
                when (w) {
                    0 -> open(s)
                    1 -> editDialog(s)
                    2 -> MaterialAlertDialogBuilder(this).setMessage(R.string.delete_confirm)
                        .setPositiveButton(R.string.delete) { _, _ -> Servers.delete(this, s.id); reload() }
                        .setNegativeButton(R.string.cancel, null).show()
                }
            }.show()
    }

    private fun deleteWithUndo(pos: Int) {
        val s = servers.getOrNull(pos) ?: return
        val wasCurrent = Servers.currentId(this) == s.id
        Servers.delete(this, s.id)
        reload()
        Snackbar.make(findViewById(R.id.root), getString(R.string.deleted, s.name), Snackbar.LENGTH_LONG)
            .setAction(R.string.undo) {
                Servers.insertAt(this, pos, s)
                if (wasCurrent) Servers.setCurrent(this, s.id)
                reload()
            }.setActionTextColor(getColor(R.color.accent)).show()
    }

    private fun statusColor(id: String): Int = when (val ms = status[id]) {
        null -> getColor(R.color.unknown)
        else -> if (ms >= 0) getColor(R.color.online) else getColor(R.color.offline)
    }

    private fun pingText(id: String): String = when (val ms = status[id]) {
        null -> "…"
        -1L -> "—"
        else -> "$ms ms"
    }

    inner class Holder(v: View) : RecyclerView.ViewHolder(v)

    inner class Adapter : RecyclerView.Adapter<Holder>() {
        private val addType = 99
        override fun getItemCount() = servers.size + if (style == Prefs.STYLE_GRID) 1 else 0
        override fun getItemViewType(position: Int) = if (position >= servers.size) addType else 0

        override fun onCreateViewHolder(parent: ViewGroup, viewType: Int): Holder {
            val layout = when (style) {
                Prefs.STYLE_GRID -> R.layout.item_tile
                Prefs.STYLE_MINIMAL -> R.layout.item_row
                else -> R.layout.item_card
            }
            return Holder(LayoutInflater.from(parent.context).inflate(layout, parent, false))
        }

        override fun onBindViewHolder(h: Holder, position: Int) {
            val v = h.itemView
            fun <T : View> f(id: Int): T = v.findViewById(id)
            if (getItemViewType(position) == addType) {
                f<TextView>(R.id.flag).text = "➕"
                f<TextView>(R.id.name).setText(R.string.add_tile)
                f<TextView>(R.id.status).text = ""
                f<View>(R.id.dot).visibility = View.GONE
                v.setOnClickListener { editDialog(null) }
                v.setOnLongClickListener(null)
                return
            }
            val s = servers[position]
            f<TextView>(R.id.flag).text = Flags.display(s.flag)
            f<TextView>(R.id.name).text = s.name
            val dot = f<View>(R.id.dot)
            dot.visibility = View.VISIBLE
            dot.backgroundTintList = ColorStateList.valueOf(statusColor(s.id))
            when (style) {
                Prefs.STYLE_GRID -> f<TextView>(R.id.status).text = when (val ms = status[s.id]) {
                    null -> getString(R.string.checking)
                    -1L -> getString(R.string.offline)
                    else -> getString(R.string.online) + " · $ms ms"
                }
                Prefs.STYLE_MINIMAL -> {
                    f<TextView>(R.id.ping).text = pingText(s.id)
                    f<TextView>(R.id.last).text = getString(R.string.last_opened,
                        if (s.lastOpened == 0L) getString(R.string.never)
                        else DateUtils.getRelativeDateTimeString(this@ServersActivity, s.lastOpened,
                            DateUtils.MINUTE_IN_MILLIS, DateUtils.WEEK_IN_MILLIS, 0))
                }
                else -> {
                    f<TextView>(R.id.url).text = s.url + if (s.allowInsecure) "  ⚠" else ""
                    f<TextView>(R.id.ping).text = pingText(s.id)
                }
            }
            v.setOnClickListener { open(s) }
            if (style == Prefs.STYLE_MINIMAL) v.setOnLongClickListener(null)
            else v.setOnLongClickListener { itemMenu(s); true }
        }
    }

    inner class SwipeCallback : ItemTouchHelper.SimpleCallback(0, ItemTouchHelper.LEFT or ItemTouchHelper.RIGHT) {
        override fun onMove(rv: RecyclerView, a: RecyclerView.ViewHolder, b: RecyclerView.ViewHolder) = false
        override fun onSwiped(vh: RecyclerView.ViewHolder, direction: Int) {
            val pos = vh.bindingAdapterPosition
            if (direction == ItemTouchHelper.LEFT) deleteWithUndo(pos)
            else {
                list.adapter?.notifyItemChanged(pos)
                servers.getOrNull(pos)?.let { editDialog(it) }
            }
        }

        override fun onChildDraw(c: Canvas, rv: RecyclerView, vh: RecyclerView.ViewHolder, dX: Float, dY: Float, state: Int, active: Boolean) {
            val v = vh.itemView
            val paint = android.graphics.Paint().apply { color = getColor(if (dX < 0) R.color.offline else R.color.accent) }
            if (dX < 0) c.drawRect(v.right + dX, v.top.toFloat(), v.right.toFloat(), v.bottom.toFloat(), paint)
            else if (dX > 0) c.drawRect(v.left.toFloat(), v.top.toFloat(), v.left + dX, v.bottom.toFloat(), paint)
            super.onChildDraw(c, rv, vh, dX, dY, state, active)
        }
    }

    private fun editDialog(existing: Server?) {
        val pad = dp(20)
        val box = LinearLayout(this).apply { orientation = LinearLayout.VERTICAL; setPadding(pad, pad / 2, pad, 0) }
        val flagLabel = TextView(this).apply { setText(R.string.flag) }
        val flag = Spinner(this)
        flag.adapter = ArrayAdapter(this, android.R.layout.simple_spinner_dropdown_item,
            Flags.list.map { (e, c) -> if (e.isEmpty()) c else "$e  $c" })
        flag.setSelection(Flags.list.indexOfFirst { it.first == (existing?.flag ?: "") }.coerceAtLeast(0))
        var flagTouched = existing != null && existing.flag.isNotEmpty()
        flag.setOnTouchListener { _, _ -> flagTouched = true; false }
        val name = EditText(this).apply { setHint(R.string.name); setSingleLine(); setText(existing?.name ?: "") }
        val url = EditText(this).apply {
            setHint(R.string.url_hint); setSingleLine()
            inputType = android.text.InputType.TYPE_CLASS_TEXT or android.text.InputType.TYPE_TEXT_VARIATION_URI
            setText(existing?.url ?: "https://")
            setOnFocusChangeListener { _, has ->
                if (!has && !flagTouched) {
                    val g = Flags.guess(Uri.parse(text.toString().trim()).host)
                    flag.setSelection(Flags.list.indexOfFirst { it.first == g }.coerceAtLeast(0))
                }
            }
        }
        val insecure = CheckBox(this).apply { setText(R.string.allow_insecure); isChecked = existing?.allowInsecure ?: false }
        box.addView(name); box.addView(url); box.addView(flagLabel); box.addView(flag); box.addView(insecure)
        val d = MaterialAlertDialogBuilder(this).setTitle(if (existing == null) R.string.add_server else R.string.edit_server)
            .setView(box).setPositiveButton(R.string.save, null).setNegativeButton(R.string.cancel, null).create()
        d.setOnShowListener {
            d.getButton(android.app.AlertDialog.BUTTON_POSITIVE).setOnClickListener {
                val u = url.text.toString().trim()
                val parsed = Uri.parse(u)
                if ((parsed.scheme != "http" && parsed.scheme != "https") || parsed.host.isNullOrEmpty()) {
                    Toast.makeText(this, R.string.invalid_url, Toast.LENGTH_LONG).show(); return@setOnClickListener
                }
                val n = name.text.toString().trim().ifEmpty { parsed.host!! }
                var fl = Flags.list[flag.selectedItemPosition].first
                if (fl.isEmpty() && !flagTouched) fl = Flags.guess(parsed.host)
                val s = (existing ?: Server(Servers.newId(), n, u, false))
                    .copy(name = n, url = u, allowInsecure = insecure.isChecked, flag = fl)
                Servers.upsert(this, s)
                if (Servers.current(this) == null) Servers.setCurrent(this, s.id)
                d.dismiss(); reload(); pingAll()
            }
        }
        d.show()
    }

    override fun onCreateOptionsMenu(menu: Menu): Boolean {
        if (style == Prefs.STYLE_GRID) menu.add(0, 1, 0, R.string.add_server).setShowAsAction(MenuItem.SHOW_AS_ACTION_NEVER)
        menu.add(0, 2, 1, R.string.settings).setIcon(R.drawable.ic_settings).setShowAsAction(MenuItem.SHOW_AS_ACTION_ALWAYS)
            .icon?.setTint(if (style == Prefs.STYLE_MINIMAL) getColor(R.color.accent) else Color.WHITE)
        return true
    }

    override fun onOptionsItemSelected(item: MenuItem): Boolean {
        when (item.itemId) {
            1 -> editDialog(null)
            2 -> startActivity(Intent(this, SettingsActivity::class.java))
            else -> return super.onOptionsItemSelected(item)
        }
        return true
    }
}
