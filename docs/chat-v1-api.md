# 单聊文本消息 V1

## 范围

V1 支持已互为正常好友的用户进行单聊文本聊天，包括 WebSocket 连接、发送和实时接收、历史消息分页、按会话增量同步。消息以 MySQL 为准；离线消息通过同步接口补拉。

暂不支持群聊、图片/语音/文件、消息撤回、送达回执、已读回执和多实例 WebSocket 转发。每个会话有独立递增的 `sequence`；消息 `id` 仍是全局主键，不应用它判断会话内是否漏消息。

## 认证与建立连接

### 申请一次性 WebSocket 票据

**POST** `/ws/ticket`

使用现有登录 Token，通过 `Authorization: Bearer <token>` 认证。请求无参数。票据有效期为 60 秒、只能使用一次，并绑定申请时的 `Origin`。

成功响应：

```json
{
  "code": 200,
  "message": "操作成功",
  "data": {
    "ticket": "一次性票据",
    "expires_in": 60
  },
  "success": true
}
```

### 建立 WebSocket 连接

```text
ws(s)://<host>/ws?ticket=<ticket>
```

票据过期、已使用或 `Origin` 与申请时不一致时，连接会被拒绝。生产环境应使用 `wss://`。

服务端每 25 秒发送 WebSocket Ping；客户端需正常响应 Pong。连接空闲超过约 60 秒会被断开，客户端应重连并调用增量同步接口。

## WebSocket 协议

### 发送单聊文本

客户端发送：

```json
{
  "type": "message.send",
  "request_id": "request-uuid",
  "data": {
    "target_id": 456,
    "client_msg_id": "message-uuid",
    "message_type": 1,
    "content": "你好"
  }
}
```

字段说明：

| 字段 | 类型 | 必填 | 说明 |
|---|---|---:|---|
| 外层 `type` | string | 是 | 固定为 `message.send` |
| `request_id` | string | 否 | 请求关联 ID，服务端 ACK/错误会原样返回 |
| `data.target_id` | uint | 是 | 接收方用户 ID |
| `data.client_msg_id` | string | 是 | 客户端生成的消息 UUID，用于重试幂等，最多 64 字符 |
| `data.message_type` | uint8 | 是 | V1 固定为 `1`（文本） |
| `data.content` | string | 是 | 非空文本，最多 4000 个 Unicode 字符 |

服务端校验发送者和接收者均为可用用户、双方仍是正常好友且没有任一方向拉黑。消息和双方会话摘要/未读数在同一数据库事务中保存；只有事务成功后才推送。

### 发送成功 ACK

```json
{
  "type": "message.ack",
  "request_id": "request-uuid",
  "data": {
    "id": 789,
    "conv_key": "1_123_456",
    "sequence": 12,
    "sender_id": 123,
    "receiver_id": 456,
    "message_type": 1,
    "content": "你好",
    "sender_nickname": "Alice",
    "sender_avatar": "",
    "status": 1,
    "client_msg_id": "message-uuid",
    "created_at": "2026-11-22T18:00:00+08:00"
  }
}
```

`message.ack` 表示服务端已持久化，不表示接收方已收到或已读。使用相同 `client_msg_id` 和相同内容重试会返回原消息，不会再次增加未读数。

### 接收新消息

接收方在线时收到：

```json
{
  "type": "message.new",
  "data": {
    "id": 789,
    "conv_key": "1_123_456",
    "sequence": 12,
    "sender_id": 123,
    "receiver_id": 456,
    "message_type": 1,
    "content": "你好",
    "sender_nickname": "Alice",
    "sender_avatar": "",
    "status": 1,
    "client_msg_id": "message-uuid",
    "created_at": "2026-11-22T18:00:00+08:00"
  }
}
```

同一用户的其他在线设备也会收到该事件。离线时不会丢消息；重连后通过 `/message/sync` 补拉。

### 错误事件

```json
{
  "type": "message.error",
  "request_id": "request-uuid",
  "code": 403,
  "message": "只能向正常好友发送消息"
}
```

常见业务码：`400` 参数无效或 `client_msg_id` 冲突，`403` 非好友/已拉黑，`404` 用户不存在或不可用，`500` 服务端处理失败。

## 历史消息

**GET** `/message/history?target_id=456&limit=50`

需要登录。首次请求不传 `before_sequence`，返回最新一页；继续向前翻页时，将响应的 `next_cursor` 作为 `before_sequence`。

| 参数 | 类型 | 必填 | 说明 |
|---|---|---:|---|
| `target_id` | uint | 是 | 对方用户 ID |
| `before_sequence` | uint64 | 否 | 只取会话序号小于该值的更早消息 |
| `limit` | int | 否 | 默认 50，最大 100 |

成功响应：

```json
{
  "code": 200,
  "message": "操作成功",
  "data": {
    "list": [],
    "has_more": false,
    "next_cursor": 0
  },
  "success": true
}
```

历史消息按 `sequence` 正序返回。仅当前用户已有会话或双方仍是正常好友时可访问。

## 增量同步

**GET** `/message/sync?target_id=456&after_sequence=12&limit=100`

需要登录，返回一个单聊会话中指定序号之后的消息，按会话序号正序排列。客户端应为每个会话单独保存同步游标；登录后可按活跃度优先逐会话拉取。

| 参数 | 类型 | 必填 | 说明 |
|---|---|---:|---|
| `target_id` | uint | 是 | 对方用户 ID |
| `after_sequence` | uint64 | 否 | 该会话客户端已持久化的最大序号；首次同步传 `0` 或不传 |
| `limit` | int | 否 | 默认 100，最大 200 |

响应使用同样的 `data.list`、`data.has_more`、`data.next_cursor` 结构。客户端应先将整页消息写入 IndexedDB，再持久化该会话的 `next_cursor`；若 `has_more` 为 `true`，继续使用新的 `after_sequence` 拉取。

会话列表项新增 `last_sequence`，表示服务端当前记录的该会话最新序号，可用于判断本地游标是否落后；同步游标仍应以已成功写入 IndexedDB 的最大 `sequence` 为准。

## 客户端建议

- 先将待发送消息及 `client_msg_id` 写入 IndexedDB，再通过 WebSocket 发送。
- 收到 `message.ack` 后按 `client_msg_id` 更新本地消息，重试时不要更换 ID。
- 收到实时消息或同步消息时按 `id` 去重，并按所属会话的 `sequence` 排序。
- 消息是否可见不代表已读；V1 不提供已读回执。
- 当前 `Conversation.UnreadCount` 会在接收消息时递增。免打扰只影响客户端提示，不影响消息持久化和未读数。
