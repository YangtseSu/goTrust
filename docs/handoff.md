中文 | [English](handoff.en.md)

# 外部程序接入：PAC、ZeroOmega、Clash 与 Mihomo

goTrust 不创建 TUN，也不改系统代理。它只提供本地 SOCKS5/HTTP 代理，并按授权分流：
被授权的目的进隧道，其余从本机直连。所以任何程序只要指向代理端口，就已经是“资源走
goTrust、其余直连”，不需要规则。

需要“只让浏览器走 goTrust”，或与另一个代理（Clash/Mihomo）共存时，才用本文的 PAC
和规则片段。

## 前提

1. goTrust 已登录；PAC 和规则都来自这次授权。
2. 代理监听默认是 `127.0.0.1:1080`（SOCKS5）和 `127.0.0.1:1081`（HTTP），可在设置里改。
3. 托盘版和命令行版使用同一份配置；用 `--config` 时，下面的命令也带同一个参数。
4. PAC/规则是授权快照。授权变化后，PAC 会自动跟着实时会话变；Clash 片段和完整配置
   是静态的，需要重新生成。

## PAC

运行期间默认在 `http://127.0.0.1:1082/proxy.pac` 提供 PAC：

- GUI：代理页的 PAC 地址，点“复制”。
- CLI：`gotrust-cli --pac-url` 打印地址；`gotrust-cli --print-pac` 从上次登录的缓存
  打印脚本。
- 设置文件里 `[handoff] listen = ""` 关闭端点；改端口后下次启动生效。

ZeroOmega：

1. 新建 **PAC profile**，URL 填上面的地址。
2. 浏览器切换到这个 profile。
3. 系统代理也可以直接填这个 PAC URL。

匹配规则：

- 字面 IPv4 命中授权前缀 → 代理；
- 域名等于或位于授权后缀之下 → 代理（浏览器把域名交给代理，goTrust 在隧道内解析）；
- `localhost`、回环地址、网关本身 → 直连；
- 其余 → 直连。

断线时 PAC 返回全 `DIRECT`，浏览器不会因为 goTrust 重连而卡住。

边界：

- PAC 不覆盖 UDP；QUIC/WebRTC 会绕过代理。
- PAC 只认授权里出现的名字（`Resources.Domains` 和网关下发的 域名→IP 映射）。如果
  资源只授权了 IP、你却用域名访问，PAC 不知道这个名字对应哪个 IP；把浏览器全局指向
  goTrust（或 Clash 的 fake-IP），让 goTrust 在隧道内解析。
- PAC 返回 HTTP 代理（CONNECT 带域名）；HTTP 监听关闭时回退 SOCKS5。

## Clash / Mihomo

三种输出，按客户端选，不需要改键名：

| 命令 | 用法 |
|---|---|
| `gotrust-cli --print-clash` | 普通片段：粘进 Mihomo/Clash 配置的 `proxies`、`proxy-groups`、`rules` |
| `gotrust-cli --print-clash-script` | Clash Verge Rev 的全局扩展脚本（Script） |
| `gotrust-cli --print-clash-config` | 完整的本地 Mihomo 配置：任何客户端都可作为本地配置导入（只含 goTrust） |

代理页的 Clash 格式选择（`Clash`、`Clash Script`、`Clash 配置`）旁提供“复制”，
选中的格式对应上面三条命令，用的是实时授权，不是缓存。

三种输出共用一套规则，顺序是：

1. goTrust 进程（`gotrust` / `gotrust-cli`，含 `.exe`）→ `DIRECT`；
2. 网关地址 → `DIRECT`；
3. 授权域名 → `goTrust-routes`；
4. 授权 IP 段 → `goTrust-routes`（`no-resolve`）。

前两条防止 TUN 把 goTrust 自己的底层 TLS 再送回代理；不要删。没有 `10/8` 之类的私有段
预设：授权可能是公网地址，也可能就在私有段里。

### 普通片段（Mihomo 核心 / Clash 配置）

把输出合并进主配置：

- `proxies` 里是 `goTrust`（SOCKS5，`udp: true`）；
- `proxy-groups` 里是 `goTrust-routes`（`select`，成员 `goTrust`、`DIRECT`）；
- `rules` 必须放在原配置的 `MATCH` 之前。

订阅更新会覆盖主配置，长期使用建议用下面的 Merge 或独立配置。

### Clash Verge Rev 的全局扩展脚本（Script）

**不要用 Merge 配置**：Verge Rev 的 Merge 是对配置做深合并，列表键会被整体替换，并且
当前版本并不实现 `prepend-rules` / `prepend-proxies` 这类前插键。把片段放进 Merge 后，
这些键会原样留在运行配置里、被 Mihomo 忽略，`rules:` 仍是订阅原样，规则不会生效。

正确做法是用 Script：

1. 运行 `gotrust-cli --print-clash-script`。
2. Verge Rev → “配置/订阅”页 → 全局扩展配置里的 **Script（脚本）** → 粘贴，保存并启用 🔥。
3. 脚本会把 `goTrust` 代理、`goTrust-routes` 组和规则前插到每份配置；订阅更新不受影响。
4. 如果使用每份配置自己的 Script，效果相同，但只对那份配置生效；改完要重新启用一次。

脚本会先去掉同名的旧代理/组/规则，再前插回去，所以重复执行不会产生重复项。规则顺序仍是
进程/网关直连 → 授权域名 → 授权 IP。

替代做法：Verge Rev 每份配置有“扩展规则 / 扩展代理 / 扩展策略组”三个文件，格式是
`prepend:` / `append:` / `delete:` 列表；把 `--print-clash` 的内容拆到这三个 `prepend`
里也可以，不需要脚本引擎。

### 完整配置（本地配置导入）

`gotrust-cli --print-clash-config` 输出一份可运行的 Mihomo 配置（`mixed-port: 7890`、
`mode: rule`、以 `MATCH,DIRECT` 收尾），可直接作为本地配置导入。它只含 goTrust，
不含你原来的订阅节点；适合“goTrust 是唯一出口”的用法。

### FlClash

FlClash 的**标准覆写只能追加规则，不能新增代理和策略组**，所以没有“原样粘贴就加节点”
的 YAML 入口：

- 要独立用 goTrust：把 `--print-clash-config` 作为本地配置导入；
- 要和订阅共存：在 FlClash 的自定义配置里手工加 SOCKS5 `127.0.0.1:1080`，或用它的
  脚本覆写。Verge 的全局扩展脚本只适用于 Verge Rev。

### TUN 与 fake-IP

- 用 TUN：启用 fake-IP，并且**不要把授权域名放进 `fake-ip-filter`**。过滤后域名变成
  真实 IP，`DOMAIN-SUFFIX` 规则可能不命中，纯域名授权会失效。
- 只用系统代理、不开 TUN：不需要 fake-IP。
- 浏览器不要再同时全局指向 goTrust：让浏览器走 Clash，或让浏览器直连 goTrust、Clash
  管其它程序，避免多一跳。

### 验证

- 访问内网地址，在 Clash 的“连接”页确认命中 `DOMAIN-SUFFIX,...` 或
  `IP-CIDR,...,goTrust-routes`，代理链是 `goTrust`。
- 访问公网，确认仍命中原有规则。
- 断开 goTrust 后访问内网应失败，而不是悄悄直连。

### 常见问题

| 现象 | 原因与处理 |
|---|---|
| 内网访问没有命中 goTrust | 目标不在本次授权；重新登录后重新生成。或者用域名访问但授权只有 IP，且 TUN 未开 fake-IP |
| 规则完全不生效 | 把片段放进了 Merge 配置（Verge 不实现 `prepend-*`）；改用 Script 或“扩展规则/代理/组” |
| 启动后没有 `goTrust` 代理 | 片段端口与 goTrust 实际 SOCKS5 端口不一致；改 `proxies` 里的 `port` |
| 与已有名称冲突 | 已有同名 `goTrust` / `goTrust-routes` 时先改名 |
| 订阅更新后规则消失 | 直接改了主配置；改用全局扩展脚本或独立本地配置 |
| TUN 下 goTrust 自己连不上网关 | 片段最前面的 `PROCESS-NAME` 和网关直连规则被删了；保留它们 |
| 想让 goTrust 掉线时自动绕过 | 把组类型从 `select` 改成 `fallback`，成员顺序保持 `goTrust`、`DIRECT` |

## 不经过外部程序

```sh
ALL_PROXY=socks5h://127.0.0.1:1080 curl https://内网地址/
HTTPS_PROXY=http://127.0.0.1:1081 git clone https://内网地址/repo
ssh -o ProxyCommand="nc -X 5 -x 127.0.0.1:1080 %h %p" user@内网主机
```

systemd 服务可以写 `Environment=ALL_PROXY=socks5h://127.0.0.1:1080`。程序如果不认这些
变量，用 proxychains-ng 只把它的 TCP 流量转进来。

## 限制

- 仅 IPv4 资源和规则。
- HTTP 代理仅 TCP；UDP 只有 SOCKS5。
- 没有 DNS 监听器：名字由浏览器/Clash交给代理，goTrust 在隧道内解析。工具自己先解析
  再按 IP 连接时，只有 IP 授权能命中。
- PAC 和 Clash 规则不改变 goTrust 的分流：未授权目的始终由 goTrust 从本机直连。
- 托盘版和命令行版同时运行（不同配置）时，PAC 默认都占 `127.0.0.1:1082`；后启动的
  一方绑定失败并记入日志，应改端口或留空关闭。
