# 配置参考

goTrust 的设置保存在 `config.toml`，位置随系统：

| 系统 | 路径 |
|---|---|
| Linux | `~/.config/gotrust/config.toml`（遵循 `$XDG_CONFIG_HOME`） |
| macOS | `~/Library/Application Support/gotrust/config.toml` |
| Windows | `%AppData%\gotrust\config.toml` |

文件权限为 0600，因为它可能保存密码。设置窗口写的就是这个文件；手工编辑也可以，保存前设置窗口会重新读盘。

## 完整示例

```toml
version = 2
active_profile = "默认"

[app]
autostart = false
start_hidden = true
auto_connect = false
auto_reconnect = false

[proxy]
socks5_bind = "127.0.0.1:1080"
http_bind = "127.0.0.1:1081"

[handoff]
listen = "127.0.0.1:1082"

[logging]
verbose = false

[identity]
device_id = "…"

[[profiles]]
name = "默认"

[profiles.server]
address = "vpn.example.com"
port = 443

[profiles.auth]
username = "alice"
auth_type = ""
domain = "local"
store_password = true
password = "…"

[profiles.keepalive]
enabled = true
interval = "90s"
targets = []

[profiles.debug]
client_data_path = ""
```

## 字段说明

### 顶层

| 字段 | 说明 |
|---|---|
| `version` | 配置格式版本，当前为 2。比程序新的版本会被拒绝读取，而不是按旧格式改写 |
| `active_profile` | 当前使用的配置名，对应某个 `[[profiles]]` 的 `name` |

### `[app]`

| 字段 | 默认 | 说明 |
|---|---|---|
| `autostart` | `false` | 登录系统时启动。设置窗口修改后立即写入系统的自启动项 |
| `start_hidden` | `true` | 启动时只显示托盘，不弹出窗口 |
| `auto_connect` | `false` | 启动后自动登录。需要已保存的密码 |
| `auto_reconnect` | `false` | 会话中断后自动重连：1/2/4/8/16 秒，最多 5 次 |

### `[proxy]`

| 字段 | 默认 | 说明 |
|---|---|---|
| `socks5_bind` | `127.0.0.1:1080` | SOCKS5 监听地址；留空表示关闭该监听 |
| `http_bind` | `127.0.0.1:1081` | HTTP 代理监听地址；留空表示关闭该监听 |

两个监听只能绑定回环地址（`127.0.0.1`、`::1`、`localhost`），端口 1–65535。填 `0.0.0.0:1080` 这类地址会在登录前被拒绝，设置窗口保存时也会提示同一条规则。至少保留一个监听。

### `[handoff]`

| 字段 | 默认 | 说明 |
|---|---|---|
| `listen` | `127.0.0.1:1082` | 提供 `/proxy.pac` 的回环监听；留空表示关闭该端点 |

端点只接受 `GET` 和 `HEAD`，每次请求按当前会话的授权重新生成 PAC，不缓存。断线时返回全 `DIRECT`。和代理监听一样只能绑定回环地址；修改后下次启动生效。托盘版和命令行版同时运行（使用不同配置）时，两者默认都占 `127.0.0.1:1082`；后启动的一方绑定失败并记入日志，应改端口或留空关闭。

### `[logging]`

| 字段 | 默认 | 说明 |
|---|---|---|
| `verbose` | `false` | 输出协议层调试日志。设置窗口的开关会立即生效并写入这里 |

### `[identity]`

一般不需要手动改，设置窗口只读显示设备标识。

| 字段 | 说明 |
|---|---|
| `device_id` | 32 位十六进制设备标识。首次启动生成，之后注入到每个配置的登录数据中，切换账号不会被门户看成新设备。手工改成门户读不懂的形式会在启动时被拒绝 |
| `client_version` | 请求中上报的客户端版本。留空使用程序内置的默认值 |
| `os_family` | `linux`（默认）、`uos`、`windows`。决定 User-Agent 的措辞；后两者使用门户对应的取值 |
| `os_arch` | User-Agent 里的架构名 |
| `user_agent` | 直接覆盖整个 User-Agent |
| `process_name` / `process_path` / `process_version` | 隧道进程块。留空则沿用客户端自己的值 |

### `[[profiles]]`

每个配置对应一个账号。

| 字段 | 说明 |
|---|---|
| `name` | 配置名，也是设置窗口里显示的名字 |
| `server.address` / `server.port` | 网关地址和端口 |
| `auth.username` | 用户名 |
| `auth.auth_type` / `auth.domain` | 认证方式与登录域。留空表示自动：网关只提供一种就用它，提供多种时询问并记住答案 |
| `auth.store_password` | 是否把密码写进配置文件 |
| `auth.password` | 保存的密码；`store_password = false` 时留空 |
| `auth.totp_secret` | TOTP 密钥。只有用户在一次性验证码面板里主动选择保存时才写入；关闭保存密码会一并清除 |
| `keepalive.enabled` | 是否发送保活请求 |
| `keepalive.interval` | 保活间隔，默认 `90s`，最小 10 s |
| `keepalive.targets` | 手工保活目标，一行一个，优先级高于自动推导的目标 |
| `debug.client_data_path` | 自定义登录数据文件路径。留空时使用配置目录下的 `client_data-<配置名>.json` |

`client_data*.json` 保存 cookies 和会话票据，权限同样是 0600。重命名配置会移动它，删除配置会删除它，两者都在保存设置之后才发生。

## 兼容性

旧版 JSON 格式（`config.json`）不做迁移，存在也直接忽略，可以删掉。配置里出现程序不认识的键会被拒绝：拼错的键不应该悄悄退回默认值。
