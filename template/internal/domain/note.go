// Package domain содержит сущности и бизнес-инварианты приложения.
// Слой не зависит ни от чего, кроме стандартной библиотеки: ни БД, ни HTTP,
// ни сторонних пакетов сюда не протекают. Это ядро чистой архитектуры.
//
// ОБРАЗЕЦ ДЛЯ ГЕНЕРАЦИИ: новая сущность добавляется по аналогии с Note —
// поля, конструктор-валидатор (NewNote), бизнес-правила в методах. Никаких
// json/db-тегов в domain: представления для транспорта и хранилища — отдельные
// структуры в своих слоях, конвертация на стыках.
package domain

import (
	"strings"
	"time"
)

// Note — заметка. Минимальная сущность-образец: её достаточно, чтобы показать
// полный вертикальный срез (domain → usecase → repository → transport).
type Note struct {
	ID        int64
	OwnerID   int64 // владелец (пользователь); проставляет usecase из текущей сессии
	Title     string
	Body      string
	CreatedAt time.Time
}

// NewNote — конструктор-валидатор. Все инварианты сущности проверяются здесь,
// в одном месте: усечение пробелов, обязательность заголовка, лимит длины.
// ID и CreatedAt проставляются на этапе сохранения (репозиторием/часами).
func NewNote(title, body string) (Note, error) {
	title = strings.TrimSpace(title)
	body = strings.TrimSpace(body)

	if title == "" {
		return Note{}, ErrValidation{Field: "title", Msg: "заголовок обязателен"}
	}
	if len(title) > 200 {
		return Note{}, ErrValidation{Field: "title", Msg: "заголовок длиннее 200 символов"}
	}
	if len(body) > 10000 {
		return Note{}, ErrValidation{Field: "body", Msg: "текст длиннее 10000 символов"}
	}

	return Note{Title: title, Body: body}, nil
}
