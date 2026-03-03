package pki

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
)

func NewP256Keypair() (*ecdsa.PrivateKey, error) {
	return ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
}
