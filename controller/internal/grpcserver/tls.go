package grpcserver

import (
	"crypto/tls"
	"fmt"

	"github.com/igris/ztna/controller/internal/config"
	"google.golang.org/grpc/credentials"
)

func serverTransportCredentials(cfg config.Config) (credentials.TransportCredentials, error) {
	cert, err := tls.LoadX509KeyPair(cfg.TLSCertFile, cfg.TLSKeyFile)
	if err != nil {
		return nil, fmt.Errorf("load server TLS cert/key: %w", err)
	}

	tlsCfg := &tls.Config{
		MinVersion:   tls.VersionTLS13,
		Certificates: []tls.Certificate{cert},
		// Bootstrap methods still need to work before enrollment; client cert is
		// validated at the RPC layer for auth-required methods like Renew.
		ClientAuth: tls.RequestClientCert,
	}

	return credentials.NewTLS(tlsCfg), nil
}
