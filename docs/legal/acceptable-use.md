# Proxy Coin Acceptable Use Policy

**Effective Date**: [DATE]
**Last Updated**: [DATE]

> **Legal Notice**: This document is a skeleton template for review by qualified legal counsel before publication. Do not publish without attorney review.

---

## Overview

This Acceptable Use Policy ("AUP") defines the rules governing use of the Proxy Coin network. It applies to both Node Operators and API Customers. Violations may result in immediate suspension or termination.

---

## 1. Prohibited Content Categories

The following categories of content may not be accessed, transmitted, or distributed through the Proxy Coin network:

### 1.1 Illegal Content
- Child sexual abuse material (CSAM) — zero tolerance; immediate ban and law enforcement notification
- Content that facilitates terrorism, violence, or genocide
- Content that violates export controls or sanctions regulations (OFAC, EU, etc.)
- Stolen credentials, financial fraud instruments, or identity theft tools

### 1.2 Malicious Activity
- Distribution of malware, ransomware, spyware, or botnets
- Command-and-control (C2) communications for any malicious purpose
- Network attacks: DDoS, port scanning, brute force, vulnerability scanning without authorization
- Phishing pages, credential harvesting, or social engineering campaigns

### 1.3 Unauthorized Access
- Accessing computer systems without authorization
- Circumventing security controls, firewalls, or authentication mechanisms
- Exploiting vulnerabilities in third-party systems

---

## 2. Traffic Filtering

Proxy Coin implements the following technical controls to enforce this AUP:

| Filter Type | Method | Coverage |
|-------------|--------|----------|
| CSAM detection | PhotoDNA hash matching | All image/video traffic |
| Known malware domains | DNSBL lookups (multiple feeds) | All DNS requests |
| Tor exit node targeting | IP blocklist | Destination IPs |
| Botnet C2 | Threat intelligence feeds | Destination IPs and domains |
| OFAC-sanctioned countries | IP geolocation | Destination IPs |

Node Operators acknowledge that perfect filtering is technically infeasible; Proxy Coin makes reasonable commercial efforts to block prohibited content.

---

## 3. API Customer Restrictions

API Customers using the proxy network agree that they will not use it to:

- **Scrape** websites in violation of those websites' Terms of Service
- **Circumvent** geo-restrictions on streaming services (Netflix, Hulu, etc.) in violation of those services' licensing agreements
- **Generate** artificial engagement (fake views, clicks, likes, reviews)
- **Send** unsolicited commercial email (spam)
- **Test** security vulnerabilities of systems without explicit written authorization from the system owner
- **Mine** cryptocurrency using proxy nodes

---

## 4. Node Operator Restrictions

Node Operators agree that their devices:

- Are **not** located in a datacenter, VPS, or cloud environment
- Are **not** operating behind a VPN, proxy, or anonymizing service
- Have a **genuine residential or mobile internet connection**
- Are **personal devices** they own or have explicit authorization to use for this purpose

Node Operators also agree not to:
- Run multiple instances of the app on the same physical device
- Artificially inflate bandwidth metrics using synthetic traffic
- Attempt to circumvent the Android Play Integrity check
- Operate nodes from jurisdictions where participation is prohibited by local law

---

## 5. Enforcement

### 5.1 Detection Methods

Proxy Coin uses the following to detect violations:
- Real-time IP intelligence and DNSBL filtering
- Behavioral analysis (bandwidth patterns, sleep cycles, request rates)
- Android Play Integrity API verification
- Machine learning anomaly detection
- User reports

### 5.2 Consequences

| Violation Severity | Consequence |
|-------------------|-------------|
| Low (first offense, minor) | Warning; reward reduction |
| Medium (pattern violations) | Temporary suspension (7-30 days) |
| High (intentional abuse) | Permanent ban; forfeiture of pending rewards |
| Critical (CSAM, terrorism, attacks) | Immediate permanent ban; law enforcement notification |

### 5.3 Appeals

To appeal an enforcement action, contact: abuse@proxycoin.io within 14 days of the action. Include your node ID or customer ID and a description of why you believe the action was in error.

---

## 6. Reporting Violations

To report a violation of this AUP, contact: abuse@proxycoin.io

For CSAM or serious illegal content, contact NCMEC CyberTipline: cybertipline.org

---

## 7. DMCA / Copyright

To submit a copyright takedown notice, contact: dmca@proxycoin.io

Include:
- Identification of the copyrighted work
- Identification of the infringing content/URL
- Your contact information
- A statement under penalty of perjury that you are the copyright owner or authorized agent

---

## 8. Changes

Proxy Coin may update this AUP at any time. Continued use of the Service after changes take effect constitutes acceptance. Material changes will be notified in accordance with the Terms of Service.

---

## 9. Contact

**Abuse reports**: abuse@proxycoin.io
**DMCA**: dmca@proxycoin.io
**Security**: security@proxycoin.io
