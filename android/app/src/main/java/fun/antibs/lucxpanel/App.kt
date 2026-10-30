package `fun`.antibs.lucxpanel

import android.app.Application
import android.content.Context
import androidx.appcompat.app.AppCompatDelegate

object Prefs {
    const val STYLE_CARDS = "cards"
    const val STYLE_GRID = "grid"
    const val STYLE_MINIMAL = "minimal"
    private fun p(c: Context) = c.getSharedPreferences("settings", Context.MODE_PRIVATE)
    fun style(c: Context) = p(c).getString("style", STYLE_CARDS) ?: STYLE_CARDS
    fun setStyle(c: Context, v: String) = p(c).edit().putString("style", v).apply()
    fun theme(c: Context) = p(c).getString("theme", "system") ?: "system"
    fun setTheme(c: Context, v: String) { p(c).edit().putString("theme", v).apply(); applyTheme(v) }
    fun applyTheme(v: String) = AppCompatDelegate.setDefaultNightMode(
        when (v) {
            "dark" -> AppCompatDelegate.MODE_NIGHT_YES
            "light" -> AppCompatDelegate.MODE_NIGHT_NO
            else -> AppCompatDelegate.MODE_NIGHT_FOLLOW_SYSTEM
        }
    )
}

class App : Application() {
    override fun onCreate() {
        super.onCreate()
        Prefs.applyTheme(Prefs.theme(this))
    }
}
