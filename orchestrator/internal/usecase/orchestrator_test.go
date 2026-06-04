// Тест ядра оркестратора на in-memory fake-store и заглушках провайдера/деплойера.
// Проверяет ключевую логику размещения: создание/переиспользование узлов по
// ёмкости и корректные статусы развёртываний — без БД и облака.
package usecase

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/chudno/zerovibe/orchestrator/internal/deployer/ssh"
	"github.com/chudno/zerovibe/orchestrator/internal/domain"
	"github.com/chudno/zerovibe/orchestrator/internal/provider/timeweb"
)

// memStore — потокобезопасное in-memory хранилище для тестов.
type memStore struct {
	mu    sync.Mutex
	apps  []domain.App
	nodes []domain.Node
	deps  []domain.Deployment
}

func (m *memStore) CreateApp(_ context.Context, a domain.App) (domain.App, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.apps = append(m.apps, a)
	return a, nil
}
func (m *memStore) GetApp(_ context.Context, id string) (domain.App, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, a := range m.apps {
		if a.ID == id {
			return a, nil
		}
	}
	return domain.App{}, domain.ErrNotFound{Entity: "app", ID: id}
}
func (m *memStore) ListApps(context.Context) ([]domain.App, error) { return m.apps, nil }

func (m *memStore) CreateNode(_ context.Context, n domain.Node) (domain.Node, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.nodes = append(m.nodes, n)
	return n, nil
}
func (m *memStore) ListNodes(context.Context) ([]domain.Node, error) { return m.nodes, nil }
func (m *memStore) SetNodeStatus(_ context.Context, id string, st domain.NodeStatus) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.nodes {
		if m.nodes[i].ID == id {
			m.nodes[i].Status = st
		}
	}
	return nil
}
func (m *memStore) CountDeploymentsOnNode(_ context.Context, nodeID string) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	c := 0
	for _, d := range m.deps {
		if d.NodeID == nodeID && d.Status != domain.DeployFailed {
			c++
		}
	}
	return c, nil
}
func (m *memStore) CreateDeployment(_ context.Context, d domain.Deployment) (domain.Deployment, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.deps = append(m.deps, d)
	return d, nil
}
func (m *memStore) SetDeploymentStatus(_ context.Context, id string, st domain.DeployStatus) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.deps {
		if m.deps[i].ID == id {
			m.deps[i].Status = st
		}
	}
	return nil
}
func (m *memStore) ListDeployments(context.Context) ([]domain.Deployment, error) { return m.deps, nil }

// seqIDGen — детерминированные id для предсказуемости теста.
type seqIDGen struct{ n int }

func (g *seqIDGen) NewID() string {
	g.n++
	return "id-" + strconv.Itoa(g.n) + "-padding-to-8plus"
}

type fixedClock struct{}

func (fixedClock) Now() time.Time { return time.Unix(1_700_000_000, 0).UTC() }

func newOrch(capacity int) (*Orchestrator, *memStore) {
	store := &memStore{}
	o := New(
		timeweb.NewFake("10.0.0.1"),
		ssh.NewNoOp(),
		store,
		&seqIDGen{},
		fixedClock{},
		Config{BaseDomain: "zerovibe.ru", NodeCapacity: capacity},
	)
	return o, store
}

func TestDeployApp_FirstCreatesNode(t *testing.T) {
	o, store := newOrch(10)
	dep, err := o.DeployApp(context.Background(), "owner1", "My App", "myapp")
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if dep.Status != domain.DeployRunning {
		t.Errorf("ожидался running, получено %s", dep.Status)
	}
	if dep.Domain != "myapp.zerovibe.ru" {
		t.Errorf("домен неверный: %s", dep.Domain)
	}
	if len(store.nodes) != 1 {
		t.Errorf("ожидался 1 созданный узел, получено %d", len(store.nodes))
	}
}

func TestDeployApp_ReusesNodeUntilFull(t *testing.T) {
	o, store := newOrch(2) // ёмкость 2 приложения на узел
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		if _, err := o.DeployApp(ctx, "owner1", "App", "app"+strconv.Itoa(i)); err != nil {
			t.Fatalf("деплой %d: %v", i, err)
		}
	}
	// 3 приложения при ёмкости 2 → должно быть 2 узла (2+1).
	if len(store.nodes) != 2 {
		t.Errorf("ожидалось 2 узла для 3 приложений (cap=2), получено %d", len(store.nodes))
	}
}

func TestDeployApp_InvalidSubdomain(t *testing.T) {
	o, _ := newOrch(10)
	_, err := o.DeployApp(context.Background(), "owner1", "App", "Bad_Domain!")
	var ve domain.ErrValidation
	if !errors.As(err, &ve) {
		t.Fatalf("ожидалась ErrValidation, получено %v", err)
	}
}
