// Package timeweb — провайдер VM. Реальная реализация (Terraform-exec или REST
// к Timeweb API) подключается позже за портом usecase.Provider.
//
// Сейчас — Fake: возвращает «готовую» VM с фиксированным IP. Этого достаточно,
// чтобы доказать логику оркестратора end-to-end без обращения к облаку.
package timeweb

import (
	"context"

	"github.com/chudno/zerovibe/orchestrator/internal/domain"
)

// Fake — провайдер-заглушка. Каждый CreateNode возвращает узел с заданным IP.
type Fake struct {
	IP string
}

// NewFake создаёт заглушку с фиксированным IP (по умолчанию loopback).
func NewFake(ip string) *Fake {
	if ip == "" {
		ip = "127.0.0.1"
	}
	return &Fake{IP: ip}
}

// CreateNode возвращает сразу ready-узел (заглушка не ждёт provisioning).
func (f *Fake) CreateNode(_ context.Context, name string) (domain.Node, error) {
	return domain.Node{
		ProviderID: "fake-" + name,
		IPv4:       f.IP,
		Status:     domain.NodeReady,
	}, nil
}
