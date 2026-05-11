package application

import (
	"crypto/rand"
	"encoding/hex"
)

type TokenGenerator interface {
	Generate() (string, error)
}

type CryptoTokenGenerator struct {
	Size int
}

const defaultTokenBytes = 32

func NewCryptoTokenGenerator(size int) CryptoTokenGenerator {
	if size <= 0 {
		size = defaultTokenBytes
	}
	return CryptoTokenGenerator{Size: size}
}

func (g CryptoTokenGenerator) Generate() (string, error) {
	b := make([]byte, g.Size)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
