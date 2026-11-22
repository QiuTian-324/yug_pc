# 会话置顶与免打扰接口

## 认证

接口需要登录，按当前项目的认证方式携带 Token。

## 设置置顶和免打扰

**POST** `/conversation/pin-mute`

请求头：

```http
Content-Type: application/json
```

请求参数：

| 字段 | 类型 | 必填 | 说明 |
|---|---|---:|---|
| `target_id` | uint | 是 | 会话目标 ID |
| `target_type` | int | 是 | `1`：单聊；`2`：群聊 |
| `is_pinned` | bool | 否 | `true`：置顶；`false`：取消置顶 |
| `is_muted` | bool | 否 | `true`：免打扰；`false`：取消免打扰 |

`is_pinned` 和 `is_muted` 至少传一个。未传的字段保持原状态；两个字段可以同时传。设置只影响当前登录用户。

### 请求示例

置顶单聊：

```json
{
  "target_id": 123,
  "target_type": 1,
  "is_pinned": true
}
```

取消置顶并开启免打扰：

```json
{
  "target_id": 123,
  "target_type": 1,
  "is_pinned": false,
  "is_muted": true
}
```

取消群聊免打扰：

```json
{
  "target_id": 456,
  "target_type": 2,
  "is_muted": false
}
```

### 成功响应

```json
{
  "code": 200,
  "message": "设置成功",
  "data": {
    "target_id": 123,
    "target_type": 1,
    "is_pinned": false,
    "is_muted": true
  },
  "success": true
}
```

`data` 返回更新后的完整置顶和免打扰状态。会话列表接口 `GET /conversation/list` 也会返回 `is_pinned` 和 `is_muted`；列表按置顶优先、最近活跃时间倒序排列。

### 错误响应

| 业务码 | 含义 |
|---:|---|
| `400` | 参数无效，或没有传 `is_pinned` / `is_muted` |
| `404` | 当前用户没有对应会话 |
| `500` | 更新会话设置失败 |

当前统一响应封装会以 HTTP `200` 返回业务错误；客户端应检查响应体中的 `code` 或 `success`。
