package outbound

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/Dreamacro/clash/component/dialer"
	C "github.com/Dreamacro/clash/constant"
	"github.com/Dreamacro/clash/log"
	"github.com/kevinburke/ssh_config"
	"golang.org/x/crypto/ssh"
)

const (
	defaultSSHPort = 22
)

type Ssh struct {
	*Base
	user           string
	pass           string
	privateKeyPath string
	sshConfig      *ssh.ClientConfig
	proxyJump      string

	// Connection multiplexing
	clientMu  sync.Mutex
	client    *ssh.Client
	underConn net.Conn
}

type SshOption struct {
	BasicOption
	Name       string `proxy:"name"`
	Server     string `proxy:"server"`
	Port       int    `proxy:"port,omitempty"`
	UserName   string `proxy:"username,omitempty"`
	Password   string `proxy:"password,omitempty"`
	PrivateKey string `proxy:"privatekey,omitempty"`

	// ~/.ssh/config is read automatically (when present) by loadSSHConfig.
	// Clash YAML has priority over ssh_config values for any fields.

	ProxyJump string `proxy:"proxy-jump,omitempty"`
}

type sshConn struct {
	net.Conn
	client    *ssh.Client
	underConn net.Conn
}

func (c *sshConn) Close() error {
	// Close the tunnel connection first
	connErr := c.Conn.Close()
	// Close the SSH client
	clientErr := c.client.Close()
	// Close the underlying connection
	underErr := c.underConn.Close()

	// Return the first error encountered
	if connErr != nil {
		return connErr
	}
	if clientErr != nil {
		return clientErr
	}
	return underErr
}

func (c *sshConn) Read(b []byte) (int, error) {
	n, err := c.Conn.Read(b)
	if err != nil && err != io.EOF {
		_ = c.Close()
	}
	return n, err
}

func (c *sshConn) Write(b []byte) (int, error) {
	n, err := c.Conn.Write(b)
	if err != nil && err != io.EOF {
		_ = c.Close()
	}
	return n, err
}

// getOrCreateClient returns an active SSH client, creating one if necessary
func (ss *Ssh) getOrCreateClient(ctx context.Context, opts ...dialer.Option) (*ssh.Client, error) {
	ss.clientMu.Lock()
	defer ss.clientMu.Unlock()

	// Check if existing client is still alive
	if ss.client != nil {
		return ss.client, nil
	}
	log.Infoln("[SSH] %s@%s connecting...", ss.sshConfig.User, ss.addr)

	// If ProxyJump configured, build jump chain and dial through jumps
	if ss.proxyJump != "" {
		// Parse proxy jump list (comma separated)
		parts := strings.Split(ss.proxyJump, ",")
		var jumps []JumpHost
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}

			// support user@host:port or host:port or host
			var jumpUser string
			hostport := p
			if strings.Contains(p, "@") {
				idx := strings.Index(p, "@")
				jumpUser = p[:idx]
				hostport = p[idx+1:]
			}

			// prepare per-jump ssh config; inherit auth/hostkey/timeout from ss.sshConfig
			// Extract host portion (without port) for ssh_config lookup
			hostOnly := hostport
			if h, _, err := net.SplitHostPort(hostOnly); err == nil {
				hostOnly = h
			}

			// Try to load per-jump settings from ~/.ssh/config (clash YAML still has priority)
			var tmpOpt SshOption
			// Load values for the host pattern
			if o, err := loadSSHConfigForHost(hostOnly); err == nil {
				tmpOpt = o
			}

			// determine user: jumpUser > tmpOpt.UserName > main ss.sshConfig.User
			user := ss.sshConfig.User
			if tmpOpt.UserName != "" {
				user = tmpOpt.UserName
			}
			if jumpUser != "" {
				user = jumpUser
			}

			// build per-jump auth methods: prefer tmpOpt (from ssh_config) if present
			var authMethods []ssh.AuthMethod
			if tmpOpt.PrivateKey != "" {
				for _, keyPath := range strings.Split(tmpOpt.PrivateKey, ",") {
					if strings.HasPrefix(keyPath, "~/") {
						home, err := os.UserHomeDir()
						if err == nil {
							keyPath = filepath.Join(home, keyPath[2:])
						}
					}
					if key, err := os.ReadFile(keyPath); err == nil {
						if signer, err := ssh.ParsePrivateKey(key); err == nil {
							authMethods = append(authMethods, ssh.PublicKeys(signer))
						}
					}
				}
			}
			if tmpOpt.Password != "" {
				authMethods = append(authMethods, ssh.Password(tmpOpt.Password))
			}
			// fallback to main auth if no per-jump auth found
			if len(authMethods) == 0 {
				authMethods = ss.sshConfig.Auth
			}

			h := hostport
			// If ssh_config provides HostName, use it as the actual host to dial from jump
			if tmpOpt.Server != "" {
				// preserve the port from h
				if _, portPart, err := net.SplitHostPort(h); err == nil {
					h = net.JoinHostPort(tmpOpt.Server, portPart)
				} else {
					// if SplitHostPort failed, just replace host portion (rare)
					h = net.JoinHostPort(tmpOpt.Server, strconv.Itoa(tmpOpt.Port))
				}
			}

			cfg := &ssh.ClientConfig{
				User:            user,
				Auth:            authMethods,
				HostKeyCallback: ss.sshConfig.HostKeyCallback,
				Timeout:         ss.sshConfig.Timeout,
			}

			jumps = append(jumps, JumpHost{Addr: h, Config: cfg})
		}

		// append final target as last jump (use ss.sshConfig for final auth)
		jumps = append(jumps, JumpHost{Addr: ss.addr, Config: ss.sshConfig})

		client, firstConn, err := dialThroughJumps(ctx, dialer.DialContext, jumps, ss.Base.DialOptions(opts...)...)
		if err != nil {
			return nil, fmt.Errorf("ssh dial through proxyjump failed: %w", err)
		}

		// ensure keepalive on underlying first connection
		tcpKeepAlive(firstConn)

		ss.client = client
		ss.underConn = firstConn
		jumpList := make([]string, len(jumps)-1)
		for i, j := range jumps[:len(jumps)-1] {
			jumpList[i] = j.Config.User + "@" + j.Addr
		}
		log.Infoln("[SSH] %s@%s connected successfully via proxyjump[%s] (multiplexing enabled)", ss.sshConfig.User, ss.addr, strings.Join(jumpList, " -> "))

		// Start a goroutine to monitor the connection
		go ss.monitorConnection()

		return client, nil
	}

	// Default: direct connect to target
	underConn, err := dialer.DialContext(ctx, "tcp", ss.addr, ss.Base.DialOptions(opts...)...)
	if err != nil {
		return nil, fmt.Errorf("ssh %s tcp connect error: %w", ss.addr, err)
	}
	tcpKeepAlive(underConn)

	clientConn, chans, reqs, err := ssh.NewClientConn(underConn, ss.addr, ss.sshConfig)
	if err != nil {
		_ = underConn.Close()
		return nil, fmt.Errorf("ssh connection %s@%s failed: %w", ss.sshConfig.User, ss.addr, err)
	}

	client := ssh.NewClient(clientConn, chans, reqs)
	ss.client = client
	ss.underConn = underConn

	log.Infoln("[SSH] %s@%s connected successfully (multiplexing enabled)", ss.sshConfig.User, ss.addr)

	// Start a goroutine to monitor the connection
	go ss.monitorConnection()

	return client, nil
}

// monitorConnection monitors the SSH connection and clears it only if still the same client.
func (ss *Ssh) monitorConnection() {
	ss.clientMu.Lock()
	client := ss.client
	ss.clientMu.Unlock()

	if client == nil {
		return
	}

	// Block until this client connection exits
	if err := client.Wait(); err != nil {
		log.Infoln("[SSH] client wait returned error: %v", err)
	}

	ss.clientMu.Lock()
	defer ss.clientMu.Unlock()

	// Only clear if ss.client is still the same client we waited on
	if ss.client == client {
		log.Infoln("[SSH] %s@%s connection closed, will reconnect on next request", ss.sshConfig.User, ss.addr)
		ss.client = nil
		if ss.underConn != nil {
			_ = ss.underConn.Close()
			ss.underConn = nil
		}
	}
}

// DialContext implements C.ProxyAdapter
func (ss *Ssh) DialContext(ctx context.Context, metadata *C.Metadata, opts ...dialer.Option) (_ C.Conn, err error) {
	// Get or create multiplexed SSH client
	client, err := ss.getOrCreateClient(ctx, opts...)
	if err != nil {
		return nil, err
	}

	// Dial to the target through SSH tunnel
	remoteAddr := net.JoinHostPort(metadata.String(), metadata.DstPort.String())
	remoteConn, err := client.Dial("tcp", remoteAddr)
	if err != nil {
		return nil, fmt.Errorf("ssh tunnel dial to %s failed: %w", remoteAddr, err)
	}

	return NewConn(&sshTunnelConn{Conn: remoteConn}, ss), nil
}

// sshTunnelConn wraps a connection created through SSH tunnel
type sshTunnelConn struct {
	net.Conn
}

// StreamConn implements C.ProxyAdapter
func (ss *Ssh) StreamConn(c net.Conn, metadata *C.Metadata) (net.Conn, error) {
	// This method is kept for compatibility with the ProxyAdapter interface,
	// but is not used in normal operation since DialContext uses multiplexing.
	// This creates a temporary non-multiplexed SSH client for this single connection.
	clientConn, chans, reqs, err := ssh.NewClientConn(c, ss.addr, ss.sshConfig)
	if err != nil {
		return nil, fmt.Errorf("ssh connection %s@%s failed: %w", ss.sshConfig.User, ss.addr, err)
	}

	client := ssh.NewClient(clientConn, chans, reqs)

	// Dial to the target through SSH tunnel
	remoteAddr := net.JoinHostPort(metadata.String(), metadata.DstPort.String())
	remoteConn, err := client.Dial("tcp", remoteAddr)
	if err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("ssh tunnel dial failed: %w", err)
	}

	return &sshConn{
		Conn:      remoteConn,
		client:    client,
		underConn: c,
	}, nil
}

// ListenPacketContext implements C.ProxyAdapter
func (ss *Ssh) ListenPacketContext(ctx context.Context, metadata *C.Metadata, opts ...dialer.Option) (C.PacketConn, error) {
	_ = ctx
	_ = metadata
	_ = opts
	return nil, fmt.Errorf("ssh does not support UDP")
}

func NewSsh(option SshOption) (*Ssh, error) {
	// Track if port was explicitly configured (0 means not configured)
	portExplicitlySet := option.Port != 0

	// Set default port if not specified
	if option.Port == 0 {
		option.Port = defaultSSHPort
	}

	// Prepare SSH client configuration
	sshConfig := &ssh.ClientConfig{
		User: option.UserName,
		// Note: Using InsecureIgnoreHostKey bypasses host key verification.
		// This is insecure but commonly used for proxies. In production,
		// consider implementing proper host key verification using known_hosts.
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         C.DefaultTCPTimeout,
	}

	// Handle SSH config file if enabled
	//if option.UseSSHConfig {}
	if err := loadSSHConfig(&option, portExplicitlySet); err != nil {
		return nil, fmt.Errorf("failed to load SSH config: %w", err)
	}

	if sshConfig.User == "" {
		sshConfig.User = option.UserName
	}

	// Setup authentication
	var authMethods []ssh.AuthMethod

	// Try private key authentication first
	if option.PrivateKey != "" {
		for _, keyPath := range strings.Split(option.PrivateKey, ",") {
			if strings.HasPrefix(keyPath, "~/") {
				home, err := os.UserHomeDir()
				if err != nil {
					return nil, fmt.Errorf("failed to get home directory: %w", err)
				}
				keyPath = filepath.Join(home, keyPath[2:])
			}

			key, err := os.ReadFile(keyPath)
			if err != nil {
				return nil, fmt.Errorf("failed to read private key: %w", err)
			}

			signer, err := ssh.ParsePrivateKey(key)
			if err != nil {
				return nil, fmt.Errorf("failed to parse private key: %w", err)
			}

			authMethods = append(authMethods, ssh.PublicKeys(signer))
		}
	}

	// Add password authentication if provided
	if option.Password != "" {
		authMethods = append(authMethods, ssh.Password(option.Password))
	}

	if len(authMethods) == 0 {
		return nil, fmt.Errorf("no authentication method provided")
	}

	sshConfig.Auth = authMethods

	return &Ssh{
		Base: &Base{
			name:  option.Name,
			addr:  net.JoinHostPort(option.Server, strconv.Itoa(option.Port)),
			tp:    C.Ssh,
			udp:   false, // SSH proxy does not support UDP
			iface: option.Interface,
			rmark: option.RoutingMark,
		},
		user:           option.UserName,
		pass:           option.Password,
		privateKeyPath: option.PrivateKey,
		sshConfig:      sshConfig,
		proxyJump:      option.ProxyJump,
	}, nil
}

// loadSSHConfig loads SSH configuration from ~/.ssh/config using ssh_config package
// portExplicitlySet indicates whether the port was explicitly configured in the proxy config
func loadSSHConfig(option *SshOption, portExplicitlySet bool) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	configPath := filepath.Join(home, ".ssh", "config")

	// Open and parse SSH config file
	f, err := os.Open(configPath)
	if err != nil {
		// Config file is optional, return nil if not found
		if os.IsNotExist(err) {
			return nil
		}
		// For other errors, return them as they might indicate permission issues
		return fmt.Errorf("failed to open SSH config: %w", err)
	}
	defer func() { _ = f.Close() }()

	cfg, err := ssh_config.Decode(f)
	if err != nil {
		return fmt.Errorf("failed to parse SSH config: %w", err)
	}

	// Get configuration for the specified host
	host := option.Server

	// Only use values from ~/.ssh/config when the corresponding clash YAML
	// option is not set. Clash config (provided in SshOption) has priority.

	// Get HostName (the actual server to connect to)
	// Only set option.Server from ssh config if it wasn't provided in the clash config.
	//if option.Server == "" {
	//}
	hostname, _ := cfg.Get(host, "HostName")
	if hostname != "" {
		option.Server = hostname
	}

	// Get Port if not explicitly set in proxy config
	if !portExplicitlySet {
		portStr, _ := cfg.Get(host, "Port")
		if portStr != "" {
			if port, err := strconv.Atoi(portStr); err == nil {
				option.Port = port
			}
		}
	}

	// Get User if not already set
	if option.UserName == "" {
		users, _ := cfg.GetAll(host, "User")
		if len(users) > 0 {
			option.UserName = users[0]
		} else {
			log.Warnln("SSH config: No User found for host %s", host)
		}
	}

	// Get IdentityFile if not already set
	if option.PrivateKey == "" {
		identityFile, _ := cfg.Get(host, "IdentityFile")
		if identityFile != "" {
			option.PrivateKey = identityFile
		} else {
			//default to ~/.ssh/id_rsa or ~/.ssh/id_ed25519 ; use the first  exists file
			idRsaPath := filepath.Join(home, ".ssh", "id_rsa")
			idEd25519Path := filepath.Join(home, ".ssh", "id_ed25519")
			if _, err := os.Stat(idRsaPath); err == nil {
				option.PrivateKey = idRsaPath
			}
			if _, err := os.Stat(idEd25519Path); err == nil {
				if option.PrivateKey != "" {
					option.PrivateKey += "," + idEd25519Path
				} else {
					option.PrivateKey = idEd25519Path
				}
			}
		}
	}
	if option.ProxyJump == "" {
		proxyJump, _ := cfg.Get(host, "ProxyJump")
		if proxyJump != "" {
			option.ProxyJump = proxyJump
		}
	}
	if option.Password == "" {
		password, _ := cfg.Get(host, "Password")
		if password != "" {
			option.Password = password
		}
	}
	// Set defaults if not configured
	if option.Port == 0 {
		option.Port = defaultSSHPort
	}

	return nil
}

// loadSSHConfigForHost loads SSH configuration for a specific host pattern from ~/.ssh/config
// It always queries the provided host pattern in ssh_config and returns populated SshOption fields
func loadSSHConfigForHost(host string) (SshOption, error) {
	var opt SshOption

	home, err := os.UserHomeDir()
	if err != nil {
		return opt, err
	}

	configPath := filepath.Join(home, ".ssh", "config")
	f, err := os.Open(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return opt, nil
		}
		return opt, fmt.Errorf("failed to open SSH config: %w", err)
	}
	defer func() { _ = f.Close() }()

	cfg, err := ssh_config.Decode(f)
	if err != nil {
		return opt, fmt.Errorf("failed to parse SSH config: %w", err)
	}

	opt.Name = host

	// HostName
	if hn, _ := cfg.Get(host, "HostName"); hn != "" {
		opt.Server = hn
	}
	if opt.Server == "" {
		opt.Server = host
	}

	// Port
	if p, _ := cfg.Get(host, "Port"); p != "" {
		if port, err := strconv.Atoi(p); err == nil {
			opt.Port = port
		}
	}
	// User
	if users, _ := cfg.GetAll(host, "User"); len(users) > 0 {
		opt.UserName = users[0]
	}
	// IdentityFile
	if idf, _ := cfg.Get(host, "IdentityFile"); idf != "" {
		opt.PrivateKey = idf
	}
	// ProxyJump
	if pj, _ := cfg.Get(host, "ProxyJump"); pj != "" {
		opt.ProxyJump = pj
	}
	// Password (less common in ssh_config but supported earlier)
	if pw, _ := cfg.Get(host, "Password"); pw != "" {
		opt.Password = pw
	}
	// Get IdentityFile if not already set
	if opt.PrivateKey == "" {
		//default to ~/.ssh/id_rsa or ~/.ssh/id_ed25519 ; use the first  exists file
		idRsaPath := filepath.Join(home, ".ssh", "id_rsa")
		idEd25519Path := filepath.Join(home, ".ssh", "id_ed25519")
		if _, err := os.Stat(idRsaPath); err == nil {
			opt.PrivateKey = idRsaPath
		}
		if _, err := os.Stat(idEd25519Path); err == nil {
			if opt.PrivateKey == "" {
				opt.PrivateKey = idEd25519Path
			} else {
				opt.PrivateKey += "," + idEd25519Path
			}

		}

	}

	if opt.Port == 0 {
		opt.Port = defaultSSHPort
	}

	return opt, nil
}

// JumpHost 表示一跳的目标（host:port）和该跳的 ssh.Config
type JumpHost struct {
	Addr   string
	Config *ssh.ClientConfig
}

// dialThroughJumps 建立通过若干跳的 SSH 链路。
// dialerFunc 用于建立到首跳的 TCP 连接（可以传入 dialer.DialContext）。
// 返回最终的 *ssh.Client 和首跳的底层 net.Conn（用于关闭整个链路）。
func dialThroughJumps(
	ctx context.Context,
	dialerFunc func(context.Context, string, string, ...dialer.Option) (net.Conn, error),
	jumps []JumpHost,
	opts ...dialer.Option,
) (*ssh.Client, net.Conn, error) {
	if len(jumps) == 0 {
		return nil, nil, fmt.Errorf("no jumps provided")
	}

	// 首跳：直接 TCP 连接
	firstConn, err := dialerFunc(ctx, "tcp", jumps[0].Addr, opts...)
	if err != nil {
		return nil, nil, fmt.Errorf("jump: tcp dial to %s failed: %w", jumps[0].Addr, err)
	}

	// set keepalive on first connection
	tcpKeepAlive(firstConn)

	// 将首跳的 TCP conn 升级为 SSH client（client1）
	clientConn, chans, reqs, err := ssh.NewClientConn(firstConn, jumps[0].Addr, jumps[0].Config)
	if err != nil {
		_ = firstConn.Close()
		return nil, nil, fmt.Errorf("jump: ssh handshake to %s@%s failed: %w", jumps[0].Config.User, jumps[0].Addr, err)
	} else {
		log.Infoln("jump: ssh handshake to %s@%s success", jumps[0].Config.User, jumps[0].Addr)
	}
	client := ssh.NewClient(clientConn, chans, reqs)

	// 对后续每一跳，用上一级 client.Dial 得到的 net.Conn 再做 NewClientConn
	for i := 1; i < len(jumps); i++ {
		nextAddr := jumps[i].Addr
		nextTCP, err := client.Dial("tcp", nextAddr)
		if err != nil {
			_ = client.Close()
			_ = firstConn.Close()
			return nil, nil, fmt.Errorf("jump: dial from jump %d to %s failed: %w", i-1, nextAddr, err)
		}

		cconn, cchans, creqs, err := ssh.NewClientConn(nextTCP, nextAddr, jumps[i].Config)
		if err != nil {
			_ = nextTCP.Close()
			_ = client.Close()
			_ = firstConn.Close()
			return nil, nil, fmt.Errorf("jump: ssh handshake to %s@%s failed: %w", jumps[i].Config.User, nextAddr, err)
		}
		// 用新的 client 替换上一级 client（注意：上一级 client 需要 Close 得到合适的资源回收，
		// 这里可选择在失败场景才 Close，上层根据需要处理旧 client 的 Close）
		client = ssh.NewClient(cconn, cchans, creqs)
	}

	// 返回最终 client 和首跳的底层 TCP 连接（用于后续关闭）
	return client, firstConn, nil
}
