package `fun`.antibs.lucxpanel

import android.os.Bundle
import android.view.MenuItem
import android.widget.LinearLayout
import android.widget.RadioButton
import android.widget.RadioGroup
import android.widget.ScrollView
import android.widget.TextView
import androidx.appcompat.app.AppCompatActivity

class SettingsActivity : AppCompatActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setTitle(R.string.settings)
        supportActionBar?.setDisplayHomeAsUpEnabled(true)
        val pad = (16 * resources.displayMetrics.density).toInt()
        val box = LinearLayout(this).apply { orientation = LinearLayout.VERTICAL; setPadding(pad, pad, pad, pad) }
        fun header(t: Int) = TextView(this).apply {
            setText(t); textSize = 15f; setTypeface(typeface, android.graphics.Typeface.BOLD)
            setTextColor(getColor(R.color.accent)); setPadding(0, pad, 0, pad / 2)
        }
        fun group(items: List<Pair<String, Int>>, cur: String, onPick: (String) -> Unit) = RadioGroup(this).apply {
            items.forEachIndexed { i, (k, label) ->
                addView(RadioButton(this@SettingsActivity).apply { id = 1000 + i; setText(label); isChecked = k == cur })
            }
            setOnCheckedChangeListener { _, id -> onPick(items[id - 1000].first) }
        }
        box.addView(header(R.string.list_style))
        box.addView(group(listOf(Prefs.STYLE_CARDS to R.string.style_cards, Prefs.STYLE_GRID to R.string.style_grid,
            Prefs.STYLE_MINIMAL to R.string.style_minimal), Prefs.style(this)) { Prefs.setStyle(this, it) })
        box.addView(header(R.string.theme))
        box.addView(group(listOf("system" to R.string.theme_system, "dark" to R.string.theme_dark, "light" to R.string.theme_light),
            Prefs.theme(this)) { Prefs.setTheme(this, it) })
        setContentView(ScrollView(this).apply { addView(box) })
    }

    override fun onOptionsItemSelected(item: MenuItem): Boolean {
        if (item.itemId == android.R.id.home) { finish(); return true }
        return super.onOptionsItemSelected(item)
    }
}
