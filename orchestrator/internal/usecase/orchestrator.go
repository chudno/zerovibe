package usecase

import (
	"context"
	"fmt"

	"github.com/chudno/zerovibe/orchestrator/internal/domain"
)

// Orchestrator — ядро: разворачивает приложения вайбкодеров на наших VM.
type Orchestrator struct {
	provider Provider
	deployer Deployer
	store    Store
	ids      IDGen
	clock    Clock

	baseDomain   string // напр. "zerovibe.ru" → app.zerovibe.ru
	nodeCapacity int    // сколько приложений на одну VM (простая модель)
}

// Config — параметры оркестратора.
type Config struct {
	BaseDomain   string
	NodeCapacity int
}

// New собирает оркестратор с внедрёнными адаптерами.
func New(p Provider, d Deployer, s Store, ids IDGen, clock Clock, cfg Config) *Orchestrator {
	if cfg.NodeCapacity <= 0 {
		cfg.NodeCapacity = 10
	}
	return &Orchestrator{
		provider: p, deployer: d, store: s, ids: ids, clock: clock,
		baseDomain: cfg.BaseDomain, nodeCapacity: cfg.NodeCapacity,
	}
}

// DeployApp — главная операция. Регистрирует приложение, подбирает/создаёт узел,
// разворачивает контейнер и фиксирует развёртывание.
//
// Порядок и идемпотентность важны: при ошибке деплоя помечаем Deployment failed,
// но App и Node остаются (повторный деплой переиспользует их).
func (o *Orchestrator) DeployApp(ctx context.Context, ownerID, name, subdomain string) (domain.Deployment, error) {
	// 1. Валидируем и сохраняем приложение.
	app, err := domain.NewApp(ownerID, name, subdomain)
	if err != nil {
		return domain.Deployment{}, err
	}
	app.ID = o.ids.NewID()
	app.CreatedAt = o.clock.Now()
	app, err = o.store.CreateApp(ctx, app)
	if err != nil {
		return domain.Deployment{}, fmt.Errorf("сохранить app: %w", err)
	}

	// 2. Подбираем узел с местом или создаём новый.
	node, err := o.pickOrCreateNode(ctx)
	if err != nil {
		return domain.Deployment{}, err
	}

	// 3. Готовим запись о развёртывании (pending).
	domainName := app.Subdomain + "." + o.baseDomain
	dep := domain.Deployment{
		ID:        o.ids.NewID(),
		AppID:     app.ID,
		NodeID:    node.ID,
		Domain:    domainName,
		Status:    domain.DeployPending,
		CreatedAt: o.clock.Now(),
	}
	dep, err = o.store.CreateDeployment(ctx, dep)
	if err != nil {
		return domain.Deployment{}, fmt.Errorf("создать deployment: %w", err)
	}

	// 4. Дожидаемся готовности узла и разворачиваем приложение.
	if err := o.deployer.WaitReady(ctx, node); err != nil {
		_ = o.store.SetDeploymentStatus(ctx, dep.ID, domain.DeployFailed)
		return dep, fmt.Errorf("узел не готов: %w", err)
	}
	if err := o.deployer.Deploy(ctx, node, app, domainName); err != nil {
		_ = o.store.SetDeploymentStatus(ctx, dep.ID, domain.DeployFailed)
		return dep, fmt.Errorf("деплой на узел: %w", err)
	}

	// 5. Успех.
	if err := o.store.SetDeploymentStatus(ctx, dep.ID, domain.DeployRunning); err != nil {
		return dep, fmt.Errorf("обновить статус: %w", err)
	}
	dep.Status = domain.DeployRunning
	return dep, nil
}

// pickOrCreateNode возвращает ready-узел со свободным местом; если такого нет —
// создаёт новый через провайдера и регистрирует его.
func (o *Orchestrator) pickOrCreateNode(ctx context.Context) (domain.Node, error) {
	nodes, err := o.store.ListNodes(ctx)
	if err != nil {
		return domain.Node{}, fmt.Errorf("список узлов: %w", err)
	}
	for _, n := range nodes {
		used, err := o.store.CountDeploymentsOnNode(ctx, n.ID)
		if err != nil {
			return domain.Node{}, err
		}
		if n.HasRoom(used) {
			return n, nil
		}
	}

	// Места нет — создаём новый узел.
	name := "zerovibe-node-" + o.ids.NewID()[:8]
	node, err := o.provider.CreateNode(ctx, name)
	if err != nil {
		return domain.Node{}, fmt.Errorf("%w: %v", domain.ErrNoCapacity, err)
	}
	node.ID = o.ids.NewID()
	node.Capacity = o.nodeCapacity
	if node.Status == "" {
		node.Status = domain.NodeProvisioning
	}
	if node.CreatedAt.IsZero() {
		node.CreatedAt = o.clock.Now()
	}
	node, err = o.store.CreateNode(ctx, node)
	if err != nil {
		return domain.Node{}, fmt.Errorf("сохранить узел: %w", err)
	}
	return node, nil
}

// ListApps / ListNodes / ListDeployments — чтение для панели/CLI.
func (o *Orchestrator) ListApps(ctx context.Context) ([]domain.App, error) {
	return o.store.ListApps(ctx)
}
func (o *Orchestrator) ListNodes(ctx context.Context) ([]domain.Node, error) {
	return o.store.ListNodes(ctx)
}
func (o *Orchestrator) ListDeployments(ctx context.Context) ([]domain.Deployment, error) {
	return o.store.ListDeployments(ctx)
}
