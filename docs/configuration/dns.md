---
sidebarTitle: Clash DNS
sidebarOrder: 6
---

# Clash DNS

Since some parts of Clash run on the Layer 3 (Network Layer), they would've been impossible to obtain domain names of the packets for rule-based routing.

*Enter fake-ip*. It enables rule-based routing, minimises the impact of DNS pollution attack and improves network performance, sometimes drastically.

## Why DNS Configuration is Optional

DNS configuration in Clash is **optional** because Clash can operate in different modes:

1. **When DNS is disabled** (`enable: false` or DNS section omitted): Clash uses the **system's default DNS resolver** to resolve domain names. This is the simplest setup but provides no protection against DNS pollution.

2. **When DNS is enabled** (`enable: true`): Clash uses its own DNS resolver with the configured nameservers, which provides:
   - Protection against DNS pollution
   - Support for fake-ip mode for better performance
   - Custom DNS routing policies
   - DoH (DNS over HTTPS) and DoT (DNS over TLS) support

## How DNS Listen Address Works

The `listen` option in DNS configuration controls whether Clash provides a DNS server:

- **With listen address** (e.g., `listen: 0.0.0.0:53`): Clash starts a DNS server on the specified address and port. Other devices on your network can use Clash as their DNS server.
- **Without listen address**: Clash's DNS resolver is only used internally for Clash's own traffic routing. No external DNS server is exposed.

## DNS Resolution Behavior

### When DNS is Enabled

When you enable DNS in the configuration with `enable: true`:

1. Clash creates an internal DNS resolver using the configured `nameserver` list
2. All domain name resolution for Clash's traffic routing goes through this internal resolver
3. If `listen` is configured, Clash also provides a DNS server for other applications
4. The configured nameservers (e.g., `8.8.8.8`, `1.1.1.1`) are used to resolve domain names

### When DNS is Disabled

When DNS is disabled or not configured:

1. Clash uses the **system's default DNS resolver** (typically `/etc/resolv.conf` on Linux, system DNS settings on Windows/macOS)
2. No DNS server is provided even if `listen` is configured
3. No fake-ip or enhanced DNS features are available

## fake-ip

The concept of "fake IP" addresses is originated from [RFC 3089](https://tools.ietf.org/rfc/rfc3089):

> A "fake IP" address is used as a key to look up the corresponding "FQDN" information.

The default CIDR for the fake-ip pool is `198.18.0.1/16`, a reserved IPv4 address space, which can be changed in `dns.fake-ip-range`.

### How fake-ip Works

When a DNS request is sent to the Clash DNS, the core allocates a *free* fake-ip address from the pool, by managing an internal mapping of domain names and their fake-ip addresses.

**Key Points:**

1. **fake-ip addresses are virtual**: The `198.18.0.0/16` range is a reserved address space. These IP addresses are not real network IPs. Clash only maintains an in-memory mapping between domain names and fake-ip addresses.

2. **Clash handles the entire fake-ip range**: When your system or applications send packets to fake-ip addresses, these packets are routed to Clash (via transparent proxy, TUN mode, or system proxy settings). Clash doesn't need to listen on each individual fake-ip address - it handles all fake-ip traffic through:
   - **Transparent proxy mode** (redir/tproxy): Uses iptables/nftables rules to redirect all traffic destined for the fake-ip range to Clash
   - **TUN mode**: Clash creates a virtual network interface and takes over routing for the entire fake-ip range
   - **System proxy mode**: Applications send all traffic to Clash's proxy port

3. **Why fake-ip improves network performance**:
   - **Avoids DNS query latency**: Applications get a fake-ip immediately and can establish connections without waiting for real DNS resolution
   - **Defers DNS resolution**: Clash only resolves real IPs when necessary (e.g., for `GEOIP` or `IP-CIDR` rules). For proxy protocols that forward domain names directly (like SOCKS5/VMess), DNS resolution is completely skipped
   - **Reduces DNS leaks**: All DNS queries are handled internally by Clash, avoiding leakage of real DNS requests upstream
   - **Parallel connections**: Applications can initiate connections without waiting for DNS resolution, improving concurrency

### System Configuration Requirements for fake-ip

For fake-ip to work properly, the following configuration is required:

**1. DNS Configuration (Required)**

First, you need to set your system DNS to Clash's DNS server address. This can be done by:

- **Manual configuration**: Change system DNS settings to point to Clash DNS listen address (e.g., `127.0.0.1:53`)
- **DHCP configuration**: If Clash runs on a router, distribute Clash DNS address via DHCP
- **System-level configuration**: Modify `/etc/resolv.conf` (Linux) or network settings (Windows/macOS)

Enable DNS in Clash configuration:

```yaml
dns:
  enable: true
  listen: 0.0.0.0:53  # Provide DNS service
  enhanced-mode: fake-ip
  fake-ip-range: 198.18.0.1/16
  nameserver:
    - 8.8.8.8
    - 1.1.1.1
```

**2. Traffic Routing Configuration (Mode-Dependent)**

After fake-ip returns virtual IP addresses, traffic destined for the fake-ip range needs to be routed to Clash. Different modes require different configuration:

**System Proxy Mode (Simplest)**
- Applications use system proxy settings to send all traffic to Clash's HTTP/SOCKS5 port
- **No additional configuration needed**: Applications automatically send traffic to Clash
- **Limitation**: Only works for applications that respect proxy settings

**Transparent Proxy Mode (Manual Configuration Required)**
- Uses iptables (Linux) or pf (macOS) rules to redirect fake-ip range traffic to Clash
- **Manual configuration required**: iptables/pf rules must be created at startup
- **Example (Linux iptables)**:
  ```bash
  # Redirect TCP traffic to Clash redir port
  iptables -t nat -A OUTPUT -d 198.18.0.0/16 -p tcp -j REDIRECT --to-ports 7892
  # Redirect UDP traffic (tproxy mode)
  iptables -t mangle -A OUTPUT -d 198.18.0.0/16 -p udp -j TPROXY --on-port 7893
  ```
- **Note**: These rules need manual maintenance and must be reconfigured after each reboot

**TUN Mode (Automatic Configuration - Premium Version)**
- Clash Premium supports TUN mode with **automatic management** of route tables and rules
- **Automatic configuration**: With `tun.auto-route: true`, Clash automatically configures routes
- **Example configuration**:
  ```yaml
  tun:
    enable: true
    stack: system
    dns-hijack:
      - any:53
    auto-route: true  # Automatically configure routing rules
    auto-detect-interface: true
  ```
- **Advantage**: No manual iptables configuration needed. Clash automatically sets up rules on startup and cleans up on exit
- **Note**: Requires administrator privileges. See [TUN Device documentation](/premium/tun-device) for details

**Configuration Summary**:
1. Configure Clash DNS (required)
2. Set system DNS to point to Clash (manual configuration)
3. Choose traffic routing mode:
   - System proxy: Configure application proxy settings (manual)
   - Transparent proxy: Configure iptables/pf rules (manual, on each startup)
   - TUN mode: Enable `auto-route` (automatic, recommended)

Take an example of accessing `http://google.com` with your browser.

1. The browser asks Clash DNS for the IP address of `google.com`
2. Clash checks the internal mapping and returned `198.18.1.5`
3. The browser sends an HTTP request to `198.18.1.5` on `80/tcp`
4. When receiving the inbound packet for `198.18.1.5`, Clash looks up the internal mapping and realises the client is actually sending a packet to `google.com`
5. Depending on the rules:

    1. Clash may just send the domain name to an outbound proxy like SOCKS5 or shadowsocks and establish the connection with the proxy server

    2. or Clash might look for the real IP address of `google.com`, in the case of encountering a `SCRIPT`, `GEOIP`, `IP-CIDR` rule, or the case of DIRECT outbound

Being a confusing concept, I'll take another example of accessing `http://google.com` with the cURL utility:

```txt{2,3,5,6,8,9}
$ curl -v http://google.com
<---- cURL asks your system DNS (Clash) about the IP address of google.com
----> Clash decided 198.18.1.70 should be used as google.com and remembers it
*   Trying 198.18.1.70:80...
<---- cURL connects to 198.18.1.70 tcp/80
----> Clash will accept the connection immediately, and..
* Connected to google.com (198.18.1.70) port 80 (#0)
----> Clash looks up in its memory and found 198.18.1.70 being google.com
----> Clash looks up in the rules and sends the packet via the matching outbound
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
