package idgen

import "crypto/rand"

type IDGenerator struct {
}

func New() *IDGenerator {
	return &IDGenerator{}
}

func (gen *IDGenerator) NewID() (string, error) {
	return rand.Text(), nil
}
