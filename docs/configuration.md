中文 | [English](configuration.en.md)

# 配置参考

goTrust 的设置保存在 `config.toml`，位置随系统：

| 系统 | 路径 |
|---|---|
| Linux | `~/.config/gotrust/config.toml`（遵循 `$XDG_CONFIG_HOME`） |
| macOS | `~/Library/Application Support/gotrust/config.toml` |
| Windows | `%AppData%\gotrust\config.toml` |

文件权限为 0600，因为它可能保存密码。窗口底部的保存栏写的就是这个文件；手工编辑也可以，但程序运行期间只在启动时读取，窗口中的保存会用自己的草稿覆盖文件。

## 完整示例

```toml
version = 4
active_profile = "工作"

[[profiles]]
id = "…"
name = "工作"
auto_reconnect = false

[profiles.server]
address = "vpn.example.com"
port = 443

[profiles.auth]
username = "alice"
auth_type = ""
domain = "local"
store_password = true
password = "…"

[profiles.app]
autostart = false
start_hidden = true
auto_connect = false

[profiles.proxy]
socks5_bind = "127.0.0.1:1080"
http_bind = "127.0.0.1:1081"

[profiles.handoff]
listen = "127.0.0.1:1082"

[profiles.logging]
verbose = false

[profiles.identity]
device_id = "…"
mac_addresses = ["…"]
os_family = "linux"

[profiles.keepalive]
enabled = true
interval = "90s"
targets = []

[[profiles]]
name = "个人"
# 其余段落省略，账户按需取默认值。
```

## 一个账户就是一套完整配置

同一时刻只有一个账户在线，因此会话用到的每一项设置都属于某个账户。全局段只保留文件自身的结构信息。两个账户因此可以使用不同的监听端口、不同的日志级别、呈现不同的客户端；生效的永远是当前账户的那一份。

新增账户有两种方式（设置 → 多账户），区别在于网关看到的是什么：

| 操作 | 网关记录的设备 |
|---|---|
| 新建 | 一台新设备。该账户首次登录时生成自己的 `device_id` 与硬件地址，网关把它当作另一台机器 |
| 复制 | 来源账户的那台设备。用同一台机器登录第二个账号，对网关而言仍是一台设备，副本沿用它的 `device_id` 与硬件地址 |

只有确实是另一台机器时才用“新建”。同一台机器上的第二个账号请用“复制”，再改用户名和密码。想让副本成为独立设备，手工修改 `device_id` 与 `mac_addresses`。

## 字段说明

### 顶层

| 字段 | 说明 |
|---|---|
| `version` | 配置格式版本，当前为 4。3 及更早版本的文件会在读取时迁移：共享设置复制进每个账户。比程序新的版本会被拒绝读取 |
| `active_profile` | 当前使用的账户名，对应某个 `[[profiles]]` 的 `name` |

### `[[profiles]]`

每个 `[[profiles]]` 是一个账户，承载该账户用到的全部设置。

| 字段 | 默认 | 说明 |
|---|---|---|
| `id` | 自动生成 | 本地稳定标识。重命名不变，不发给网关，只用于本地状态索引 |
| `name` | `默认` | 账户名，也是账户页显示的名字和登录数据文件名的主干 |
| `auto_reconnect` | `false` | 断线后按 1/2/4/8/16 秒重试，最多 5 次。默认关闭：完整登录会更换设备令牌并踢掉该账户的其他客户端 |
| `server.address` / `server.port` | — / `443` | 网关地址与端口。界面上是一个接受 `host:port` 的“网关”字段 |
| `auth.username` | — | 用户名 |
| `auth.auth_type` / `auth.domain` | 自动 / `local` | 认证方式与登录域。留空表示自动：只有一种方式时直接使用，多种时询问 |
| `auth.store_password` | `true` | 是否把密码写入文件 |
| `auth.password` | — | 已保存的密码；`store_password = false` 时为空 |
| `auth.totp_secret` | — | TOTP 密钥，仅在用户选择保存时写入 |
| `app.autostart` | `false` | 登录系统时启动。在“启动”页修改后立即写入系统自启动项 |
| `app.start_hidden` | `true` | 启动时只显示托盘，不弹出窗口 |
| `app.auto_connect` | `false` | 启动后自动登录。需要已保存的密码 |
| `proxy.socks5_bind` / `proxy.http_bind` | `127.0.0.1:1080` / `127.0.0.1:1081` | 监听地址；留空则关闭该监听。只能绑定回环地址，端口 1–65535，至少保留一个 |
| `handoff.listen` | `127.0.0.1:1082` | 提供 `/proxy.pac` 的回环监听；留空则关闭 |
| `logging.verbose` | `false` | 输出协议级调试日志。日志页的开关立即生效 |
| `keepalive.enabled` / `keepalive.interval` / `keepalive.targets` | 开 / `90s` / 无 | 应用流量保活。手工目标优先于自动推导（授权端口 80、443、22，以及只有域名的授权） |
| `identity.device_id` | 自动生成 | 32 位十六进制的设备标识，写入该账户的登录数据 |
| `identity.mac_addresses` | 自动生成 | 网关为该账户记录的硬件地址 |
| `identity.client_version` | — | 请求中上报的客户端版本。留空使用程序内置默认值 |
| `identity.os_family` | `linux` | `linux`、`uos` 或 `windows`。决定 User-Agent 措辞；后两者使用网关期望的写法 |
| `identity.os_arch` | 本机 | User-Agent 中的架构名 |
| `identity.user_agent` | 推导 | 直接覆盖整个 User-Agent |
| `identity.process_name` / `process_path` / `process_version` | 客户端自身 | 该账户上报的隧道进程块 |

PAC 端点只接受 `GET` 与 `HEAD`，每次请求即时生成当前会话的 PAC，不缓存；未连接时返回全 `DIRECT` 文件。代理与 PAC 监听都只能绑定回环地址，修改在下次启动生效。

## 兼容性

3 及更早版本写出的文件在读取时迁移：共享的 `[app]`、`[proxy]`、`[handoff]`、`[logging]`、`[identity]` 设置复制进每个账户，因此升级后的第一次启动仍然监听应用已经指向的端口，也仍然呈现网关已经认识的设备。文件会在下次保存时按新结构重写。旧 JSON 格式（`config.json`）不做迁移，存在时也不会被读取，可以直接删除。无法识别的字段会导致拒载：拼错的键不应静默退回默认值。
