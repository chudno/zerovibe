// Package domain — сущности оркестратора (только stdlib).
// Оркестратор разворачивает приложения вайбкодеров в контейнерах на наших VM.
//
// Модель: App (приложение клиента) разворачивается как Deployment на Node (VM).
// Одна Node несёт много App; когда место кончается — поднимается новая Node.
package domain

import (
	"fmt"
	"strings"
	"time"
)

// NodeStatus — состояние VM.
type NodeStatus string

const (
	NodeProvisioning NodeStatus = "provisioning" // создаётся/ставится Docker
	NodeReady        NodeStatus = "ready"        // готова принимать приложения
	NodeFailed       NodeStatus = "failed"
)

// Node — арендованная VM, на которой крутятся контейнеры приложений.
type Node struct {
	ID         string // наш id
	ProviderID string // id у облачного провайдера (Timeweb)
	IPv4       string
	Status     NodeStatus
	Capacity   int // сколько приложений вмещает (простая модель ёмкости)
	CreatedAt  time.Time
}

// HasRoom сообщает, влезет ли ещё одно приложение при текущей загрузке used.
func (n Node) HasRoom(used int) bool {
	return n.Status == NodeReady && used < n.Capacity
}

// DeployStatus — состояние развёртывания.
type DeployStatus string

const (
	DeployPending DeployStatus = "pending"
	DeployRunning DeployStatus = "running" // контейнер поднят, маршрут есть
	DeployFailed  DeployStatus = "failed"
)

// App — приложение вайбкодера.
type App struct {
	ID        string
	OwnerID   string // вайбкодер-владелец (уровень 2)
	Name      string
	Subdomain string // напр. "myapp" → myapp.zerovibe.ru
	Image     string // ссылка на образ в реестре (Фаза 3); пусто = собрать из кода
	CreatedAt time.Time
}

// Deployment — факт размещения App на Node.
type Deployment struct {
	ID        string
	AppID     string
	NodeID    string
	Domain    string // полный домен (subdomain + базовый)
	Status    DeployStatus
	CreatedAt time.Time
}

// NewApp валидирует и конструирует приложение. Поддомен нормализуется и
// проверяется (только [a-z0-9-], 1..63 символов) — он станет частью DNS-имени.
func NewApp(ownerID, name, subdomain string) (App, error) {
	name = strings.TrimSpace(name)
	subdomain = strings.ToLower(strings.TrimSpace(subdomain))

	if ownerID == "" {
		return App{}, ErrValidation{Field: "owner_id", Msg: "владелец обязателен"}
	}
	if name == "" {
		return App{}, ErrValidation{Field: "name", Msg: "имя обязательно"}
	}
	if !validSubdomain(subdomain) {
		return App{}, ErrValidation{Field: "subdomain", Msg: "только a-z, 0-9, дефис; 1..63 символа"}
	}
	return App{OwnerID: ownerID, Name: name, Subdomain: subdomain}, nil
}

func validSubdomain(s string) bool {
	if len(s) == 0 || len(s) > 63 {
		return false
	}
	if s[0] == '-' || s[len(s)-1] == '-' {
		return false
	}
	for _, r := range s {
		if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-') {
			return false
		}
	}
	return true
}

// ErrValidation — нарушен инвариант. → HTTP 400.
type ErrValidation struct {
	Field string
	Msg   string
}

func (e ErrValidation) Error() string {
	if e.Field == "" {
		return e.Msg
	}
	return fmt.Sprintf("%s: %s", e.Field, e.Msg)
}

// ErrNotFound — сущность не найдена. → HTTP 404.
type ErrNotFound struct {
	Entity string
	ID     string
}

func (e ErrNotFound) Error() string {
	return fmt.Sprintf("%s %q не найден", e.Entity, e.ID)
}

// ErrNoCapacity — нет ready-узлов с местом и не удалось создать новый.
var ErrNoCapacity = fmt.Errorf("нет доступной ёмкости для размещения приложения")
