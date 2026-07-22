package client

import (
	"errors"
	"fmt"
	"net"
	"os"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// NewForwardedHTTPClient connects to an agent exposed via SSH local forward.
// baseURL is e.g. "http://127.0.0.1:8787" after `ssh -L 8787:127.0.0.1:8787 user@zfs-host`.
func NewForwardedHTTPClient(baseURL string) (*HTTPClient, error) {
	return NewTCPHTTPClient(baseURL)
}

// SSHConfig holds SSH client parameters.
type SSHConfig struct {
	Host       string
	User       string
	Password   string
	PrivateKey []byte
	// KnownHostsPath is a known_hosts file for host key verification (required).
	// Defaults to ~/.ssh/known_hosts when empty.
	KnownHostsPath string
}

// DialSSH opens port 22 on Host. Host keys are verified via known_hosts (no insecure callback).
func DialSSH(cfg SSHConfig) (*ssh.Client, error) {
	var auth []ssh.AuthMethod
	if len(cfg.PrivateKey) > 0 {
		k, err := ssh.ParsePrivateKey(cfg.PrivateKey)
		if err != nil {
			return nil, err
		}
		auth = append(auth, ssh.PublicKeys(k))
	}
	if cfg.Password != "" {
		auth = append(auth, ssh.Password(cfg.Password))
	}
	if len(auth) == 0 {
		return nil, errors.New("ssh: no auth method (password or private key required)")
	}
	khPath := cfg.KnownHostsPath
	if khPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("ssh: known_hosts path required: %w", err)
		}
		khPath = home + "/.ssh/known_hosts"
	}
	hostKeyCallback, err := knownhosts.New(khPath)
	if err != nil {
		return nil, fmt.Errorf("ssh: load known_hosts %s: %w", khPath, err)
	}
	cc := ssh.ClientConfig{
		User:            cfg.User,
		Auth:            auth,
		HostKeyCallback: hostKeyCallback,
	}
	return ssh.Dial("tcp", net.JoinHostPort(cfg.Host, "22"), &cc)
}
