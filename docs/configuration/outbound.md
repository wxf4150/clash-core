---
sidebarTitle: Outbound
sidebarOrder: 4
---

# Outbound

There are several types of outbound targets in Clash. Each type has its own features and usage scenarios. In this page, we'll cover the common features of each type and how they should be used and configured.

[[toc]]

## Proxies

Proxies are some outbound targets that you can configure. Like proxy servers, you define destinations for the packets here.

### Shadowsocks

Clash supports the following ciphers (encryption methods) for Shadowsocks:

| Family | Ciphers |
| ------ | ------- |
| AEAD | aes-128-gcm, aes-192-gcm, aes-256-gcm, chacha20-ietf-poly1305, xchacha20-ietf-poly1305 |
| Stream | aes-128-cfb, aes-192-cfb, aes-256-cfb, rc4-md5, chacha20-ietf, xchacha20 |
| Block | aes-128-ctr, aes-192-ctr, aes-256-ctr |

In addition, Clash also supports popular Shadowsocks plugins `obfs` and `v2ray-plugin`.

::: code-group

```yaml [basic]
- name: "ss1"
  type: ss
  # interface-name: eth0
  # routing-mark: 1234
  server: server
  port: 443
  cipher: chacha20-ietf-poly1305
  password: "password"
  # udp: true
```

```yaml [obfs]
- name: "ss2"
  type: ss
  # interface-name: eth0
  # routing-mark: 1234
  server: server
  port: 443
  cipher: chacha20-ietf-poly1305
  password: "password"
  plugin: obfs
  plugin-opts:
    mode: tls # or http
    # host: bing.com
```

```yaml [ws (websocket)]
- name: "ss3"
  type: ss
  # interface-name: eth0
  # routing-mark: 1234
  server: server
  port: 443
  cipher: chacha20-ietf-poly1305
  password: "password"
  plugin: v2ray-plugin
  plugin-opts:
    mode: websocket # no QUIC now
    # tls: true # wss
    # skip-cert-verify: true
    # host: bing.com
    # path: "/"
    # mux: true
    # headers:
    #   custom: value
```

:::

### ShadowsocksR

Clash supports the infamous anti-censorship protocol ShadowsocksR as well. The supported ciphers:

| Family | Ciphers |
| ------ | ------- |
| Stream | aes-128-cfb, aes-192-cfb, aes-256-cfb, rc4-md5, chacha20-ietf, xchacha20 |

Supported obfuscation methods:

- plain
- http_simple
- http_post
- random_head
- tls1.2_ticket_auth
- tls1.2_ticket_fastauth

Supported protocols:

- origin
- auth_sha1_v4
- auth_aes128_md5
- auth_aes128_sha1
- auth_chain_a
- auth_chain_b

```yaml
- name: "ssr"
  type: ssr
  # interface-name: eth0
  # routing-mark: 1234
  server: server
  port: 443
  cipher: chacha20-ietf
  password: "password"
  obfs: tls1.2_ticket_auth
  protocol: auth_sha1_v4
  # obfs-param: domain.tld
  # protocol-param: "#"
  # udp: true
```

### Vmess

Clash supports the following ciphers (encryption methods) for Vmess:

- auto
- aes-128-gcm
- chacha20-poly1305
- none

::: code-group

```yaml [basic]
- name: "vmess"
  type: vmess
  # interface-name: eth0
  # routing-mark: 1234
  server: server
  port: 443
  uuid: uuid
  alterId: 32
  cipher: auto
  # udp: true
  # tls: true
  # skip-cert-verify: true
  # servername: example.com # priority over wss host
  # network: ws
  # ws-opts:
  #   path: /path
  #   headers:
  #     Host: v2ray.com
  #   max-early-data: 2048
  #   early-data-header-name: Sec-WebSocket-Protocol
```

```yaml [HTTP]
- name: "vmess-http"
  type: vmess
  # interface-name: eth0
  # routing-mark: 1234
  server: server
  port: 443
  uuid: uuid
  alterId: 32
  cipher: auto
  # udp: true
  # network: http
  # http-opts:
  #   # method: "GET"
  #   # path:
  #   #   - '/'
  #   #   - '/video'
  #   # headers:
  #   #   Connection:
  #   #     - keep-alive
```

```yaml [HTTP/2]
- name: "vmess-h2"
  type: vmess
  # interface-name: eth0
  # routing-mark: 1234
  server: server
  port: 443
  uuid: uuid
  alterId: 32
  cipher: auto
  network: h2
  tls: true
  h2-opts:
    host:
      - http.example.com
      - http-alt.example.com
    path: /
```

```yaml [gRPC]
- name: vmess-grpc
  type: vmess
  # interface-name: eth0
  # routing-mark: 1234
  server: server
  port: 443
  uuid: uuid
  alterId: 32
  cipher: auto
  network: grpc
  tls: true
  servername: example.com
  # skip-cert-verify: true
  grpc-opts:
    grpc-service-name: "example"
```

:::

### SOCKS5

In addition, Clash supports SOCKS5 outbound as well:

```yaml
- name: "socks"
  type: socks5
  # interface-name: eth0
  # routing-mark: 1234
  server: server
  port: 443
  # username: username
  # password: password
  # tls: true
  # skip-cert-verify: true
  # udp: true
```

### HTTP

Clash also supports HTTP outbound:

::: code-group

```yaml [HTTP]
- name: "http"
  type: http
  # interface-name: eth0
  # routing-mark: 1234
  server: server
  port: 443
  # username: username
  # password: password
```

```yaml [HTTPS]
- name: "http"
  type: http
  # interface-name: eth0
  # routing-mark: 1234
  server: server
  port: 443
  tls: true
  # skip-cert-verify: true
  # sni: custom.com
  # username: username
  # password: password
```

:::

### Snell

Being an alternative protocol for anti-censorship, Clash has integrated support for Snell as well.

::: tip
Clash does not support Snell v4. ([#2466](https://github.com/Dreamacro/clash/issues/2466))
:::

```yaml
# No UDP support yet
- name: "snell"
  type: snell
  # interface-name: eth0
  # routing-mark: 1234
  server: server
  port: 44046
  psk: yourpsk
  # version: 2
  # obfs-opts:
    # mode: http # or tls
    # host: bing.com
```

### Trojan

Clash has built support for the popular protocol Trojan:

::: code-group

```yaml [basic]
- name: "trojan"
  type: trojan
  # interface-name: eth0
  # routing-mark: 1234
  server: server
  port: 443
  password: yourpsk
  # udp: true
  # sni: example.com # aka server name
  # alpn:
  #   - h2
  #   - http/1.1
  # skip-cert-verify: true
```

```yaml [gRPC]
- name: trojan-grpc
  type: trojan
  # interface-name: eth0
  # routing-mark: 1234
  server: server
  port: 443
  password: "example"
  network: grpc
  sni: example.com
  # skip-cert-verify: true
  udp: true
  grpc-opts:
    grpc-service-name: "example"
```

```yaml  [ws (websocket)]
- name: trojan-ws
  type: trojan
  # interface-name: eth0
  # routing-mark: 1234
  server: server
  port: 443
  password: "example"
  network: ws
  sni: example.com
  # skip-cert-verify: true
  udp: true
  # ws-opts:
    # path: /path
    # headers:
    #   Host: example.com
```

:::

### SSH

Clash supports SSH as a proxy protocol. SSH proxy uses SSH tunneling to forward traffic through an SSH server. It supports password and private key authentication, automatic `~/.ssh/config` consultation, multi-hop (`proxy-jump`), and connection multiplexing.

::: tip
SSH proxy does not support UDP traffic. Only TCP connections are supported through SSH tunneling.
:::

::: code-group

```yaml [password]
- name: "ssh"
  type: ssh
  # interface-name: eth0
  # routing-mark: 1234
  server: server
  port: 22
  username: user
  password: "password"
```

```yaml [private-key]
- name: "ssh"
  type: ssh
  # interface-name: eth0
  # routing-mark: 1234
  server: server
  port: 22
  username: user
  privatekey: ~/.ssh/id_rsa
```

```yaml [multiple-private-keys]
- name: "ssh"
  type: ssh
  server: server
  port: 22
  username: user
  privatekey: ~/.ssh/id_rsa,~/.ssh/id_ed25519
```

```yaml [proxy-jump]
- name: "ssh-via-jump"
  type: ssh
  server: server
  port: 22
  username: user
  privatekey: ~/.ssh/id_rsa
  proxy-jump: jump-user@jump.example.com:22,another-jump.example.com
```

:::

SSH Configuration File Support

Clash will automatically attempt to read and parse `~/.ssh/config` (using github.com/kevinburke/ssh_config) when an SSH proxy is configured. Values from `~/.ssh/config` are used only when the corresponding field is not set in the Clash YAML; Clash YAML has priority. There is no `use-ssh-config` boolean flag — loading is automatic if the file exists.

Supported ssh_config options that may be used to fill missing fields:
- `Host` (match pattern)
- `HostName` (actual host)
- `Port`
- `User`
- `IdentityFile` (one or more identity files)
- `ProxyJump` (per-host jump hosts)
- `Password` (if present)

Notes:
- `privatekey` supports comma-separated paths and `~/` expansion to the user's home directory.
- If no identity file is provided, Clash will check for `~/.ssh/id_rsa` and `~/.ssh/id_ed25519` and use the first existing file(s).
- `proxy-jump` can be a comma-separated list. Each entry may be `user@host:port`, `host:port`, or `host`. Clash will try to load per-jump ssh_config settings (User, IdentityFile, HostName, Port) for each jump host and use them when available.
- Clash maintains a persistent SSH client per configured SSH proxy (multiplexing) to reuse connections; it will reconnect automatically if the client dies.
- Host key verification is disabled by default (ssh.InsecureIgnoreHostKey). This is insecure; consider the security implications.

## Proxy Groups

Proxy Groups are groups of proxies that you can use directly as a rule policy.

### relay

The request sent to this proxy group will be relayed through the specified proxy servers sequently. There's currently no UDP support on this. The specified proxy servers should not contain another relay.

### url-test

Clash benchmarks each proxy server in the list by measuring **network latency (delay)** through sending HTTP HEAD requests to a specified URL. The latency test runs automatically at startup and then periodically based on the `interval` setting (in seconds). You can configure a maximum tolerance value, the testing interval, and the target URL.

The group automatically selects the proxy with the lowest latency. When `lazy: true` is set, health checks only run when the group is actively used (within the interval period).

### fallback

Clash periodically tests the availability of servers in the list with the same mechanism of `url-test`. The first available server will be used.

### load-balance

The request to the same eTLD+1 will be dialed with the same proxy.

### select

The first server is by default used when Clash starts up. Users can choose the server to use with the RESTful API. In this mode, you can hardcode servers in the config or use [Proxy Providers](#proxy-providers).

Either way, sometimes you might as well just route packets with a direct connection. In this case, you can use the `DIRECT` outbound.

To use a different network interface, you will need to use a Proxy Group that contains a `DIRECT` outbound with the `interface-name` option set.

```yaml
- name: "My Wireguard Outbound"
  type: select
  interface-name: wg0
  proxies: [ 'DIRECT' ]
```

## Proxy Providers

Proxy Providers give users the power to load proxy server lists dynamically, instead of hardcoding them in the configuration file. There are currently two sources for a proxy provider to load server list from:

- `http`: Clash loads the server list from a specified URL on startup. Clash periodically pulls the server list from remote if the `interval` option is set.
- `file`: Clash loads the server list from a specified location on the filesystem on startup.

Health check is available for both modes, and works exactly like `fallback` in Proxy Groups. The configuration format for the server list files is also exactly the same in the main configuration file:

::: code-group

```yaml [config.yaml]
proxy-providers:
  provider1:
    type: http
    url: "url"
    interval: 3600
    path: ./provider1.yaml
    # filter: 'a|b' # golang regex string
    health-check:
      enable: true
      interval: 600
      # lazy: true
      url: http://www.gstatic.com/generate_204
  test:
    type: file
    path: /test.yaml
    health-check:
      enable: true
      interval: 36000
      url: http://www.gstatic.com/generate_204
```

```yaml [test.yaml]
proxies:
  - name: "ss1"
    type: ss
    server: server
    port: 443
    cipher: chacha20-ietf-poly1305
    password: "password"

  - name: "ss2"
    type: ss
    server: server
    port: 443
    cipher: chacha20-ietf-poly1305
    password: "password"
    plugin: obfs
    plugin-opts:
      mode: tls
```

:::
