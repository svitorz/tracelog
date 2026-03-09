# TraceLog HTTP API

## Overview

The TraceLog HTTP API provides endpoints for event ingestion and health checks. The API accepts JSON payloads and returns JSON responses.

**Base URL**: `http://localhost:8080` (configurable via `HTTP_PORT` in `.env`)

**Content-Type**: `application/json`

---

## Endpoints

### Health Check

**GET** `/health`

Simple health check endpoint to verify the server is running.

**Response**:
```json
{
  "status": "ok"
}
```

**Status Codes**:
- `200 OK`: Server is healthy

---

### Register Technical Event

**POST** `/events`

Ingests a technical event into the TraceLog system. This is the primary endpoint used by applications to send events for monitoring and analysis.

#### Request Headers

```
Content-Type: application/json
```

#### Request Body

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `message` | string | **Yes** | Human-readable description of the event |
| `occurred_at` | string | **Yes** | ISO 8601 timestamp when the event occurred (RFC3339 format) |
| `severity` | string | **Yes** | Event severity level: `debug`, `info`, `warn`, `error`, `fatal` |
| `application_id` | string | **Yes** | UUID of the monitored application |
| `source_id` | string | **Yes** | UUID of the event source |
| `environment_id` | string | **Yes** | UUID of the environment |
| `external_id` | string | No | Client-provided identifier for the event |
| `category` | string | No | Event category: `application`, `system`, `security`, `audit` (default: `application`) |
| `payload` | object | No | Arbitrary JSON metadata with context-specific information |
| `trace_id` | string | No | Distributed tracing identifier for correlation |
| `correlation_id` | string | No | Business transaction identifier |

#### Example Request

```bash
curl -X POST http://localhost:8080/events \
  -H "Content-Type: application/json" \
  -d '{
    "message": "Database connection timeout",
    "occurred_at": "2026-03-09T14:30:00Z",
    "severity": "error",
    "category": "system",
    "application_id": "550e8400-e29b-41d4-a716-446655440000",
    "source_id": "660e8400-e29b-41d4-a716-446655440001",
    "environment_id": "770e8400-e29b-41d4-a716-446655440002",
    "external_id": "app-error-12345",
    "trace_id": "abc123xyz789",
    "correlation_id": "req-user-profile-456",
    "payload": {
      "timeout_ms": 5000,
      "pool_size": 20,
      "active_connections": 20,
      "query": "SELECT * FROM users WHERE id = $1"
    }
  }'
```

#### Success Response

**Status Code**: `201 Created`

```json
{
  "id": "880e8400-e29b-41d4-a716-446655440003",
  "external_id": "app-error-12345",
  "received_at": "2026-03-09T14:30:01.234Z",
  "created_at": "2026-03-09T14:30:01.234Z"
}
```

**Response Fields**:

| Field | Type | Description |
|-------|------|-------------|
| `id` | string | TraceLog-generated UUID for the event |
| `external_id` | string | Client-provided identifier (if supplied) |
| `received_at` | string | Timestamp when TraceLog received the event |
| `created_at` | string | Timestamp when the event was persisted in the database |

#### Error Responses

All error responses follow this format:

```json
{
  "error": "error description"
}
```

**Status Codes**:

| Code | Description | Example Scenarios |
|------|-------------|-------------------|
| `400 Bad Request` | Invalid request payload | Missing required fields, invalid JSON, malformed timestamp, invalid UUIDs |
| `405 Method Not Allowed` | HTTP method not supported | Using GET, PUT, DELETE instead of POST |
| `500 Internal Server Error` | Server-side error | Database connection failure, unexpected system error |

#### Error Examples

**Missing Required Field**:
```json
{
  "error": "message is required"
}
```

**Invalid JSON**:
```json
{
  "error": "invalid JSON payload: unexpected EOF"
}
```

**Invalid Timestamp Format**:
```json
{
  "error": "invalid occurred_at: unsupported format (use RFC3339)"
}
```

**Invalid Severity**:
```json
{
  "error": "invalid severity: must be one of debug, info, warn, error, fatal"
}
```

**Invalid UUID**:
```json
{
  "error": "invalid application_id: must be a valid UUID"
}
```

---

## Field Details

### message

A clear, human-readable description of what happened.

**Guidelines**:
- Be descriptive and specific
- Include key context in the message itself
- Use additional details in `payload` for structured data
- Keep messages consistent for similar events

**Examples**:
```json
"message": "User authentication failed"
"message": "Database connection pool exhausted"
"message": "Payment processing completed successfully"
"message": "Invalid API request: missing required parameter 'email'"
```

### occurred_at

The timestamp when the event actually occurred in the source system.

**Format**: ISO 8601 / RFC3339

**Supported Formats**:
- `2026-03-09T14:30:00Z` (recommended)
- `2026-03-09T14:30:00.123Z` (with milliseconds)
- `2026-03-09T14:30:00-03:00` (with timezone)
- `2026-03-09T14:30:00` (assumes UTC)

**Important**:
- Must not be in the future
- Should be as close to the actual event time as possible
- TraceLog will set `received_at` independently

### severity

Indicates the importance and urgency of the event.

**Values**:

| Value | When to Use | Examples |
|-------|-------------|----------|
| `debug` | Detailed diagnostic information for troubleshooting | Variable values, method calls, state changes |
| `info` | Normal operational events | Successful operations, routine status updates |
| `warn` | Potential issues that don't prevent operation | Deprecated feature usage, retry attempts, slow queries |
| `error` | Errors that allow continued operation | Failed requests, exceptions caught and handled |
| `fatal` | Critical errors causing service termination | Unrecoverable errors, out of memory, panic |

**Default**: If not specified, defaults to `info`

### category

Classifies the event by functional domain.

**Values**:

| Value | Description | Examples |
|-------|-------------|----------|
| `application` | Business logic events (default) | Order created, user registered, report generated |
| `system` | Infrastructure and runtime events | Database connection issues, cache failures, memory alerts |
| `security` | Security-related events | Login failures, unauthorized access, suspicious activity |
| `audit` | Compliance and audit trail | Data modifications, permission changes, admin actions |

**Default**: `application`

### payload

Flexible JSON object for arbitrary context-specific metadata.

**Guidelines**:
- Use for structured data that varies by event type
- Keep consistent field names across similar events
- Avoid deeply nested structures
- Don't duplicate data already in other fields

**Examples**:

**Error Context**:
```json
"payload": {
  "error_code": "DB_TIMEOUT",
  "timeout_ms": 5000,
  "retry_count": 3,
  "query": "SELECT * FROM orders WHERE id = $1"
}
```

**User Action**:
```json
"payload": {
  "user_id": "12345",
  "action": "profile_update",
  "fields_changed": ["email", "phone"],
  "ip_address": "192.168.1.100"
}
```

**Performance Metrics**:
```json
"payload": {
  "response_time_ms": 1250,
  "memory_used_mb": 512,
  "cpu_percent": 45.2
}
```

### trace_id

Distributed tracing identifier for correlating events across multiple services.

**Purpose**: Link events that are part of the same request flow across microservices.

**Example Flow**:
```
API Gateway (trace_id: "abc123") 
  → Auth Service (trace_id: "abc123")
  → User Service (trace_id: "abc123")
  → Database (trace_id: "abc123")
```

Query all events with the same `trace_id` to see the complete request path.

### correlation_id

Business transaction identifier for grouping related events.

**Purpose**: Link events that belong to the same business operation, even if they occur at different times.

**Example**:
- User starts checkout: `correlation_id: "order-12345"`
- Payment processed: `correlation_id: "order-12345"`
- Inventory updated: `correlation_id: "order-12345"`
- Email sent: `correlation_id: "order-12345"`

---

## Client Integration Examples

### Go

```go
package main

import (
    "bytes"
    "encoding/json"
    "net/http"
    "time"
)

type TraceLogClient struct {
    baseURL string
}

func (c *TraceLogClient) SendEvent(event map[string]any) error {
    event["occurred_at"] = time.Now().UTC().Format(time.RFC3339)
    
    payload, err := json.Marshal(event)
    if err != nil {
        return err
    }
    
    resp, err := http.Post(c.baseURL+"/events", "application/json", bytes.NewBuffer(payload))
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    
    return nil
}
```

### Python

```python
import requests
from datetime import datetime, timezone

class TraceLogClient:
    def __init__(self, base_url):
        self.base_url = base_url
    
    def send_event(self, event):
        event['occurred_at'] = datetime.now(timezone.utc).isoformat()
        
        response = requests.post(
            f"{self.base_url}/events",
            json=event,
            headers={"Content-Type": "application/json"}
        )
        response.raise_for_status()
        return response.json()

# Usage
client = TraceLogClient("http://localhost:8080")
client.send_event({
    "message": "User login successful",
    "severity": "info",
    "application_id": "550e8400-e29b-41d4-a716-446655440000",
    "source_id": "660e8400-e29b-41d4-a716-446655440001",
    "environment_id": "770e8400-e29b-41d4-a716-446655440002"
})
```

### Node.js

```javascript
const axios = require('axios');

class TraceLogClient {
  constructor(baseURL) {
    this.baseURL = baseURL;
  }

  async sendEvent(event) {
    event.occurred_at = new Date().toISOString();
    
    const response = await axios.post(`${this.baseURL}/events`, event, {
      headers: { 'Content-Type': 'application/json' }
    });
    
    return response.data;
  }
}

// Usage
const client = new TraceLogClient('http://localhost:8080');
client.sendEvent({
  message: 'Database query executed',
  severity: 'info',
  application_id: '550e8400-e29b-41d4-a716-446655440000',
  source_id: '660e8400-e29b-41d4-a716-446655440001',
  environment_id: '770e8400-e29b-41d4-a716-446655440002',
  payload: { query_time_ms: 45 }
});
```

### cURL

```bash
# Basic event
curl -X POST http://localhost:8080/events \
  -H "Content-Type: application/json" \
  -d '{
    "message": "Test event",
    "occurred_at": "2026-03-09T14:30:00Z",
    "severity": "info",
    "application_id": "550e8400-e29b-41d4-a716-446655440000",
    "source_id": "660e8400-e29b-41d4-a716-446655440001",
    "environment_id": "770e8400-e29b-41d4-a716-446655440002"
  }'

# With full metadata
curl -X POST http://localhost:8080/events \
  -H "Content-Type: application/json" \
  -d '{
    "message": "Payment processing failed",
    "occurred_at": "2026-03-09T14:30:00Z",
    "severity": "error",
    "category": "application",
    "application_id": "550e8400-e29b-41d4-a716-446655440000",
    "source_id": "660e8400-e29b-41d4-a716-446655440001",
    "environment_id": "770e8400-e29b-41d4-a716-446655440002",
    "external_id": "payment-error-789",
    "trace_id": "trace-abc123",
    "correlation_id": "order-456",
    "payload": {
      "amount": 99.99,
      "currency": "USD",
      "error_code": "INSUFFICIENT_FUNDS",
      "user_id": "12345"
    }
  }'
```

---

## Rate Limiting

**Current Status**: No rate limiting implemented in MVP

**Future**: Rate limiting will be added based on usage patterns and requirements.

---

## Authentication

**Current Status**: No authentication required in MVP

**Future**: API key or JWT-based authentication will be added for production use.

---

## Versioning

**Current Version**: v1 (implicit)

**Future**: API versioning will be introduced as `/v1/events` when breaking changes are needed.

---

## Best Practices

1. **Always include context in payload**: Add relevant metadata to help with debugging
2. **Use consistent severity levels**: Don't mark routine operations as errors
3. **Set occurred_at accurately**: Use the actual event time, not the current time
4. **Use trace_id for request flows**: Correlate events across services
5. **Keep messages readable**: Write messages for humans, use payload for structured data
6. **Handle errors gracefully**: Don't let TraceLog ingestion failures break your application

---

**Last Updated**: March 2026  
**API Version**: 1.0 (MVP)
