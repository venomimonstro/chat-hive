# CHAT API and Realtime Rules

## HTTP conventions

- Prefix public application APIs with `/api/v1`.
- JSON request/response bodies use UTF-8 and `snake_case` for wire fields.
- Every response carries a request ID in headers; errors return a stable machine code plus human-safe message.
- Do not expose stack traces or internal SQL errors to clients.
- State-changing requests that can be retried must support idempotency keys or stable client operation IDs.
- Unbounded collections use cursor pagination; no deep OFFSET pagination.

### Error envelope

```json
{
  "error": {
    "code": "invalid_request",
    "message": "Request could not be processed",
    "request_id": "..."
  }
}
```

## Authentication and authorization

Authentication identifies the session. Authorization is separate and evaluated for every protected object.

Handlers must never infer access from possession of an object ID. Chat/message/group/admin access is verified server-side on each action.

## Realtime protocol

WebSocket connection lifecycle:

`CONNECT -> AUTH -> READY`

Initial client events:
- `send_message`
- `read`
- `typing`
- `reaction`
- `sync`

Initial server events:
- `ack`
- `message_created`
- `message_updated`
- `message_deleted`
- `read_updated`
- `presence`
- `sync_result`
- `error`

Every frame has protocol version, event type, request/event ID and payload. Unknown versions/types fail closed.

## Reliability

A successful send acknowledgement is only emitted after durable persistence of the message. Consumers of NATS events must be idempotent because delivery may repeat.

## Versioning

Breaking wire changes require a new API/protocol version. Database implementation details are never exposed as public contracts.
