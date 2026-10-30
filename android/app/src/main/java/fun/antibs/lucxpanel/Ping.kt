package `fun`.antibs.lucxpanel

import android.os.Handler
import android.os.Looper
import java.net.HttpURLConnection
import java.net.URL
import java.security.SecureRandom
import java.security.cert.X509Certificate
import java.util.concurrent.Executors
import javax.net.ssl.HttpsURLConnection
import javax.net.ssl.SSLContext
import javax.net.ssl.TrustManager
import javax.net.ssl.X509TrustManager

/** ms >= 0 when the panel answered with any HTTP status, -1 when unreachable. */
object Ping {
    private val pool = Executors.newFixedThreadPool(4)
    private val main = Handler(Looper.getMainLooper())

    private val insecureFactory by lazy {
        val tm = arrayOf<TrustManager>(object : X509TrustManager {
            override fun checkClientTrusted(c: Array<X509Certificate>, a: String) {}
            override fun checkServerTrusted(c: Array<X509Certificate>, a: String) {}
            override fun getAcceptedIssuers(): Array<X509Certificate> = arrayOf()
        })
        SSLContext.getInstance("TLS").apply { init(null, tm, SecureRandom()) }.socketFactory
    }

    fun check(s: Server, cb: (Long) -> Unit) {
        pool.execute {
            val r = try {
                val start = System.nanoTime()
                val c = URL(s.url).openConnection() as HttpURLConnection
                if (c is HttpsURLConnection && s.allowInsecure) {
                    c.sslSocketFactory = insecureFactory
                    c.setHostnameVerifier { _, _ -> true }
                }
                c.connectTimeout = 5000
                c.readTimeout = 5000
                c.instanceFollowRedirects = false
                c.requestMethod = "HEAD"
                c.responseCode
                val ms = (System.nanoTime() - start) / 1_000_000
                c.disconnect()
                ms
            } catch (e: Exception) {
                -1L
            }
            main.post { cb(r) }
        }
    }
}
