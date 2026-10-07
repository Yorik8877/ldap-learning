package app

import (
	"crypto/rand"
	"encoding/base64"
	"time"
)

const sessionIDBytes = 32

type systemClock struct{}

func (systemClock) Now() time.Time {
	return time.Now().UTC()
}

type randomIDGenerator struct{}

func (randomIDGenerator) NewID() (string, error) {
	buffer := make([]byte, sessionIDBytes)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}
