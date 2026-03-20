# Proxy Coin - Security and Compliance

## Traffic Filtering

### Mandatory Blocklist

The proxy engine MUST block traffic to these categories. This is non-negotiable — failure creates criminal liability.

| Category | Source | Implementation |
|----------|--------|---------------|
| CSAM | Project Arachnid, IWF URL List | Hash-based domain matching |
| Malware C2 | Abuse.ch, URLhaus | Domain + IP blocklist |
| Government/Military | Manual curation (.gov, .mil) | Domain suffix matching |
| Financial institutions | FDIC bank list | Domain blocklist |
| Healthcare | HIPAA-covered entities | Domain blocklist |

### Rate-Based Blocking

| Pattern | Threshold | Action |
|---------|-----------|--------|
| Same destination, high volume | >100 req/min to one domain per node | Block + flag |
| Credential stuffing pattern | >10 login endpoints/min | Block + flag |
| DDoS pattern | >1000 req/min total from one node | Disconnect + suspend |

### Domain Filtering Architecture

```
Backend maintains master blocklist (updated daily):
  - Hash-based for privacy (nodes never see the actual blocked domains)
  - Node receives blocklist hash set
  - Before executing request, node checks URL hash against set
  - Unknown domains: execute (allowlist is impractical at scale)
  - Backend also filters server-side before routing to node
```

### Content Policy

| Allowed | Not Allowed |
|---------|-------------|
| Web scraping (public data) | Credential harvesting |
| Price monitoring | DDoS / stress testing |
| Ad verification | Spam / phishing |
| SEO monitoring | Social media manipulation |
| Market research | Circumventing access controls |
| Academic research | Illegal content access |

## Anti-Fraud System

### Threat Model

| Attack | Severity | Mitigation |
|--------|----------|------------|
| **Self-proxying** | High | All traffic originates from backend (nodes can't generate work) |
| **Emulator farms** | High | Play Integrity attestation, hardware fingerprinting |
| **Bandwidth inflation** | High | Server-side byte counting (authoritative) |
| **Sybil attack** | Medium | Device attestation, IP analysis, staking requirement |
| **Traffic replay** | Medium | Unique request IDs, nonce-based verification |
| **Collusion** | Medium | Cross-node traffic pattern analysis |

### Defense Layers

**Layer 1: Server-Side Verification (Primary)**
- Backend is the ONLY source of proxy requests — nodes cannot self-generate work
- Backend counts bytes server-side through the WebSocket
- Backend performs spot-checks: sends known URLs, verifies response integrity
- If client-reported bytes diverge >10% from server-measured bytes → flag

**Layer 2: Device Attestation**
- Google Play Integrity API (replaces SafetyNet)
- Verifies: genuine device, genuine APK, not rooted/modified, no hooks
- Periodic re-attestation (not just at registration)
- Failed attestation → earnings suspended, manual review required

**Layer 3: IP Intelligence**
- Classify IP: residential, datacenter, VPN, proxy
- Datacenter IPs earn 0x (worthless as exit nodes for proxy customers)
- Known VPN/proxy IPs earn 0x
- GeoIP cross-reference: claimed location must match IP geolocation
- ASN analysis: residential ISP expected, hosting provider = suspicious

**Layer 4: Behavioral Analysis**
- New node ramp-up: 0.5x earnings for first 7 days
- Consistency checks: sudden 10x bandwidth spike → manual review
- Pattern detection: multiple nodes from same IP range
- Activity patterns: real phones have sleep cycles, emulators don't

**Layer 5: Economic Deterrents**
- Staking: stake PRXY to earn higher multiplier (slashed if caught cheating)
- Minimum claim: 100 PRXY (makes small-scale fraud unprofitable)
- 7-day earning maturation (fraud detected before payout)
- Trust score: long-term honest behavior rewarded exponentially

### Trust Score Model

```python
trust_score = weighted_sum(
    device_attestation_recency * 0.15,  # Recent attestation = higher
    uptime_consistency       * 0.20,    # Stable daily hours
    verification_pass_rate   * 0.25,    # Spot-check accuracy
    ip_quality               * 0.15,    # Residential > datacenter
    bandwidth_consistency    * 0.15,    # Stable with connection type
    account_age              * 0.10     # Older = more trusted
)

# trust_score range: 0.0 - 1.0
# Earning multiplier: 0.5x (score=0.3) to 2.0x (score=1.0)
# Below 0.3: suspended pending review
# Below 0.1: banned
```

## Play Store Compliance

### Required Disclosures

1. **Data Safety Section**: Declare all data collected
   - Device identifiers (fraud prevention)
   - IP address (geo-routing)
   - Bandwidth usage metrics
   - Wallet address
   - NOT collected: proxied content, personal files, contacts

2. **Foreground Service Justification**
   - Type: `dataSync`
   - Reason: Continuous bandwidth sharing requires persistent connection
   - User consent: explicit opt-in required

3. **Battery Optimization**
   - Must explain why battery optimization exemption is needed
   - Must respect user's battery threshold settings

### Policy Risks

| Risk | Mitigation |
|------|-----------|
| "VPN/Proxy app" category requires review | Submit detailed explanation of business model, not a VPN |
| Background battery drain complaints | Aggressive power management, clear controls |
| Crypto/token features | PRXY is a utility token, not financial product (legal opinion needed) |
| In-app currency | PRXY is not an in-app currency under Play Store terms (it's on-chain) |

### Backup Distribution

If Play Store rejects or removes the app:
1. **Direct APK** from proxycoin.io (with auto-update mechanism)
2. **F-Droid** (open-source app store)
3. **Samsung Galaxy Store**
4. **Amazon Appstore**

## Legal Requirements

### Terms of Service (Key Clauses)

1. User consents to device acting as proxy exit node
2. User acknowledges their IP may be visible to destination servers
3. Prohibited uses: user must not use the network for illegal purposes
4. User responsible for compliance with local laws
5. Company not liable for traffic content flowing through user's device
6. Company reserves right to suspend/ban nodes for policy violations
7. Token rewards are not guaranteed; rates may change
8. No refund policy for staked tokens (except post-slash appeals)

### Privacy Policy (GDPR/CCPA Compliant)

**Data collected**:
- Device ID (hardware-backed, for fraud prevention)
- IP address (for geo-routing and fraud detection)
- Bandwidth usage metrics (for reward calculation)
- Wallet address (for token distribution)
- Device info (model, OS version, for compatibility)

**Data NOT collected**:
- Content of proxied requests/responses
- Personal files, contacts, messages
- Location (beyond IP-based geolocation)
- Browsing history

**Data retention**:
- Usage metrics: 90 days
- Account data: until account deletion + 30 days
- Fraud records: 2 years

**Rights**:
- Right to data export (GDPR Article 20)
- Right to deletion (GDPR Article 17)
- Right to object (GDPR Article 21)
- California: CCPA opt-out of sale (we don't sell data)

### Regulatory Considerations

| Jurisdiction | Concern | Approach |
|-------------|---------|----------|
| **USA (SEC)** | Token may be classified as security | Utility token argument; consider excluding US from token sale |
| **USA (FinCEN)** | Money transmitter classification | Tokens are earned, not purchased through us |
| **EU (MiCA)** | Crypto-asset regulation | Utility token under MiCA Article 4 |
| **China** | Crypto ban | Geo-block CN IP addresses |
| **India** | Crypto tax complexity | User responsible for tax compliance |

**Recommended**: Obtain legal opinion from crypto-specialized law firm before token launch.

### Geographic Restrictions

Block from participating (as nodes):
- Countries with proxy/VPN bans (China, Russia, Iran, etc.)
- OFAC-sanctioned countries
- Countries where crypto is banned

## Application Security

### Android App Security

| Measure | Implementation |
|---------|---------------|
| Certificate pinning | OkHttp CertificatePinner for all API calls |
| Key storage | Android Keystore (hardware-backed when available) |
| Data encryption | EncryptedSharedPreferences for sensitive data |
| Root detection | Check for su, Magisk, system modifications |
| Emulator detection | Build props, sensor count, telephony checks |
| Code obfuscation | R8/ProGuard with aggressive optimization |
| Log stripping | Remove all log calls in release builds |
| Memory safety | Zero-out sensitive byte arrays after use |
| Tamper detection | APK signature verification at runtime |

### Backend Security

| Measure | Implementation |
|---------|---------------|
| TLS 1.3 | All connections encrypted |
| JWT rotation | Short-lived tokens (15 min), refresh tokens (7 days) |
| Rate limiting | Per-IP and per-API-key limits |
| Input validation | Strict schema validation on all endpoints |
| SQL injection | Parameterized queries (pgx), no raw SQL interpolation |
| CORS | Strict origin allowlist |
| Secrets management | Infisical (or Vault) for all secrets |
| Dependency scanning | Snyk/Dependabot for Go dependencies |
| Container scanning | Trivy for Docker image vulnerabilities |

### Smart Contract Security

| Measure | Implementation |
|---------|---------------|
| Audit | Professional audit before mainnet (Trail of Bits, OpenZeppelin) |
| Testing | >95% code coverage, fuzz testing (1000+ runs) |
| Formal verification | Critical paths (claim, slash) |
| Access control | OpenZeppelin AccessControl, not ad-hoc |
| Reentrancy | Checks-effects-interactions pattern |
| Admin multisig | Gnosis Safe (3-of-5) |
| Timelock | 48h delay on admin operations |
| Emergency pause | Pausable for critical vulnerabilities |
| Bug bounty | Immunefi program post-launch |

## Incident Response

### Severity Levels

| Level | Definition | Response Time | Example |
|-------|-----------|--------------|---------|
| P0 | Token theft, data breach | 15 min | Smart contract exploit |
| P1 | Service outage, fraud spike | 1 hour | Backend crash, coordinated fraud |
| P2 | Degraded service | 4 hours | High latency, partial outage |
| P3 | Minor issue | 24 hours | UI bug, minor metering discrepancy |

### Response Procedures

**P0 (Token/Security)**:
1. Activate contract pause (if applicable)
2. Notify team via PagerDuty
3. Assess scope and impact
4. Publish incident disclosure within 24h
5. Post-mortem within 72h

**Smart Contract Emergency**:
1. Call `pause()` on affected contract
2. Snapshot all state
3. Prepare fix or migration
4. Governance vote for resolution (if time permits)
5. Deploy fix, unpause
