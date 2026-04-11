package com.proxycoin.app.security

import okhttp3.CertificatePinner

/**
 * OkHttp [CertificatePinner] configuration for the ProxyCoin API domain.
 *
 * Pins are SHA-256 hashes of the Subject Public Key Info (SPKI) from the TLS certificate chain.
 * Two pins are recommended: one for the leaf/intermediate certificate currently in use and one
 * backup pin for the next rotation cycle.
 *
 * To generate a pin for a live domain, run:
 *   openssl s_client -connect api.proxycoin.io:443 -servername api.proxycoin.io 2>/dev/null \
 *     | openssl x509 -pubkey -noout \
 *     | openssl pkey -pubin -outform DER \
 *     | openssl dgst -sha256 -binary \
 *     | base64
 *
 * Or use OkHttp's built-in logging approach:
 *   Enable CertificatePinner with `sha256/AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=`
 *   and check the exception message; it lists the actual pins.
 *
 * WARNING: Incorrect pins will break all HTTPS calls. Verify pins against the live certificate
 * before shipping. The placeholder values below MUST be replaced before production release.
 */
object ProxyCoinCertificatePinner {

    /**
     * The API hostname. Adjust if the backend is served from a subdomain.
     */
    private const val API_HOST = "api.proxycoin.io"

    /**
     * Returns a configured [CertificatePinner] instance.
     *
     * In debug builds, certificate pinning is deliberately loose (no pins) so developers
     * can inspect traffic with a proxy (e.g., Charles Proxy). Tighten this for QA / release.
     *
     * Production release: replace the placeholder SHA-256 strings below with the real pins.
     */
    fun build(isDebug: Boolean = false): CertificatePinner {
        if (isDebug) {
            // No pins in debug — allows traffic inspection with MITM proxies.
            return CertificatePinner.DEFAULT
        }

        return CertificatePinner.Builder()
            // Primary pin — leaf or intermediate certificate currently deployed.
            // Replace with: sha256/<base64-encoded SPKI hash>
            .add(API_HOST, "sha256/AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=") // REPLACE BEFORE RELEASE
            // Backup pin — next rotation certificate (pre-generated).
            // Replace with: sha256/<base64-encoded SPKI hash of backup cert>
            .add(API_HOST, "sha256/BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB=") // REPLACE BEFORE RELEASE
            .build()
    }
}
