# Proxy Coin API Reference

Base URL: `https://api.proxycoin.io/v1`

All responses are JSON. Errors follow the format `{"error": "<message>"}`.

---

## Authentication

Proxy Coin uses two authentication schemes:

| Scheme | Header | Used For |
|--------|--------|----------|
| Bearer JWT | `Authorization: Bearer <token>` | Account management endpoints |
| API Key | `Authorization: Bearer prxy_live_<key>` or `X-API-Key: prxy_live_<key>` | Proxy request endpoints |

### Rate Limits

| Plan | Daily Requests | Included GB | Monthly Price |
|------|----------------|-------------|---------------|
| Free | 100 | 1 GB | $0 |
| Starter | 10,000 | 100 GB | $49 |
| Pro | 100,000 | 1,000 GB | $199 |
| Enterprise | 1,000,000 | 10,000 GB | Contact sales |

Rate limit headers are returned on every response:

```
X-RateLimit-Limit: 10000
X-RateLimit-Remaining: 9876
X-RateLimit-Reset: 2026-03-22T00:00:00Z
```

---

## Auth Endpoints

### POST /v1/auth/register

Register a new customer account.

**Request**

```json
{
  "email": "user@example.com",
  "password": "s3cr3tP@ssw0rd"
}
```

**Response** `201 Created`

```json
{
  "customer": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "email": "user@example.com",
    "tier": "free",
    "created_at": "2026-03-21T10:00:00Z"
  },
  "access_token": "<ACCESS_TOKEN>",
  "refresh_token": "<REFRESH_TOKEN>"
}
```

**Errors**

| Status | Description |
|--------|-------------|
| 400 | Missing email or password |
| 409 | Email already registered |

---

### POST /v1/auth/login

Authenticate and receive a JWT pair.

**Request**

```json
{
  "email": "user@example.com",
  "password": "s3cr3tP@ssw0rd"
}
```

**Response** `200 OK`

Same shape as `/v1/auth/register`.

**Errors**

| Status | Description |
|--------|-------------|
| 401 | Invalid credentials |

---

### POST /v1/auth/refresh

Exchange a refresh token for a new access + refresh token pair.

**Request**

```json
{
  "refresh_token": "<REFRESH_TOKEN>"
}
```

**Response** `200 OK`

```json
{
  "access_token": "<ACCESS_TOKEN>",
  "refresh_token": "<REFRESH_TOKEN>"
}
```

---

### GET /v1/auth/apikey

Generate (or retrieve) an API key for programmatic access.

**Auth**: Bearer JWT required.

**Response** `200 OK`

```json
{
  "api_key_id": "a1b2c3d4",
  "api_key": "<PRXY_LIVE_API_KEY>",
  "created_at": "2026-03-21T10:00:00Z"
}
```

> **Important**: The `api_key` plaintext value is only returned once. Store it securely.

---

### POST /v1/auth/apikey/rotate

Revoke the current API key and issue a new one.

**Auth**: Bearer JWT required.

**Response** `200 OK`

Same shape as `GET /v1/auth/apikey`.

---

## Proxy Endpoints

### POST /v1/proxy

Execute a single proxy request through the Proxy Coin network.

**Auth**: API Key required.

**Request**

```json
{
  "url": "https://httpbin.org/ip",
  "method": "GET",
  "headers": {
    "Accept": "application/json"
  },
  "body": null,
  "geo_country": "US",
  "geo_region": "California",
  "session_id": "my-sticky-session-123",
  "timeout_ms": 30000
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `url` | string | Yes | Target URL to proxy |
| `method` | string | No | HTTP method (default: GET) |
| `headers` | object | No | Request headers to forward |
| `body` | bytes | No | Request body (base64 encoded) |
| `geo_country` | string | No | ISO 3166-1 alpha-2 country code for geo-targeting |
| `geo_region` | string | No | Region/state name for geo-targeting |
| `session_id` | string | No | Session ID for sticky session routing |
| `timeout_ms` | int | No | Request timeout in milliseconds (default: 30000) |

**Response** `200 OK`

```json
{
  "request_id": "550e8400-e29b-41d4-a716-446655440001",
  "status_code": 200,
  "headers": {
    "Content-Type": "application/json"
  },
  "body": "eyJvcmlnaW4iOiAiMTAzLjEyMy40NS42NyJ9",
  "node_country": "US",
  "node_region": "California",
  "node_ip": "103.123.45.67",
  "total_ms": 842,
  "proxy_ms": 610,
  "bytes_transferred": 312
}
```

> Body is base64-encoded. Decode it to get the raw response bytes.

**Errors**

| Status | Description |
|--------|-------------|
| 401 | Invalid or missing API key |
| 429 | Rate limit exceeded |
| 502 | Node error or no available nodes |

---

### POST /v1/proxy/batch

Execute up to 50 proxy requests concurrently.

**Auth**: API Key required.

**Request**: Array of proxy request objects (same schema as `/v1/proxy`).

```json
[
  {"url": "https://httpbin.org/ip", "geo_country": "US"},
  {"url": "https://httpbin.org/ip", "geo_country": "GB"}
]
```

**Response** `200 OK`

```json
{
  "results": [
    {
      "request_id": "...",
      "status_code": 200,
      ...
    },
    {
      "error": "proxy: no available nodes matching request criteria"
    }
  ]
}
```

Results are returned in the same order as the input array.

---

## Reward Endpoints

### GET /v1/rewards/proof

Get the Merkle proof for claiming PRXY rewards on-chain.

**Query Parameters**

| Parameter | Required | Description |
|-----------|----------|-------------|
| `wallet` | Yes | Ethereum wallet address (with or without `0x` prefix) |

**Response** `200 OK`

```json
{
  "wallet_address": "0xaBcD1234...",
  "leaf_index": 7,
  "cumulative_amount_wei": "50000000000000000000",
  "proof": [
    "0xa1b2c3d4...",
    "0xe5f6g7h8..."
  ],
  "merkle_root": "0x1234abcd...",
  "merkle_root_id": 42
}
```

**Errors**

| Status | Description |
|--------|-------------|
| 400 | Missing `wallet` parameter |
| 404 | No pending rewards for wallet |

---

### GET /v1/rewards/history

Get earnings history for a node.

**Query Parameters**

| Parameter | Required | Description |
|-----------|----------|-------------|
| `node_id` | Yes | Node UUID |
| `from` | No | Start timestamp (ISO 8601) |
| `to` | No | End timestamp (ISO 8601) |

**Response** `200 OK`

```json
{
  "node_id": "550e8400-e29b-41d4-a716-446655440000",
  "summary": {
    "today_earnings": 2.5,
    "all_time_earnings": 143.7,
    "pending_claim": 80.2,
    "trust_score": 0.87
  },
  "records": [
    {
      "node_id": "550e8400-e29b-41d4-a716-446655440000",
      "epoch": "2026-03-21T10:00:00Z",
      "bytes_proxied": 1073741824,
      "requests_served": 1205,
      "success_rate": 0.98,
      "avg_latency_ms": 245,
      "base_reward": 10.74,
      "multiplier": 1.82,
      "final_reward": 19.55
    }
  ]
}
```

---

## Status Endpoint

### GET /v1/status

Get aggregate network statistics and geographic node distribution.

**Auth**: None required.

**Response** `200 OK`

```json
{
  "total_nodes": 15243,
  "active_nodes": 11872,
  "countries": 47,
  "distribution": {
    "US": {
      "country": "US",
      "total": 3451,
      "regions": {
        "California": 892,
        "Texas": 647,
        "New York": 412
      }
    },
    "GB": {
      "country": "GB",
      "total": 1203,
      "regions": {
        "England": 987,
        "Scotland": 156
      }
    }
  }
}
```

---

## Usage Endpoint

### GET /v1/usage

Get current usage summary for the authenticated customer.

**Auth**: Bearer JWT required.

**Response** `200 OK`

```json
{
  "customer_id": "550e8400-e29b-41d4-a716-446655440000",
  "period_start": "0001-01-01T00:00:00Z",
  "period_end": "2026-03-21T10:00:00Z",
  "bytes_used": 5368709120,
  "requests": 4213
}
```

---

## Error Reference

All error responses use the following JSON format:

```json
{
  "error": "human-readable error description"
}
```

| HTTP Status | Meaning |
|-------------|---------|
| 400 Bad Request | Invalid request body or missing required parameters |
| 401 Unauthorized | Missing, expired, or invalid authentication |
| 403 Forbidden | Valid auth but insufficient permissions |
| 404 Not Found | Resource does not exist |
| 409 Conflict | Duplicate resource (e.g. email already registered) |
| 429 Too Many Requests | Rate limit exceeded |
| 500 Internal Server Error | Unexpected server error |
| 502 Bad Gateway | Upstream node or backend error |
| 503 Service Unavailable | Proxy network temporarily unavailable |

---

## Versioning

The API is versioned via the URL path (`/v1/`). Breaking changes will be
introduced in `/v2/` with a deprecation period for `/v1/`.

## SDKs

- **Node.js / TypeScript**: coming soon
- **Python**: coming soon
- **Go**: use the `backend/` module directly (internal use)
