package usecase

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

// UUIDGen — простой генератор случайных id (16 байт hex). Без внешних зависимостей.
type UUIDGen struct{}

func (UUIDGen) NewID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// SystemClock — реальные часы.
type SystemClock struct{}

func (SystemClock) Now() time.Time { return time.Now() }
