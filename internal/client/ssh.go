package client

import (
	"net"

	"golang.org/x/crypto/ssh"
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
}

// DialSSH opens port 22 on Host (HostKeyCallback insecure; pin keys for production).
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
	cc := ssh.ClientConfig{
		User:            cfg.User,
		Auth:            auth,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), //nolint:gosec
	}
	return ssh.Dial("tcp", net.JoinHostPort(cfg.Host, "22"), &cc)
}
