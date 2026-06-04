// Package usecase — бизнес-логика оркестратора и порты (интерфейсы адаптеров).
// Зависит только от domain. Реализации портов (Timeweb, SSH, SQLite) внедряются
// снаружи — инверсия зависимостей, ядро не знает про конкретное облако/транспорт.
package usecase

import (
	"context"
	"time"

	"github.com/chudno/zerovibe/orchestrator/internal/domain"
)

// Provider — порт облачного провайдера VM (реализация: Timeweb).
// Абстрагирует ядро от конкретного облака: сменить провайдера = новая реализация.
type Provider interface {
	// CreateNode создаёт VM (с Docker через cloud-init) и возвращает её,
	// когда она получила IP. Статус может быть provisioning — готовность к
	// приёму приложений подтверждает Deployer.WaitReady.
	CreateNode(ctx context.Context, name string) (domain.Node, error)
}

// Deployer — порт «доставить и запустить приложение на конкретной VM»
// (реализация: SSH). По сути — логика deploy.sh, вызываемая из Go.
type Deployer interface {
	// WaitReady ждёт готовности Docker на узле (после cloud-init).
	WaitReady(ctx context.Context, node domain.Node) error
	// Deploy разворачивает приложение на узле под доменом и поднимает маршрут.
	Deploy(ctx context.Context, node domain.Node, app domain.App, domainName string) error
}

// Store — порт хранилища метаданных оркестратора (реализация: SQLite).
type Store interface {
	CreateApp(ctx context.Context, a domain.App) (domain.App, error)
	GetApp(ctx context.Context, id string) (domain.App, error)
	ListApps(ctx context.Context) ([]domain.App, error)

	CreateNode(ctx context.Context, n domain.Node) (domain.Node, error)
	ListNodes(ctx context.Context) ([]domain.Node, error)
	SetNodeStatus(ctx context.Context, id string, status domain.NodeStatus) error
	// CountDeploymentsOnNode — сколько приложений уже размещено на узле (для ёмкости).
	CountDeploymentsOnNode(ctx context.Context, nodeID string) (int, error)

	CreateDeployment(ctx context.Context, d domain.Deployment) (domain.Deployment, error)
	SetDeploymentStatus(ctx context.Context, id string, status domain.DeployStatus) error
	ListDeployments(ctx context.Context) ([]domain.Deployment, error)
}

// IDGen — генератор идентификаторов (реализация: uuid). Вынесен в порт, чтобы
// usecase оставался детерминированным в тестах.
type IDGen interface {
	NewID() string
}

// Clock — источник времени (порт для тестируемости).
type Clock interface {
	Now() time.Time
}
