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

	// Connection multiplexing
	clientMu  sync.Mutex
	client    *ssh.Client
	underConn net.Conn
}

type SshOption struct {
	BasicOption
	Name         string `proxy:"name"`
	Server       string `proxy:"server"`
	Port         int    `proxy:"port,omitempty"`
	UserName     string `proxy:"username,omitempty"`
	Password     string `proxy:"password,omitempty"`
	PrivateKey   string `proxy:"privatekey,omitempty"`
	UseSSHConfig bool   `proxy:"use-ssh-config,omitempty"`
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
		c.Close()
	}
	return n, err
}

func (c *sshConn) Write(b []byte) (int, error) {
	n, err := c.Conn.Write(b)
	if err != nil && err != io.EOF {
		c.Close()
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

		// Simple check: try to open a session
		//sess, err := ss.client.NewSession()
		//if err == nil {
		//	sess.Close()
		//	return ss.client, nil
		//}

		// 发送 keepalive 全局请求，错误表示连接不可用
		//_, err := ss.client.SendRequest("keepalive@openssh.com", true, nil)
		//if err == nil {
		//	return ss.client, nil
		//}

		// Connection is dead, close and recreate
		//log.Infoln("[SSH] %s connection lost, reconnecting...", ss.addr)
		//ss.client.Close()
		//if ss.underConn != nil {
		//	ss.underConn.Close()
		//}
		//ss.client = nil
		//ss.underConn = nil
	}
	log.Infoln("[SSH] %s@%s connecting...", ss.sshConfig.User, ss.addr)

	// Create new SSH connection
	underConn, err := dialer.DialContext(ctx, "tcp", ss.addr, ss.Base.DialOptions(opts...)...)
	if err != nil {
		return nil, fmt.Errorf("ssh %s tcp connect error: %w", ss.addr, err)
	}
	tcpKeepAlive(underConn)

	clientConn, chans, reqs, err := ssh.NewClientConn(underConn, ss.addr, ss.sshConfig)
	if err != nil {
		underConn.Close()
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
	client.Wait()

	ss.clientMu.Lock()
	defer ss.clientMu.Unlock()

	// Only clear if ss.client is still the same client we waited on
	if ss.client == client {
		log.Infoln("[SSH] %s@%s connection closed, will reconnect on next request", ss.sshConfig.User, ss.addr)
		ss.client = nil
		if ss.underConn != nil {
			ss.underConn.Close()
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
		client.Close()
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
	if option.UseSSHConfig {
		if err := loadSSHConfig(&option, portExplicitlySet); err != nil {
			return nil, fmt.Errorf("failed to load SSH config: %w", err)
		}
	}
	if sshConfig.User == "" {
		sshConfig.User = option.UserName
	}

	// Setup authentication
	var authMethods []ssh.AuthMethod

	// Try private key authentication first
	if option.PrivateKey != "" {
		keyPath := option.PrivateKey
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
	defer f.Close()

	cfg, err := ssh_config.Decode(f)
	if err != nil {
		return fmt.Errorf("failed to parse SSH config: %w", err)
	}

	// Get configuration for the specified host
	host := option.Server

	// Get HostName (the actual server to connect to)
	// If the key doesn't exist in config, Get returns an error, which we ignore
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
			} else if _, err := os.Stat(idEd25519Path); err == nil {
				option.PrivateKey = idEd25519Path
			}
		}
	}

	// Set defaults if not configured
	if option.Port == 0 {
		option.Port = defaultSSHPort
	}

	return nil
}
