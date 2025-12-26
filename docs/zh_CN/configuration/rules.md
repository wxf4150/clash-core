---
sidebarTitle: Rules 规则
sidebarOrder: 5
---

# Rules 规则

在[快速入手](/zh_CN/configuration/getting-started)中, 我们介绍了Clash中基于规则的匹配的基本知识. 在本章中, 我们将介绍最新版本的 Clash 中所有可用的规则类型.

```txt
# 类型,参数,策略(,no-resolve)
TYPE,ARGUMENT,POLICY(,no-resolve)
```

`no-resolve` 选项是可选的, 它用于跳过规则的 DNS 解析. 当您想要使用 `GEOIP`、`IP-CIDR`、`IP-CIDR6`、`SCRIPT` 规则, 但又不想立即将域名解析为 IP 地址时, 这个选项就很有用了.

[[toc]]

## 策略

目前有四种策略类型, 其中:

- DIRECT: 通过 `interface-name` 直接连接到目标 (不查找系统路由表)
- REJECT: 丢弃数据包
- Proxy: 将数据包路由到指定的代理服务器
- Proxy Group: 将数据包路由到指定的策略组

## 规则类型

以下部分介绍了每种规则类型及其使用方法:

### DOMAIN 域名

`DOMAIN,www.google.com,policy` 将 `www.google.com` 路由到 `policy`.

### DOMAIN-SUFFIX 域名后缀

`DOMAIN-SUFFIX,youtube.com,policy` 将任何以 `youtube.com` 结尾的域名路由到 `policy`.

在这种情况下, `www.youtube.com` 和 `foo.bar.youtube.com` 都将路由到 `policy`.

### DOMAIN-KEYWORD 域名关键字

`DOMAIN-KEYWORD,google,policy` 将任何包含 `google` 关键字的域名路由到 `policy`.

在这种情况下, `www.google.com` 或 `googleapis.com` 都将路由到 `policy`.

### GEOIP IP地理位置 (国家代码)

GEOIP 规则用于根据数据包的目标 IP 地址的**国家代码**路由数据包. Clash 使用 [MaxMind GeoLite2](https://dev.maxmind.com/geoip/geoip2/geolite2/) 数据库来实现这一功能.

::: warning
使用这种规则时, Clash 将域名解析为 IP 地址, 然后查找 IP 地址的国家代码.
如果要跳过 DNS 解析, 请使用 `no-resolve` 选项.
:::

`GEOIP,CN,policy` 将任何目标 IP 地址为中国的数据包路由到 `policy`.

### IP-CIDR IPv4地址段

IP-CIDR 规则用于根据数据包的**目标 IPv4 地址**路由数据包.

::: warning
使用这种规则时, Clash 将域名解析为 IPv4 地址.
如果要跳过 DNS 解析, 请使用 `no-resolve` 选项.
:::

`IP-CIDR,127.0.0.0/8,DIRECT` 将任何目标 IP 地址为 `127.0.0.0/8` 的数据包路由到 `DIRECT`.

### IP-CIDR6 IPv6地址段

IP-CIDR6 规则用于根据数据包的**目标 IPv6 地址**路由数据包.

::: warning
使用这种规则时, Clash 将域名解析为 IPv6 地址.
如果要跳过 DNS 解析, 请使用 `no-resolve` 选项.
:::

`IP-CIDR6,2620:0:2d0:200::7/32,policy` 将任何目标 IP 地址为 `2620:0:2d0:200::7/32` 的数据包路由到 `policy`.

### SRC-IP-CIDR 源IP段地址

SRC-IP-CIDR 规则用于根据数据包的**源 IPv4 地址**路由数据包.

`SRC-IP-CIDR,192.168.1.201/32,DIRECT` 将任何源 IP 地址为 `192.168.1.201/32` 的数据包路由到 `DIRECT`.

### SRC-PORT 源端口

SRC-PORT 规则用于根据数据包的**源端口**路由数据包.

`SRC-PORT,80,policy` 将任何源端口为 `80` 的数据包路由到 `policy`.

### DST-PORT 目标端口

DST-PORT 规则用于根据数据包的**目标端口**路由数据包.

`DST-PORT,80,policy` 将任何目标端口为 `80` 的数据包路由到 `policy`.

### PROCESS-NAME 源进程名

PROCESS-NAME 规则用于根据发送数据包的进程名称路由数据包.

::: warning
目前, 仅支持 macOS、Linux、FreeBSD 和 Windows.
:::

`PROCESS-NAME,nc,DIRECT` 将任何来自进程 `nc` 的数据包路由到 `DIRECT`.

### PROCESS-PATH 源进程路径

PROCESS-PATH 规则用于根据发送数据包的进程路径路由数据包.

::: warning
目前, 仅支持 macOS、Linux、FreeBSD 和 Windows.
:::

`PROCESS-PATH,/usr/local/bin/nc,DIRECT` 将任何来自路径为 `/usr/local/bin/nc` 的进程的数据包路由到 `DIRECT`.

### IPSET IP集

IPSET 规则用于根据 IP 集匹配并路由数据包. 根据 [IPSET 的官方网站](https://ipset.netfilter.org/) 的介绍:

> IP 集是 Linux 内核中的一个框架, 可以通过 ipset 程序进行管理. 根据类型, IP 集可以存储 IP 地址、网络、 (TCP/UDP) 端口号、MAC 地址、接口名称或它们以某种方式的组合, 以确保在集合中匹配条目时具有闪电般的速度.

因此, 此功能仅在 Linux 上工作, 并且需要安装 `ipset`.

::: warning
使用此规则时, Clash 将解析域名以获取 IP 地址, 然后查找 IP 地址是否在 IP 集中.
如果要跳过 DNS 解析, 请使用 `no-resolve` 选项.
:::

`IPSET,chnroute,policy` 将任何目标 IP 地址在 IP 集 `chnroute` 中的数据包路由到 `policy`.

### RULE-SET 规则集

::: info
此功能仅在 [Premium 版本](/zh_CN/premium/introduction) 中可用.
:::

RULE-SET 规则用于根据 [Rule Providers 规则集](/zh_CN/premium/rule-providers) 的结果路由数据包. 当 Clash 使用此规则时, 它会从指定的 Rule Providers 规则集中加载规则, 然后将数据包与规则进行匹配. 如果数据包与任何规则匹配, 则将数据包路由到指定的策略, 否则跳过此规则.

::: warning
使用 RULE-SET 时, 当规则集的类型为 IPCIDR , Clash 将解析域名以获取 IP 地址.
如果要跳过 DNS 解析, 请使用 `no-resolve` 选项.
:::

`RULE-SET,my-rule-provider,DIRECT` 从 `my-rule-provider` 加载所有规则

### SCRIPT 脚本

::: info
此功能仅在 [Premium 版本](/zh_CN/premium/introduction) 中可用.
:::

SCRIPT 规则用于根据脚本的结果路由数据包. 当 Clash 使用此规则时, 它会执行指定的脚本, 然后将数据包路由到脚本的输出.

::: warning
使用 SCRIPT 时, Clash 将解析域名以获取 IP 地址.
如果要跳过 DNS 解析, 请使用 `no-resolve` 选项.
:::

`SCRIPT,script-path,DIRECT` 将数据包路由到脚本 `script-path` 的输出.

### MATCH 全匹配

MATCH 规则用于路由剩余的数据包. 该规则是**必需**的, 通常用作最后一条规则.

`MATCH,policy` 将剩余的数据包路由到 `policy`.

## 规则处理流程

下面是 Clash 在评估规则时的典型处理流程（简化说明）：

1. 规则按配置文件中的顺序自上而下依次评估。遇到第一条匹配的规则即停止匹配并按该规则的策略处理（first-match wins）。
2. 对于只有域名信息的连接，Clash 可能需要先做 DNS 解析以获得目标 IP；某些规则（如 GEOIP、IP-CIDR、IPSET、SCRIPT、以及 RULE-SET 中的 IPCIDR 类型）会触发这种解析，除非在对应规则末尾加上 `no-resolve` 来跳过解析。
3. 如果连接已经有目标 IP（比如本地直连或已由 DNS 缓存解析），IP 相关规则（IP-CIDR、IPSET、GEOIP）会直接基于 IP 做快速匹配，避免再次 DNS 查询。
4. PROCESS-*、SRC-*、PORT 类规则基于本地元信息（来源进程、源/目标端口等）进行匹配，这类规则在支持的平台上可以非常精确地控制流量，但也应注意顺序与优先级。
5. RULE-SET 与 SCRIPT 规则会调用外部资源或执行脚本，其内部匹配逻辑可能比普通域名/后缀匹配更复杂，且通常会带有加载/缓存延迟。
6. 未命中的流量最终由 `MATCH` 规则接管（因此 `MATCH` 应放在规则最后）。

> 小结：理解“自上而下、首次匹配生效”是掌握规则行为的核心；同时注意哪些规则会触发 DNS 解析，以避免不必要的解析开销。

## 如何高效配置规则（实践建议）

下面给出一组实用的原则与示例，帮助你编写既正确又高效的规则配置：

- 总原则 — 从最精确到最泛化：把最具体、匹配代价小或匹配范围广但开销低的规则放在前面，把代价高或广泛匹配的规则放后面。

- 推荐顺序（性能友好）：
  1. IP-CIDR / IP-CIDR6（明确的 IP 段）
  2. IPSET（Linux 上极快的集合匹配）
  3. GEOIP（按国家分流；若不希望解析域名请用 `no-resolve`）
  4. SRC-IP-CIDR / SRC-PORT / DST-PORT / PROCESS-NAME / PROCESS-PATH（基于本机元信息的精确规则）
  5. RULE-SET（Rule Provider，便于管理大量规则，放在合适位置以覆盖大量目标）
  6. DOMAIN / DOMAIN-SUFFIX（精确域名或后缀）
  7. DOMAIN-KEYWORD（关键字匹配最宽松且最耗费匹配时间，应放后面）
  8. SCRIPT（有外部开销或执行延迟的脚本规则，尽量放在靠后且做好缓存）
  9. MATCH（最后的兜底规则）

- 使用 `no-resolve`：
  - 当规则来自于 IP 列表或您不希望在匹配时触发 DNS（例如 RULE-SET 中包含 IPCIDR 或您用到了 GEOIP 而不想解析域名）时，使用 `,no-resolve` 可以避免额外的 DNS 请求，从而减少时延与外部依赖。示例：

    RULE-SET,cnip-lists,DIRECT,no-resolve

- 把大量静态或经常更新的列表放到 Rule Providers（RULE-SET）里管理：
  - 把像广告屏蔽、地区路由这类长规则通过 provider 引入，既便于维护，又能让主 rules 部分更紧凑。

- 用 IP 段替代大量域名规则：
  - 当你能用 CIDR 或 GEOIP 表示时，优先使用 IP-CIDR/GEOIP，匹配速度更快且更可靠（避免域名重定向或 CDN 干扰）。

- 在 Linux 上优先考虑 IPSET：
  - IPSET 由内核支持，匹配速度极快，适合包含大量 IP 的集合（如 chnroute、gfwlist 的 IP 变体等）。

- 降低 DOMAIN-KEYWORD 的使用：
  - 关键词匹配会检查子串，可能误伤且开销较大；尽可能用后缀或精确域名替代。

- 避免规则重复与交叉覆盖：
  - 如果一条更泛的规则会覆盖后面的多条具体规则，会导致具体规则失效或配置冗余。保持规则互不冲突并遵循“从特殊到通用”的顺序。

- 控制 SCRIPT 与外部调用开销：
  - 如果使用 SCRIPT，请尽量让脚本自身做缓存并快速返回，避免在每次连接时阻塞主流程。

- 性能调优小贴士：
  - 将常命中且需要高性能的规则放在文件顶部。
  - 把大型静态表（如广告/追踪/区域表）转为 provider 并定期更新，而不是把这些条目直接塞进主规则文件。
  - 使用合理的 DNS 缓存与本地解析策略，减少因规则触发的重复解析。

### 示例（推荐的规则片段）

以下是一个按性能建议排序的简单示例（仅示意）：

```
# 先用明确的 IP 段
IP-CIDR,10.0.0.0/8,DIRECT
IP-CIDR6,fd00::/8,DIRECT

# IP 集（Linux）
IPSET,chnroute,DIRECT

# 按国家分流（如需禁止某些国家走代理）
GEOIP,CN,DIRECT,no-resolve

# 本机进程或端口策略
PROCESS-NAME,update-manager,DIRECT
SRC-PORT,53,DIRECT

# 大型远程列表，放到 provider 管理
RULE-SET,gfwlist,Proxy

# 精确域名/后缀
DOMAIN-SUFFIX,google.com,Proxy
DOMAIN,accounts.youtube.com,Proxy

# 关键词匹配（放在后面）
DOMAIN-KEYWORD,video,Proxy

# 脚本（尽量缓存）
SCRIPT,./rules/scripts/custom_match.py,Proxy

# 最后一条兜底规则
MATCH,Proxy
```

## 小结

- 记住：规则按顺序匹配，首次命中生效。把常用且高性能的匹配放前面，昂贵或广泛匹配的放后面。充分利用 Rule Providers、IPSET、GEOIP 与 `no-resolve`，可以显著减少匹配开销并提升规则运作效率。
