package `fun`.antibs.lucxpanel

import android.annotation.SuppressLint
import android.app.Activity
import android.app.DownloadManager
import android.content.ActivityNotFoundException
import android.content.ContentValues
import android.content.Intent
import android.graphics.Bitmap
import android.net.Uri
import android.net.http.SslError
import android.os.Build
import android.os.Bundle
import android.os.Environment
import android.provider.MediaStore
import android.util.Base64
import android.view.Menu
import android.view.MenuItem
import android.view.View
import android.webkit.*
import android.widget.ProgressBar
import android.widget.Toast
import androidx.appcompat.app.AppCompatActivity
import androidx.swiperefreshlayout.widget.SwipeRefreshLayout
import java.io.File

class MainActivity : AppCompatActivity() {
    private lateinit var web: WebView
    private lateinit var swipe: SwipeRefreshLayout
    private lateinit var progress: ProgressBar
    private var server: Server? = null
    private var fileCallback: ValueCallback<Array<Uri>>? = null

    @SuppressLint("SetJavaScriptEnabled")
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        Servers.currentId(this)?.let { if (savedInstanceState == null) Servers.markOpened(this, it) }
        server = Servers.current(this)
        if (server == null) {
            startActivity(Intent(this, ServersActivity::class.java)); finish(); return
        }
        setContentView(R.layout.activity_main)
        web = findViewById(R.id.web)
        swipe = findViewById(R.id.swipe)
        progress = findViewById(R.id.progress)
        title = server!!.name

        val cm = CookieManager.getInstance()
        cm.setAcceptCookie(true)
        cm.setAcceptThirdPartyCookies(web, true)

        web.settings.apply {
            javaScriptEnabled = true
            domStorageEnabled = true
            databaseEnabled = true
            allowFileAccess = false
            loadWithOverviewMode = true
            useWideViewPort = true
            builtInZoomControls = true
            displayZoomControls = false
            mixedContentMode = WebSettings.MIXED_CONTENT_COMPATIBILITY_MODE
            setSupportMultipleWindows(false)
            javaScriptCanOpenWindowsAutomatically = true
        }
        if (Build.VERSION.SDK_INT >= 29) {
            val night = (resources.configuration.uiMode and android.content.res.Configuration.UI_MODE_NIGHT_MASK) ==
                android.content.res.Configuration.UI_MODE_NIGHT_YES
            @Suppress("DEPRECATION")
            if (Build.VERSION.SDK_INT < 33) web.settings.forceDark = if (night) WebSettings.FORCE_DARK_AUTO else WebSettings.FORCE_DARK_OFF
        }
        web.addJavascriptInterface(BlobBridge(), "LucXAndroid")

        web.webViewClient = object : WebViewClient() {
            override fun shouldOverrideUrlLoading(view: WebView, request: WebResourceRequest): Boolean =
                handleUrl(request.url)

            override fun onPageStarted(view: WebView, url: String?, favicon: Bitmap?) {
                progress.visibility = View.VISIBLE
            }

            override fun onPageFinished(view: WebView, url: String?) {
                progress.visibility = View.GONE
                swipe.isRefreshing = false
                CookieManager.getInstance().flush()
            }

            @SuppressLint("WebViewClientOnReceivedSslError")
            override fun onReceivedSslError(view: WebView, handler: SslErrorHandler, error: SslError) {
                val s = server
                val host = Uri.parse(error.url ?: "").host
                if (s != null && s.allowInsecure && host == Uri.parse(s.url).host) handler.proceed()
                else {
                    handler.cancel()
                    swipe.isRefreshing = false
                    Toast.makeText(this@MainActivity, R.string.ssl_error, Toast.LENGTH_LONG).show()
                }
            }
        }
        web.webChromeClient = object : WebChromeClient() {
            override fun onProgressChanged(view: WebView, newProgress: Int) {
                progress.progress = newProgress
                progress.visibility = if (newProgress < 100) View.VISIBLE else View.GONE
            }

            override fun onReceivedTitle(view: WebView, t: String?) {
                title = server?.name ?: getString(R.string.app_name)
            }

            override fun onShowFileChooser(view: WebView, cb: ValueCallback<Array<Uri>>, params: FileChooserParams): Boolean {
                fileCallback?.onReceiveValue(null)
                fileCallback = cb
                val i = Intent(Intent.ACTION_GET_CONTENT).addCategory(Intent.CATEGORY_OPENABLE).setType("*/*")
                if (params.mode == FileChooserParams.MODE_OPEN_MULTIPLE) i.putExtra(Intent.EXTRA_ALLOW_MULTIPLE, true)
                return try {
                    startActivityForResult(Intent.createChooser(i, null), REQ_FILE); true
                } catch (e: ActivityNotFoundException) {
                    fileCallback = null; false
                }
            }
        }
        web.setDownloadListener { url, ua, cd, mime, _ -> download(url, ua, cd, mime) }

        swipe.setOnRefreshListener { web.reload() }
        // Only allow pull-to-refresh when page is scrolled to top
        swipe.setOnChildScrollUpCallback { _, _ -> web.scrollY > 0 }

        if (savedInstanceState != null) web.restoreState(savedInstanceState) else web.loadUrl(server!!.url)
    }

    private fun handleUrl(uri: Uri): Boolean {
        val scheme = uri.scheme?.lowercase()
        val panelHost = Uri.parse(server!!.url).host
        if ((scheme == "http" || scheme == "https") && uri.host == panelHost) return false
        if (scheme == "blob" || scheme == "data" || scheme == "about" || scheme == "javascript") return false
        try {
            val intent = if (scheme == "intent") Intent.parseUri(uri.toString(), Intent.URI_INTENT_SCHEME)
            else Intent(Intent.ACTION_VIEW, uri)
            startActivity(intent.addFlags(Intent.FLAG_ACTIVITY_NEW_TASK))
        } catch (e: Exception) {
            Toast.makeText(this, R.string.no_app, Toast.LENGTH_SHORT).show()
        }
        return true
    }

    private fun download(url: String, ua: String?, cd: String?, mime: String?) {
        when {
            url.startsWith("blob:") -> {
                val js = """
                    (function(){var x=new XMLHttpRequest();x.open('GET','$url',true);x.responseType='blob';
                    x.onload=function(){var r=new FileReader();r.onloadend=function(){
                    LucXAndroid.saveBase64(r.result, ${jsStr(guessName(url, cd, mime))}, x.response.type||'application/octet-stream');};
                    r.readAsDataURL(x.response);};x.send();})();
                """.trimIndent()
                web.evaluateJavascript(js, null)
            }
            url.startsWith("data:") -> BlobBridge().saveBase64(url, guessName(url, cd, mime), mime ?: "application/octet-stream")
            else -> {
                try {
                    val name = URLUtil.guessFileName(url, cd, mime)
                    val req = DownloadManager.Request(Uri.parse(url))
                        .addRequestHeader("Cookie", CookieManager.getInstance().getCookie(url) ?: "")
                        .addRequestHeader("User-Agent", ua ?: "")
                        .setMimeType(mime)
                        .setTitle(name)
                        .setNotificationVisibility(DownloadManager.Request.VISIBILITY_VISIBLE_NOTIFY_COMPLETED)
                        .setDestinationInExternalPublicDir(Environment.DIRECTORY_DOWNLOADS, name)
                    (getSystemService(DOWNLOAD_SERVICE) as DownloadManager).enqueue(req)
                    Toast.makeText(this, R.string.downloading, Toast.LENGTH_SHORT).show()
                } catch (e: Exception) {
                    Toast.makeText(this, getString(R.string.download_failed) + ": " + e.message, Toast.LENGTH_LONG).show()
                }
            }
        }
    }

    private fun jsStr(s: String) = "'" + s.replace("\\", "\\\\").replace("'", "\\'") + "'"

    private fun guessName(url: String, cd: String?, mime: String?): String {
        if (!cd.isNullOrEmpty()) return URLUtil.guessFileName(url, cd, mime)
        val ext = MimeTypeMap.getSingleton().getExtensionFromMimeType(mime) ?: "bin"
        return "lucx-${System.currentTimeMillis()}.$ext"
    }

    inner class BlobBridge {
        @JavascriptInterface
        fun saveBase64(dataUrl: String, name: String, mime: String) {
            try {
                val comma = dataUrl.indexOf(',')
                val meta = dataUrl.substring(0, comma)
                val payload = dataUrl.substring(comma + 1)
                val bytes = if (meta.contains(";base64")) Base64.decode(payload, Base64.DEFAULT)
                else Uri.decode(payload).toByteArray()
                val where: String
                if (Build.VERSION.SDK_INT >= 29) {
                    val v = ContentValues().apply {
                        put(MediaStore.Downloads.DISPLAY_NAME, name)
                        put(MediaStore.Downloads.MIME_TYPE, mime)
                    }
                    val uri = contentResolver.insert(MediaStore.Downloads.EXTERNAL_CONTENT_URI, v)!!
                    contentResolver.openOutputStream(uri)!!.use { it.write(bytes) }
                    where = "Downloads/$name"
                } else {
                    val dir = getExternalFilesDir(Environment.DIRECTORY_DOWNLOADS)!!
                    val f = File(dir, name); f.writeBytes(bytes); where = f.absolutePath
                }
                runOnUiThread { Toast.makeText(this@MainActivity, getString(R.string.saved_to, where), Toast.LENGTH_LONG).show() }
            } catch (e: Exception) {
                runOnUiThread { Toast.makeText(this@MainActivity, getString(R.string.download_failed) + ": " + e.message, Toast.LENGTH_LONG).show() }
            }
        }
    }

    @Deprecated("Deprecated in Java")
    override fun onActivityResult(requestCode: Int, resultCode: Int, data: Intent?) {
        if (requestCode == REQ_FILE) {
            val cb = fileCallback; fileCallback = null
            var result = WebChromeClient.FileChooserParams.parseResult(resultCode, data)
            val clip = data?.clipData
            if (resultCode == RESULT_OK && clip != null && clip.itemCount > 1)
                result = Array(clip.itemCount) { clip.getItemAt(it).uri }
            cb?.onReceiveValue(result)
            return
        }
        super.onActivityResult(requestCode, resultCode, data)
    }

    override fun onSaveInstanceState(outState: Bundle) {
        super.onSaveInstanceState(outState)
        if (::web.isInitialized) web.saveState(outState)
    }

    override fun onPause() {
        super.onPause()
        CookieManager.getInstance().flush()
    }

    @Deprecated("Deprecated in Java")
    override fun onBackPressed() {
        if (::web.isInitialized && web.canGoBack()) web.goBack() else super.onBackPressed()
    }

    override fun onCreateOptionsMenu(menu: Menu): Boolean {
        menu.add(0, 1, 0, R.string.reload)
        menu.add(0, 2, 0, R.string.switch_server)
        menu.add(0, 3, 0, R.string.settings)
        return true
    }

    override fun onOptionsItemSelected(item: MenuItem): Boolean {
        when (item.itemId) {
            1 -> web.reload()
            2 -> startActivity(Intent(this, ServersActivity::class.java))
            3 -> startActivity(Intent(this, SettingsActivity::class.java))
            else -> return super.onOptionsItemSelected(item)
        }
        return true
    }

    override fun onResume() {
        super.onResume()
        val cur = Servers.current(this)
        if (::web.isInitialized && cur != null && (cur.id != server?.id || cur.url != server?.url)) {
            server = cur; title = cur.name
            Servers.markOpened(this, cur.id)
            web.clearHistory(); web.loadUrl(cur.url)
        } else if (cur != null) {
            server = cur
        }
    }

    companion object { private const val REQ_FILE = 1001 }
}
