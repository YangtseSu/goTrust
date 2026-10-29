# goTrust

goTrust 是 aTrust 的桌面客户端。它常驻托盘，登录 Sangfor aTrust 网关并保持会话，
同时在本机回环地址上提供 SOCKS5 和 HTTP 代理，供浏览器或其他程序访问内网。

支持 Linux、macOS 和 Windows，界面有中文和 English 两种语言，默认跟随桌面语言，
可在“设置 → 语言”里固定一种。功能范围有意收窄：

* **只支持 aTrust**，不兼容 EasyConnect。
* **不创建 TUN 设备，不修改系统代理，不需要管理员权限。** 程序以当前用户身份运行；
  需要访问内网的程序，把代理指向 goTrust 的本地端口即可。
* **一个配置对应一个网关，同一时间只保持一个会话。** 多个账号就是同一份配置文件里的
  多个“配置”，同一时间只有一个处于连接状态。

## 安装

从 [GitHub Releases](https://github.com/YangtseSu/goTrust/releases) 下载对应平台的压缩包：

| 系统 | 文件 |
|---|---|
| Linux x86_64 / aarch64 | `gotrust_<版本>_linux_amd64.tar.gz`、`gotrust_<版本>_linux_arm64.tar.gz` |
| macOS（Intel 与 Apple Silicon 通用） | `gotrust_<版本>_darwin_universal.tar.gz` |
| Windows x86_64 | `gotrust_<版本>_windows_amd64.zip` |

无界面版本在同页，文件名为 `gotrust-cli_<版本>_<系统>_<架构>`。

解压后直接运行 `gotrust`（Windows 上是 `gotrust.exe`）。Linux 还需要系统安装
GTK3、libayatana-appindicator 和 hicolor-icon-theme；macOS 和 Windows 使用系统自带
的界面组件，不需要额外的运行库。

Arch Linux 用户也可以从 AUR 安装，两个包都从发布标签构建：

```sh
yay -S gotrust        # 托盘版，会装上 GTK3 和 appindicator
yay -S gotrust-cli    # 无界面版，只依赖 glibc
```

发布的包没有代码签名：macOS 会提示开发者来源不明，Windows 会弹出 SmartScreen 提示。
如需自行构建，见 [`docs/development.md`](docs/development.md)；界面与交互的规则见
[`docs/design.md`](docs/design.md)。

## 第一次使用

1. 启动 goTrust。默认只显示托盘图标，窗口从托盘打开。
2. 第一次打开时先显示“仪表盘”；点“连接”会切到“账户”页。填写“网关”（`host:port`，
   只写主机名时使用 443）和用户名。密码可以留空——网关使用二次验证或统一认证时，
   登录过程中会提示；勾选“保存密码”才会写进设置文件（权限 0600）。
3. 回到“仪表盘”点“连接”。网关要求图形验证码时，页面会显示验证码图片。
4. 连接成功后，“仪表盘”会列出这个账号能访问的资源，代理地址显示在指标行里。
5. 其余项在左侧导航里改：“账户”的认证方式（含“获取认证方式”，用来提前查询网关支持
   哪些登录方式）、“代理”的监听地址、“保活”、“启动”的开关，以及“设置”里的语言。

语言默认跟随桌面：系统的语言是中文时显示中文，是英文时显示 English，其他语言回退到
中文。在“设置 → 语言”里可以选“跟随系统”“中文”或“English”，选择立即生效，并在下次
启动时保留。`gotrust-cli` 的输出仍然是英文，因为它在终端和日志里阅读；它的登录提示页
跟随 `LC_ALL`、`LC_MESSAGES` 或 `LANG`。

窗口支持键盘操作（macOS 上把 Ctrl 换成 Command）：`Ctrl+Enter` 连接或断开，
`/` 跳到资源搜索框。登录面板打开时 `Esc` 等于“取消登录”，`Ctrl+Enter` 不会替你
结束正在回答的登录。设置就在左侧导航里，没有快捷键。

## 代理怎么用

默认监听：

| 协议 | 地址 |
|---|---|
| SOCKS5 | `127.0.0.1:1080` |
| HTTP | `127.0.0.1:1081` |

端口可以在“代理”页里改，下次连接时生效。域名在隧道内解析，不必修改系统 DNS，
也不需要 fake-IP。

命令行示例：

```sh
curl --socks5-hostname 127.0.0.1:1080 http://内网地址/
curl -x http://127.0.0.1:1081 http://内网地址/
```

在浏览器或其他程序里，把 SOCKS5 代理设为 `127.0.0.1:1080`（勾选“代理 DNS”），或者
把 HTTP 代理设为 `127.0.0.1:1081`。

两个监听只绑定回环地址；填 `0.0.0.0:1080` 这类对外地址会被拒绝。

## 分流给浏览器或 Clash（可选）

goTrust 已经按授权分流：把程序直接指向上面的代理端口即可，不需要规则。需要只让浏览器
走 goTrust，或与 Clash/Mihomo 共存时：

- **浏览器 / ZeroOmega**：用运行期的 PAC URL，默认 `http://127.0.0.1:1082/proxy.pac`；
  GUI 和 `gotrust-cli --pac-url` 都能给出。
- **Clash / Mihomo**：`gotrust-cli --print-clash`（普通片段）、`--print-clash-script`
  （Clash Verge Rev 的全局扩展脚本）、`--print-clash-config`（完整本地配置）。GUI 的
  “代理”页有同样的三项。
- **不经过外部程序**：直接把程序指向 SOCKS5/HTTP，例如
  `ALL_PROXY=socks5h://127.0.0.1:1080`。

ZeroOmega、Clash Verge Rev、FlClash 的步骤、fake-IP、UDP 限制和常见问题见
[`docs/handoff.md`](docs/handoff.md)（[English](docs/handoff.en.md)）。

## 登录时会遇到什么

* **图形验证码**：窗口显示图片，按顺序点中字符后提交。可以撤销、清空或取消；取消即
  放弃这次登录。
* **一次性验证码**（短信、TOTP、动态口令）：面板显示网关的提示和输入框。网关允许时
  可以跳过以后的二次认证。TOTP 还可以选择保存密钥，之后由 goTrust 自己生成验证码；
  密钥只保存在勾选了“保存密码”的配置里。
* **外部登录**（CAS / OAuth2）：在系统浏览器里打开网关的登录页，登录完成后把浏览器
  最终停留的地址粘贴回面板。地址不对时面板会直接提示。
* **绑定设备**：网关要求把本机绑定到账号时弹出确认面板，允许后继续登录，取消则放弃。
* **修改密码**：网关要求改密码时（例如 90 天到期，或管理员重置后的首次登录），登录会
  停下并显示修改面板，按网关给出的规则填写旧密码和新密码。

会话在线时，也可以在“账户”页的“修改密码”里主动修改密码。改完后，
该账号在其他终端上的会话需要重新登录。

## 页面

左侧导航就是全部界面，同一时间只有一个窗口：

| 页 | 作用 |
|---|---|
| 仪表盘 | 当前会话、四个指标和授权资源；连接/断开就是会话卡片右上角的按钮，资源卡片头部可搜索 |
| 账户 | 连接与安全两张卡片：网关、用户名、密码与是否保存、认证方式、登录域；修改密码与已保存的 TOTP |
| 代理 | SOCKS5 与 HTTP 的监听地址，以及 PAC 地址和 Clash/Mihomo 配置的复制 |
| 保活 | 是否发送保活请求、间隔、手工目标，下次连接生效 |
| 启动 | 登录系统时启动、启动后自动连接、只显示托盘、断线后自动重连，下次启动生效 |
| 终端管理 | 门户里该账号的设备列表；未连接时显示 0 台的占位，可刷新、信任或取消信任本机 |
| 日志 | 当前日志的实时末尾，可过滤、复制、打开目录、清除、切换详细日志 |
| 设置 | 界面语言、本机数据（登录数据文件与设备标识） |
| 关于 | 图标、名称与版本、项目主页和许可证 |
| 多账户 | 新建、复制、重命名和删除账户；选择一行编辑，保存后才切换 |

账户、代理、保活、启动等页共用一份草稿：改动后底部出现“未保存的修改”，
“保存”写入文件，“放弃修改”恢复；隐藏窗口不会丢掉草稿。切换账户会先断开当前会话。
界面语言（跟随系统、中文或 English）在“设置”页切换，立即生效并记住选择。
标题栏右上的 ☾/☀ 按钮在明暗主题之间切换，选择在本次运行内有效。

完整的配置字段说明见 [`docs/configuration.md`](docs/configuration.md)
（[English](docs/configuration.en.md)）。

保活默认每 90 秒向内网发一个真实的 HTTP 请求。手工填写的目标优先于自动推导的目标。
保活目标要用 `http://` 地址：`https://` 请求会走 CONNECT 隧道，不算应用流量，无法
续期会话。

断线后自动重连默认关闭。开启后按 1/2/4/8/16 秒的间隔重试，最多 5 次；遇到验证码或
二次认证会停止重试并弹出面板。没有保存密码时不会自动重连，因为每次重连都是一次完整
登录。

## 托盘

托盘菜单依次是：当前状态（`已连接 203.0.113.4`、`连接中`、`失败: …`）、打开仪表盘、
打开默认应用、连接/断开、切换账户（多于一个账户时才出现）、退出。鼠标悬停时显示同样的
状态和最后一次保活结果。点击托盘图标打开窗口并保留当前页；“打开仪表盘”会切到仪表盘。
在 GNOME 里托盘图标需要双击。

“登录系统时启动”会写入系统的自启动项。如果同时开启“只显示托盘”，开机后只有托盘图标，
不弹窗口。“启动后自动连接”需要已保存的密码；没有密码时会显示窗口并说明原因，不会
静默失败。

## 资源列表

登录后，“仪表盘”列出这个账号能访问的资源：门户应用（带分组和打开方式）、域名规则、
IP 地址与网段、DNS 名称映射。单个主机允许 TCP 443 或 80 时，会给出 `https://` 或
`http://` 链接，可以一键打开或复制。goTrust 只打开 `http`/`https` 链接，门户提供的
其他协议不会交给本机程序处理。

最近一次的资源列表会缓存到配置文件旁的 `resources.json`（权限 0600），里面只有内网
地址，没有凭据。命令行可以用 `gotrust-cli --print-resources` 直接打印这份缓存。

## 日志

日志位置：

| 系统 | 路径 |
|---|---|
| Linux | `$XDG_STATE_HOME/gotrust/gotrust.log`（默认 `~/.local/state/gotrust/gotrust.log`） |
| macOS | `~/Library/Logs/gotrust` |
| Windows | `%LocalAppData%\gotrust\logs` |

文件权限 0600，单个文件 5 MiB，保留三个历史文件。密码、cookie、会话令牌和验证码内容
在写入磁盘前会被替换掉。“日志”页的“打开目录”可以直接打开日志目录；“清除”会清空当前日志和轮转文件。

## 命令行

`gotrust-cli` 是没有界面的版本，适合服务器或远程主机。它读取同一份设置、登录、提供
同样的代理端口。登录过程中需要人工操作时，它会在 `127.0.0.1` 上提供一个一次性网页，
并打印地址（加 `--print-url` 则只打印地址、不打开浏览器），在网页里完成验证码、一次性
验证码或回调地址。

```sh
gotrust-cli [--config 文件]                  # 登录并保持运行，直到 Ctrl-C 或 SIGTERM
gotrust-cli [--config 文件] --status         # 该配置是否正被某个进程使用
gotrust-cli [--config 文件] --print-resources
gotrust-cli --pac-url                        # 运行中的 PAC 地址
gotrust-cli --print-pac                      # 用缓存的授权打印 PAC
gotrust-cli --print-clash                    # 普通 Clash/Mihomo 片段
gotrust-cli --print-clash-script             # Clash Verge Rev 的全局扩展脚本
gotrust-cli --print-clash-config             # 完整的本地 Mihomo 配置
gotrust-cli --json                           # 资源与状态以 JSON 输出
gotrust-cli --version
```

| 退出码 | 含义 |
|---:|---|
| 0 | 正常运行结束；配合 `--status` 表示有进程正在使用该配置 |
| 1 | 设置、登录、监听或会话失败 |
| 2 | 命令行参数不正确 |
| 3 | `--status` 没有找到正在使用该配置的进程 |

同一个配置同一时间只能有一个进程使用：托盘版和命令行版用不同配置时可以同时运行，
用同一个配置时后启动的一方会被拒绝，避免两个进程共用一份登录数据。

## 系统要求与已知限制

* Linux 需要 GTK3、libayatana-appindicator 和 hicolor-icon-theme；macOS 和 Windows
  使用系统自带的界面组件。
* macOS 和 Windows 的构建通过了编译检查，但还没有人在真机上运行过。遇到问题请提
  issue。
* 短信、TOTP、RADIUS 和 CAS/OAuth2 这几种二次认证只有单元测试覆盖，尚未在真实网关上
  验证。
* 主认证方式只支持密码、CAS 和 OAuth2。网关如果只提供短信主认证，goTrust 无法登录。
* 不提供 TUN 模式，也不修改系统代理。需要全局代理时，自行把程序或系统代理指向本地
  监听；浏览器和 Clash 系的分流见上文。
* 会话只在本次连接内有效：断开后再连接就是重新登录，保存的登录数据无法恢复旧会话。
  所以自动连接需要保存密码。

## 鸣谢

* [zju-connect](https://github.com/Mythologyli/zju-connect)：aTrust 协议的实现，本项目的
  协议层由它导入并继续维护。
* [EZ4Connect](https://github.com/chenx-dust/EZ4Connect)：交互和界面设计的重要参考。
* [mygo](https://github.com/egoist/mygo)：托盘和窗口使用的 Go 桌面框架。

## 许可证

**AGPL-3.0-or-later**。链接进来的协议层（`core/atrust/`）为 AGPL-3.0；二进制是单一
组合作品，因此发布的可执行文件按 AGPL-3.0 授权。若你把修改后的 goTrust 作为网络服务
运行，AGPL 第 13 条要求你向该服务的使用者提供源代码。
