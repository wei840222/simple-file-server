package main

import (
	"crypto/rand"
	"crypto/sha256"
	"fmt"
)

func generateToken() (string, error) {
	randBytes := make([]byte, 32)
	if _, err := rand.Read(randBytes); err != nil {
		return "", err
	}
	b := sha256.Sum256(randBytes)
	return fmt.Sprintf("%x", b), nil
}
