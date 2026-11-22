# 聊天媒体、在线状态与注册接口 V1

## 范围

V1 支持已互为正常好友的用户进行单聊文本、图片和文件聊天，包括 WebSocket 连接、实时收发、历史分页和按会话增量同步。消息以 MySQL 为准；离线消息通过同步接口补拉。图片和文件保存在服务端本地目录中。

暂不支持群聊、语音、消息撤回、送达回执、已读回执和多实例 WebSocket 转发。每个会话有独立递增的 `sequence`；消息 `id` 仍是全局主键，不应用它判断会话内是否漏消息。

REST 业务响应使用统一结构 `{code, message, data?, success}`。项目当前业务失败也可能以 HTTP `200` 返回，前端应检查响应体中的 `success` 和 `code`。

## 接口一览

| 接口 | 认证 | 用途 |
|---|---|---|
| `POST /user/register` | 否 | 注册用户名、密码和基本资料 |
| `PUT /user/presence` | 是 | 设置当前用户在线或隐身偏好 |
| `GET /friend/list` | 是 | 分页获取好友及当前在线状态 |
| `POST /ws/ticket` | 是 | 获取一次性 WebSocket 票据 |
| `GET /ws?ticket=...` | 票据 | 建立 WebSocket 长连接 |
| `POST /common/upload` | 是 | 通用上传图片或文件 |
| `GET /files/{file_id}` | 是 | 下载文件或预览图片 |
| `GET /message/history` | 是 | 分页获取单聊历史消息 |
| `GET /message/sync` | 是 | 按单聊会话序号增量同步消息 |
| `POST /conversation/read` | 是 | 进入会话时清零当前用户未读数 |

## 数据与状态设计

- `Conversation.unread_count` 是按用户、按会话维护的未读计数。进入会话调用 `/conversation/read` 后直接清零；不推进已读游标，也不修改单条消息的已读状态。
- `User.gender` 和 `User.birthday` 保存注册资料；`User.presence_mode` 保存用户选择的 `online` / `invisible` 偏好。`User.status` 仍表示账号是否可用，不代表在线。
- 好友实际可见在线状态由“有活动 WebSocket 连接”且 `presence_mode=online` 共同决定。隐身仍保持连接、可以收发消息，但好友看到离线。
- `files` 保存通用文件 ID、上传者、原始文件名、服务器存储名、本地绝对路径、MIME 类型、文件大小和 SHA-256；文件实体保存在本机，不写入 MySQL。同一上传者上传相同内容时按 SHA-256 复用已有文件记录及磁盘文件。
- 文件先通过 `/common/upload` 上传，再通过 WebSocket 消息引用 `file_id`。消息内容保存服务端查到的文件信息和下载 URL。下载需要 Bearer Token；仅上传者和聊天消息的接收者可访问。

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

### 发送单聊消息

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
| `data.message_type` | uint8 | 是 | `1` 文本、`2` 图片、`4` 文件 |
| `data.content` | string | 条件必填 | 文本消息必填，最多 4000 个 Unicode 字符；图片/文件消息不传 |
| `data.file_id` | string | 条件必填 | 图片/文件消息必填，使用通用上传接口返回的 ID |

服务端校验发送者和接收者均为可用用户、双方仍是正常好友且没有任一方向拉黑。消息和双方会话摘要/未读数在同一数据库事务中保存；只有事务成功后才推送。

图片/文件消息的 `data` 示例：

```json
{
  "target_id": 456,
  "client_msg_id": "message-uuid",
  "message_type": 2,
  "file_id": "uuid"
}
```

文件消息的服务端 `content` 保存为 JSON 字符串，包含 `file_id`、`file_name`、`content_type`、`size` 和公开 `url`。客户端使用该 URL 下载或预览；文件名、大小和 MIME 类型均由服务端从 `files` 表读取。

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
图片和文件消息也返回相同的 `MessageVO` 字段；其中 `message_type` 分别为 `2`、`4`，`content` 是包含文件 ID、文件名、MIME 类型、大小和下载 URL 的 JSON 字符串。

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

当前进程中的在线接收方会收到该事件。离线时不会丢消息；重连后通过 `/message/sync` 补拉。

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

会话列表项新增 `last_sequence`，表示服务端当前记录的该会话最新序号，可用于判断本地游标是否落后；同步游标仍应以已成功写入 IndexedDB 的最大 `sequence` 为准。会话列表的 `last_message` 对图片和文件消息分别显示 `[图片]`、`[文件]`，文本消息仍显示文本摘要。

## 客户端建议

- 先将待发送消息及 `client_msg_id` 写入 IndexedDB，再通过 WebSocket 发送。
- 收到 `message.ack` 后按 `client_msg_id` 更新本地消息，重试时不要更换 ID。
- 收到实时消息或同步消息时按 `id` 去重，并按所属会话的 `sequence` 排序。
- 消息是否可见不代表已读；V1 不提供已读回执。
- 当前 `Conversation.UnreadCount` 会在接收消息时递增。免打扰只影响客户端提示，不影响消息持久化和未读数。



## 通用图片和文件上传

**POST** `/common/upload`

需要登录，使用 `multipart/form-data` 上传单个 `file` 字段，不需要指定聊天对象。文件保存到 `config.yaml` 的 `file.upload_dir` 所配置目录（当前为 `/yug-im-server/uploads/files`），完整本地路径及元数据保存到 `files` 表。

图片支持 JPG/JPEG、PNG、GIF、WebP（最大 10 MiB）；任意其他文件类型最大 50 MiB。服务端会清理上传文件名并使用 UUID 作为磁盘存储名。成功响应包含 `file_id`、`message_type`、`file_name`、`content_type`、`size` 和 `url`；聊天客户端用返回的 `file_id` 通过 WebSocket 发送 `message_type` 为 `2`（图片）或 `4`（文件）的消息。

## 下载图片或文件

**GET** `/files/{file_id}`

需要登录，使用 `Authorization: Bearer <token>`。前端可携带认证头通过 `fetch` 获取文件 Blob；聊天接收方仅在消息中引用该文件后可访问。图片以 `inline` 返回，普通文件以 `attachment` 返回。

成功响应是文件字节流，而不是 JSON。响应头包括 `Content-Type`、`Content-Disposition` 和 `X-Content-Type-Options: nosniff`。无效文件 ID 或本地文件已丢失时返回 `404`；无权访问返回 `403`。

## 清除会话未读数

**POST** `/conversation/read`

用户进入单聊或群聊会话时调用，清除当前用户该会话的未读数量。请求体：

```json
{
  "target_id": 456,
  "target_type": 1
}
```

`target_type` 为 `1` 单聊、`2` 群聊；`target_id` 为好友用户 ID 或群 ID。此操作只清除会话未读计数，不提供已读回执。当前群聊消息能力尚未实现。

成功响应：

```json
{
  "code": 200,
  "message": "ok",
  "success": true
}
```

重复调用是幂等的；如果当前用户没有对应会话行，也不会修改其他用户数据。
参数校验失败返回业务码 `400`，数据库错误返回 `500`。

## 注册与在线状态

### 注册

**POST** `/user/register`

下列字段均必填：`username`、`password`、`nickname`、`gender`、`birthday`。用户名遵循现有 4–16 位字母/数字/下划线规则；密码遵循现有 6–20 位规则；`gender` 为 `1` 男、`2` 女；`birthday` 格式为 `YYYY-MM-DD`，不能晚于今天。

请求示例：

```json
{
  "username": "alice_01",
  "password": "Abc123!",
  "nickname": "Alice",
  "gender": 2,
  "birthday": "2000-01-15"
}
```

成功响应为 `code: 200`、`message: "注册成功"`、`success: true`，不返回 Token；注册后再调用登录接口。用户名重复返回业务码 `1006`，参数或生日无效返回 `400`。

登录响应示例：

```json
{
  "code": 200,
  "message": "操作成功",
  "data": {
    "token": "jwt-token",
    "user_info": {
      "id": 123,
      "username": "alice_01",
      "nickname": "Alice",
      "gender": 2,
      "birthday": "2000-01-15",
      "presence_mode": "online",
      "avatar": "",
      "bio": "",
      "created_at": "2026-11-22T18:00:00+08:00"
    }
  },
  "success": true
}
```

`gender`、`birthday` 对迁移前的旧账号可能为空或省略。`presence_mode` 是用户设置的偏好，不是当前连接状态。

### 设置自己的在线偏好

**PUT** `/user/presence`

请求体：

```json
{
  "mode": "invisible"
}
```

`mode` 可为 `online` 或 `invisible`。实际在线还要求用户存在有效 WebSocket 连接；隐身用户仍可正常收发消息，但好友看到其为离线。成功响应的 `data` 包含当前 `mode` 和 `is_online`。

```json
{
  "code": 200,
  "message": "设置成功",
  "data": {
    "mode": "invisible",
    "is_online": false
  },
  "success": true
}
```

无效的 `mode` 返回业务码 `400`，其他设置失败返回 `500`。

### 获取和接收好友在线状态

`GET /friend/list?page=1&page_size=20` 的 `page`、`page_size` 均可选，默认分别为 `1`、`20`，最大每页 `100`。每个好友项包含 `is_online`；该字段在 `data.list[]` 项的顶层，而不在 `user` 对象中。

响应示例：

```json
{
  "code": 200,
  "message": "操作成功",
  "data": {
    "total": 1,
    "page": 1,
    "page_size": 20,
    "list": [
      {
        "friendship_id": 88,
        "remark": "",
        "group_id": 0,
        "created_at": "2026-11-22T18:00:00+08:00",
        "is_online": true,
        "user": {
          "id": 456,
          "username": "alice_01",
          "nickname": "Alice",
          "avatar": "",
          "bio": ""
        }
      }
    ]
  },
  "success": true
}
```

用户的 WebSocket 连接上线/断开或在线偏好发生可见变化时，已连接的好友端会收到：

```json
{
  "type": "presence.changed",
  "data": {
    "user_id": 456,
    "is_online": true
  }
}
```

数据库迁移会把旧 `message_attachments` 元数据迁移到通用 `files` 表，更新历史文件消息中的 `file_id` 和下载 URL，然后删除旧表。旧文件仍使用原 `CHAT_UPLOAD_DIR`（默认 `uploads/chat`）路径；新上传使用 `file.upload_dir` 配置的目录。首次部署此版本前请备份数据库和上传目录，并执行数据库迁移。
