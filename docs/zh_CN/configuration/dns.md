---
sidebarTitle: Clash DNS
sidebarOrder: 6
---

# Clash DNS

由于 Clash 的某些部分运行在第 3 层 (网络层) , 因此其数据包的域名是无法获取的, 也就无法进行基于规则的路由.

*Enter fake-ip*: 它支持基于规则的路由, 最大程度地减少了 DNS 污染攻击的影响, 并且提高了网络性能, 有时甚至是显著的.

## 为什么 DNS 配置是可选的

Clash 中的 DNS 配置是**可选的**, 因为 Clash 可以在不同模式下运行:

1. **当 DNS 被禁用时** (`enable: false` 或省略 DNS 配置段): Clash 使用**系统默认的 DNS 解析器**来解析域名. 这是最简单的配置方式, 但无法防止 DNS 污染.

2. **当 DNS 被启用时** (`enable: true`): Clash 使用自己的 DNS 解析器和配置的域名服务器, 这提供了:
   - 防止 DNS 污染
   - 支持 fake-ip 模式以获得更好的性能
   - 自定义 DNS 路由策略
   - 支持 DoH (DNS over HTTPS) 和 DoT (DNS over TLS)

## DNS 监听地址的工作原理

DNS 配置中的 `listen` 选项控制 Clash 是否提供 DNS 服务器:

- **配置了监听地址** (例如 `listen: 0.0.0.0:53`): Clash 在指定的地址和端口上启动 DNS 服务器. 网络中的其他设备可以使用 Clash 作为它们的 DNS 服务器.
- **未配置监听地址**: Clash 的 DNS 解析器仅在内部用于 Clash 自身的流量路由. 不会对外暴露 DNS 服务器.

## DNS 解析行为

### 当 DNS 被启用时

当您在配置中使用 `enable: true` 启用 DNS 时:

1. Clash 使用配置的 `nameserver` 列表创建内部 DNS 解析器
2. Clash 流量路由的所有域名解析都通过这个内部解析器
3. 如果配置了 `listen`, Clash 还会为其他应用程序提供 DNS 服务器
4. 配置的域名服务器 (例如 `8.8.8.8`, `1.1.1.1`) 将用于解析域名

### 当 DNS 被禁用时

当 DNS 被禁用或未配置时:

1. Clash 使用**系统默认的 DNS 解析器** (通常是 Linux 上的 `/etc/resolv.conf`, Windows/macOS 上的系统 DNS 设置)
2. 即使配置了 `listen` 也不会提供 DNS 服务器
3. fake-ip 或增强的 DNS 功能均不可用

## fake-ip

"fake IP" 的概念源自 [RFC 3089](https://tools.ietf.org/rfc/rfc3089):

> 一个 "fake IP" 地址被用于查询相应的 "FQDN" 信息的关键字.

fake-ip 池的默认 CIDR 是 `198.18.0.1/16` (一个保留的 IPv4 地址空间, 可以在 `dns.fake-ip-range` 中进行更改).

### fake-ip 的工作原理

当 DNS 请求被发送到 Clash DNS 时, Clash 内核会通过管理内部的域名和其 fake-ip 地址的映射, 从池中分配一个 *空闲* 的 fake-ip 地址.

**关键要点:**

1. **fake-ip 是虚拟 IP 地址**: `198.18.0.0/16` 网段是一个保留的地址空间, 这些 IP 地址不是真实的网络 IP. Clash 只是在内存中维护域名到 fake-ip 的映射关系.

2. **Clash 处理整个 fake-ip 网段**: 当您的系统或应用向 fake-ip 地址发送数据包时, 这些数据包会被路由到 Clash (通过透明代理、TUN 模式或系统代理设置). Clash 不需要监听每个单独的 fake-ip 地址 - 它通过以下机制处理所有 fake-ip 流量:
   - **透明代理模式** (redir/tproxy): 使用 iptables/nftables 规则将所有目标为 fake-ip 网段的流量重定向到 Clash
   - **TUN 模式**: Clash 创建虚拟网络接口, 接管整个 fake-ip 网段的路由
   - **系统代理模式**: 应用程序将所有流量发送到 Clash 的代理端口

3. **为什么 fake-ip 能提升网络速度**:
   - **避免 DNS 查询延迟**: 应用程序获得 fake-ip 后可以立即建立连接, 不需要等待真实的 DNS 解析完成
   - **延迟 DNS 解析**: Clash 仅在必要时 (如遇到 `GEOIP` 或 `IP-CIDR` 规则) 才解析真实 IP, 对于直接转发域名的代理协议 (如 SOCKS5/VMess) 则完全跳过 DNS 解析
   - **减少 DNS 泄漏**: 所有 DNS 查询都由 Clash 内部处理, 避免向上游泄漏真实的 DNS 请求
   - **并行连接**: 应用程序不需要等待 DNS 解析就能发起连接, 提高并发性能

### fake-ip 的系统配置要求

要使 fake-ip 正常工作, 需要进行以下配置:

**1. DNS 配置 (必需)**

首先, 您需要将系统 DNS 设置为 Clash 的 DNS 服务器地址. 这可以通过以下方式实现:

- **手动配置**: 修改系统 DNS 设置指向 Clash DNS 监听地址 (例如 `127.0.0.1:53`)
- **DHCP 配置**: 如果 Clash 运行在路由器上, 可以通过 DHCP 分发 Clash DNS 地址
- **系统级配置**: 修改 `/etc/resolv.conf` (Linux) 或网络设置 (Windows/macOS)

在 Clash 配置中启用 DNS:

```yaml
dns:
  enable: true
  listen: 0.0.0.0:53  # 提供 DNS 服务
  enhanced-mode: fake-ip
  fake-ip-range: 198.18.0.1/16
  nameserver:
    - 8.8.8.8
    - 1.1.1.1
```

**2. 流量路由配置 (取决于模式)**

fake-ip 返回虚拟 IP 地址后, 还需要将发往 fake-ip 网段的流量路由到 Clash. 不同模式的配置方式不同:

**系统代理模式 (最简单)**
- 应用程序使用系统代理设置, 将所有流量发送到 Clash 的 HTTP/SOCKS5 端口
- **无需额外配置**: 应用程序自动将流量发送到 Clash
- **局限性**: 仅支持配置了代理的应用程序

**透明代理模式 (需要手动配置)**
- 使用 iptables (Linux) 或 pf (macOS) 规则将 fake-ip 网段流量重定向到 Clash
- **需要手动配置**: 启动时需要创建 iptables/pf 规则
- **示例 (Linux iptables)**:
  ```bash
  # 重定向 TCP 流量到 Clash redir 端口
  iptables -t nat -A OUTPUT -d 198.18.0.0/16 -p tcp -j REDIRECT --to-ports 7892
  # 重定向 UDP 流量 (tproxy 模式)
  iptables -t mangle -A OUTPUT -d 198.18.0.0/16 -p udp -j TPROXY --on-port 7893
  ```
- **注意**: 这些规则需要手动维护, 每次重启后需要重新配置

**TUN 模式 (自动配置 - Premium 版本)**
- Clash Premium 支持 TUN 模式, 可以**自动管理**路由表和规则
- **自动配置**: 启用 `tun.auto-route: true` 后, Clash 会自动配置路由
- **示例配置**:
  ```yaml
  tun:
    enable: true
    stack: system
    dns-hijack:
      - any:53
    auto-route: true  # 自动配置路由规则
    auto-detect-interface: true
  ```
- **优势**: 无需手动配置 iptables 规则, Clash 在启动时自动设置, 退出时自动清理
- **注意**: 需要管理员权限运行, 详见 [TUN 设备文档](/zh_CN/premium/tun-device)

**总结配置步骤**:
1. 配置 Clash DNS (必需)
2. 设置系统 DNS 指向 Clash (手动配置)
3. 选择流量路由模式:
   - 系统代理: 配置应用程序代理设置 (手动)
   - 透明代理: 配置 iptables/pf 规则 (手动, 每次启动)
   - TUN 模式: 启用 `auto-route` (自动, 推荐)

以使用浏览器访问 `http://google.com` 为例.

1. 浏览器向 Clash DNS 请求 `google.com` 的 IP 地址
2. Clash 检查内部映射并返回 `198.18.1.5`
3. 浏览器向 `198.18.1.5` 的 `80/tcp` 端口发送 HTTP 请求
4. 当收到 `198.18.1.5` 的入站数据包时, Clash 查询内部映射, 发现客户端实际上是在向 `google.com` 发送数据包
5. 根据规则的不同:

    1. Clash 可能仅将域名发送到 SOCKS5 或 shadowsocks 等出站代理, 并与代理服务器建立连接

    2. 或者 Clash 可能会基于 `SCRIPT`、`GEOIP`、`IP-CIDR` 规则或者使用 DIRECT 直连出口查询 `google.com` 的真实 IP 地址

由于这是一个令人困惑的概念, 我将以使用 cURL 程序访问 `http://google.com` 为例:

```txt{2,3,5,6,8,9}
$ curl -v http://google.com
<---- cURL 向您的系统 DNS (Clash) 询问 google.com 的 IP 地址
----> Clash 决定使用 198.18.1.70 作为 google.com 的 IP 地址, 并记住它
*   Trying 198.18.1.70:80...
<---- cURL 连接到 198.18.1.70 tcp/80
----> Clash 将立即接受连接, 并且..
* Connected to google.com (198.18.1.70) port 80 (#0)
----> Clash 在其内存中查找到 198.18.1.70 对应于 google.com
----> Clash 查询对应的规则, 并通过匹配的出口发送数据包
> GET / HTTP/1.1
> Host: google.com
> User-Agent: curl/8.0.1
> Accept: */*
>
< HTTP/1.1 301 Moved Permanently
< Location: http://www.google.com/
< Content-Type: text/html; charset=UTF-8
< Content-Security-Policy-Report-Only: object-src 'none';base-uri 'self';script-src 'nonce-ahELFt78xOoxhySY2lQ34A' 'strict-dynamic' 'report-sample' 'unsafe-eval' 'unsafe-inline' https: http:;report-uri https://csp.withgoogle.com/csp/gws/other-hp
< Date: Thu, 11 May 2023 06:52:19 GMT
< Expires: Sat, 10 Jun 2023 06:52:19 GMT
< Cache-Control: public, max-age=2592000
< Server: gws
< Content-Length: 219
< X-XSS-Protection: 0
< X-Frame-Options: SAMEORIGIN
<
<HTML><HEAD><meta http-equiv="content-type" content="text/html;charset=utf-8">
<TITLE>301 Moved</TITLE></HEAD><BODY>
<H1>301 Moved</H1>
The document has moved
<A HREF="http://www.google.com/">here</A>.
</BODY></HTML>
* Connection #0 to host google.com left intact
```

<!-- TODO: nameserver, fallback, fallback-filter, hosts, search-domains, fake-ip-filter, nameserver-policy -->
