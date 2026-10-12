# QNatter

[![License: GPL v3](https://img.shields.io/badge/License-GPLv3-blue.svg)](LICENSE)

QNatter 是一个面向 OpenWrt/ImmortalWrt 的端口打洞集成包，基于 [MikeWang000000/Natter](https://github.com/MikeWang000000/Natter) 构建。

本仓库的 OpenWrt/ImmortalWrt 集成、Go 实现、LuCI 界面和文档由 AI 辅助开发；源自上游 Natter 的部分保留其原始许可与归属。

本仓库主要提供：

- `qnatter`：OpenWrt 软件包，包含 QNatter Go 运行核心、init 脚本、热重载、状态文件、通知脚本和自动防火墙规则。
- `luci-app-qnatter`：LuCI 管理界面，用于配置多实例、WAN 绑定、转发方式、qBittorrent、Cloudflare SRV 和日志查看。
- `go-qnatter`：用于 OpenWrt 打包的 QNatter Go 版本实现，保留 QNatter 打洞核心行为，并补充 OpenWrt 多 WAN/策略路由集成。
- `tests`：针对 init、hotplug、notify、rpcd、状态输出、包结构和 LuCI 静态检查的回归测试。

## 功能

- 支持多个 QNatter 实例分别绑定不同 WAN 接口。
- 支持 `nftables` / `nftables-snat` / `socket`，并在存在 `socat` 或 `gost` 时显示可选转发方式。
- 为每个实例维护持久 `route_slot`，生成独立的 `fwmark`、策略路由表和优先级，避免多 WAN 或同接口多实例互相覆盖。
- 自动清理 QNatter 自己创建的旧 nftables 规则和策略路由规则。
- 可选自动添加单独防火墙放行规则，不全局改防火墙策略。
- 可选更新 qBittorrent 监听端口。
- 可选根据映射公网端口更新 Cloudflare SRV 记录。
- LuCI 界面支持实例重命名、状态查看和日志查看。

## 安全相关的默认行为

以下默认值出于安全考虑而收紧，需要时可显式放开：

- **完成通知 Webhook 默认拒绝回环 / 内网 / 链路本地目标**（防止 SSRF）。若确实需要通知本机或局域网内的接收端，在 LuCI 全局设置中打开“允许 Webhook 指向内网/回环地址”，或执行
  `uci set qnatter.global.allow_private_webhook=1 && uci commit qnatter`。
- **完成内联脚本默认不执行**。要让 `completion_script_inline` 生效，需由 root 显式放置授权文件（删除该文件即可随时收回授权）：

  ```sh
  install -d -m 0755 /etc/qnatter
  install -m 0644 /dev/null /etc/qnatter/allow_inline_scripts
  ```

- **`completion_script_test` / `completion_webhook_test` 只授予 root 会话**；防火墙遗留规则清理（`cleanup_firewall_rule`）同样位于独立 ACL 组 `luci-app-qnatter-firewall`，需显式授权后才能由非 root 管理员调用。

## 与 OpenClash 共存

OpenClash 会把自己的链与规则**动态注入 `inet fw4` 表**（例如 `openclash_upnp`），并靠 watchdog 检查/重建；而 `fw4 reload` 会整表重建，把这些注入的规则清空，OpenClash 需要等 watchdog 下一次运行（约两分钟）才能恢复。因此：

- QNatter 的 `auto_firewall` 在检测到 OpenClash 运行时**默认跳过 `fw4 reload`**：`firewall.qnatter_*` 段照常写入 UCI，规则会在下一次防火墙 reload（或 OpenClash 停止后）生效，并在日志中写明原因。
- 需要立即生效：`uci set qnatter.global.auto_firewall_reload='always'`；完全不要 reload：`'never'`（默认 `'auto'`）。
- QNatter 自身的规则在**独立表** `ip qnatter` 中，策略路由使用 `fwmark 0x4e000<slot>` 与 `table/priority 20000+slot`，与 OpenClash 使用的 `inet fw4` 表、`table 354`（tun）等互不相交；OpenClash 只删除自己的规则，不会清理 QNatter 的表。
- QNatter 的每条 nft 规则都以「精确 IP + 协议 + 端口」为匹配条件，不会给其他流量（包括 OpenClash 转发的流量）打标或改写。
- 两者都会涉及 UPnP：OpenClash 的 `upnp_exclude()` 对识别到的 UPnP 端口做「不代理」放行，不影响 QNatter 自行发起的 IGD 映射请求。

## 安装与编译

在 OpenWrt/ImmortalWrt 源码树中，将 QNatter 作为 feed 添加：

```sh
# 在 feeds.conf 中添加 qnatter feed
echo "src-git qnatter https://github.com/qimaoww/QNatter.git" >> feeds.conf

# 更新并安装 feed
./scripts/feeds update qnatter
./scripts/feeds install -a -p qnatter

# 在 menuconfig 中选中 Network -> qnatter、LuCI -> Applications -> luci-app-qnatter
# 如需中文界面，同时选中 LuCI -> Translations -> luci-i18n-qnatter-zh-cn
make menuconfig

# 编译
make package/qnatter/compile
make package/luci-app-qnatter/compile
```

编译产物通常位于：

```text
/openwrt/immortalwrt/bin/packages/<arch>/base/
```

## APK 安装

将编译出的 APK 上传到路由器后安装：

```sh
apk add --allow-untrusted --force-overwrite /tmp/qnatter-*.apk
apk add --allow-untrusted --force-overwrite /tmp/luci-app-qnatter-*.apk
apk add --allow-untrusted --force-overwrite /tmp/luci-i18n-qnatter-zh-cn-*.apk
/etc/init.d/qnatter restart
```

安装后可在 LuCI 的 `服务 -> QNatter` 中配置实例。

## 测试

本仓库提供本地回归测试：

```sh
cd /root/codex
(cd go-qnatter && go test -count=1 ./...)
for t in tests/test_qnatter_*.sh; do "$t" || exit $?; done
node --check luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js
node --check luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/status.js
node --check luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/log.js
```

## 许可证

本项目采用 [GNU General Public License v3.0](LICENSE) 发布，SPDX 标识为 `GPL-3.0-only`。

你可以依照 GPLv3 使用、研究、修改和分发本项目。分发源代码或构建产物时，应保留适用的版权与许可证声明，并按 GPLv3 提供相应源代码。完整条款以仓库根目录的 [LICENSE](LICENSE) 为准。

上游 [MikeWang000000/Natter](https://github.com/MikeWang000000/Natter) 同样采用 GNU GPL v3.0；本仓库保留并遵循其许可证要求。

## 致谢

NAT 打洞核心思路与上游项目来自 [MikeWang000000/Natter](https://github.com/MikeWang000000/Natter)。本仓库是在该项目基础上进行 OpenWrt/ImmortalWrt 打包、LuCI 配置界面、多实例策略路由、qBittorrent 和 Cloudflare SRV 集成。
