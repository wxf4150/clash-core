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

	"github.com/Dreamacro/clash/component/dialer"
	C "github.com/Dreamacro/clash/constant"
	"golang.org/x/crypto/ssh"
)

type Ssh struct {
	*Base
	user           string
	pass           string
	privateKeyPath string
	sshConfig      *ssh.ClientConfig
}

type SshOption struct {
	BasicOption
	Name           string `proxy:"name"`
	Server         string `proxy:"server"`
	Port           int    `proxy:"port"`
	UserName       string `proxy:"username,omitempty"`
	Password       string `proxy:"password,omitempty"`
	PrivateKey     string `proxy:"privatekey,omitempty"`
	UseSSHConfig   bool   `proxy:"use-ssh-config,omitempty"`
}

type sshConn struct {
	net.Conn
	client    *ssh.Client
	underConn net.Conn
}

func (c *sshConn) Close() error {
	c.Conn.Close()
	c.client.Close()
	return c.underConn.Close()
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

// StreamConn implements C.ProxyAdapter
func (ss *Ssh) StreamConn(c net.Conn, metadata *C.Metadata) (net.Conn, error) {
	// Create SSH client from the connection
	clientConn, chans, reqs, err := ssh.NewClientConn(c, ss.addr, ss.sshConfig)
	if err != nil {
		return nil, fmt.Errorf("ssh connection failed: %w", err)
	}

	client := ssh.NewClient(clientConn, chans, reqs)
	defer func() {
		if err != nil {
			client.Close()
		}
	}()

	// Dial to the target through SSH tunnel
	remoteAddr := net.JoinHostPort(metadata.String(), metadata.DstPort.String())
	remoteConn, err := client.Dial("tcp", remoteAddr)
	if err != nil {
		return nil, fmt.Errorf("ssh tunnel dial failed: %w", err)
	}

	return &sshConn{
		Conn:      remoteConn,
		client:    client,
		underConn: c,
	}, nil
}

// DialContext implements C.ProxyAdapter
func (ss *Ssh) DialContext(ctx context.Context, metadata *C.Metadata, opts ...dialer.Option) (_ C.Conn, err error) {
	c, err := dialer.DialContext(ctx, "tcp", ss.addr, ss.Base.DialOptions(opts...)...)
	if err != nil {
		return nil, fmt.Errorf("%s connect error: %w", ss.addr, err)
	}
	tcpKeepAlive(c)

	defer func(c net.Conn) {
		safeConnClose(c, err)
	}(c)

	c, err = ss.StreamConn(c, metadata)
	if err != nil {
		return nil, err
	}

	return NewConn(c, ss), nil
}

// ListenPacketContext implements C.ProxyAdapter
func (ss *Ssh) ListenPacketContext(ctx context.Context, metadata *C.Metadata, opts ...dialer.Option) (C.PacketConn, error) {
	return nil, fmt.Errorf("ssh does not support UDP")
}

func NewSsh(option SshOption) (*Ssh, error) {
	// Prepare SSH client configuration
	sshConfig := &ssh.ClientConfig{
		User:            option.UserName,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // TODO: Add host key verification
		Timeout:         C.DefaultTCPTimeout,
	}

	// Handle SSH config file if enabled
	if option.UseSSHConfig {
		if err := loadSSHConfig(&option); err != nil {
			return nil, fmt.Errorf("failed to load SSH config: %w", err)
		}
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

// loadSSHConfig loads SSH configuration from ~/.ssh/config
func loadSSHConfig(option *SshOption) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	configPath := filepath.Join(home, ".ssh", "config")
	data, err := os.ReadFile(configPath)
	if err != nil {
		// Config file is optional
		return nil
	}

	// Simple SSH config parser
	lines := strings.Split(string(data), "\n")
	var currentHost string
	inMatchingHost := false

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}

		key := strings.ToLower(parts[0])
		value := parts[1]

		if key == "host" {
			currentHost = value
			// Check if this host matches our server
			if currentHost == option.Server || currentHost == "*" {
				inMatchingHost = true
			} else {
				inMatchingHost = false
			}
			continue
		}

		if !inMatchingHost {
			continue
		}

		// Apply configuration for matching host
		switch key {
		case "hostname":
			if option.Server == currentHost {
				option.Server = value
			}
		case "port":
			if port, err := strconv.Atoi(value); err == nil && option.Port == 0 {
				option.Port = port
			}
		case "user":
			if option.UserName == "" {
				option.UserName = value
			}
		case "identityfile":
			if option.PrivateKey == "" {
				option.PrivateKey = value
			}
		}
	}

	// Set defaults if not configured
	if option.Port == 0 {
		option.Port = 22
	}

	return nil
}
