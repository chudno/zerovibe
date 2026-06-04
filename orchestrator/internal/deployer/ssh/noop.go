// Package ssh — деплой приложения на VM по SSH (логика deploy.sh из Go).
// Реальная реализация добавляется при подключении живых VM; пока — NoOp,
// чтобы оркестратор работал end-to-end локально и в тестах.
package ssh

import (
	"context"

	"github.com/chudno/zerovibe/orchestrator/internal/domain"
)

// NoOp — деплойер-заглушка: считает любой узел готовым и «деплоит» без действий.
type NoOp struct{}

// NewNoOp создаёт заглушку.
func NewNoOp() *NoOp { return &NoOp{} }

// WaitReady мгновенно успешен.
func (NoOp) WaitReady(_ context.Context, _ domain.Node) error { return nil }

// Deploy ничего не делает (успех).
func (NoOp) Deploy(_ context.Context, _ domain.Node, _ domain.App, _ string) error { return nil }
