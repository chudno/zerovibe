package domain

import "fmt"

// Доменные ошибки — sentinel-значения и типы, по которым верхние слои принимают
// решения (транспорт мапит их в HTTP-коды). Проверять через errors.Is / errors.As.
//
// ОБРАЗЕЦ: новые предсказуемые ошибки добавляются сюда, чтобы транспортный слой
// мог единообразно их обрабатывать (см. internal/transport/web/errors.go).

// ErrNotFound — запрошенная сущность не существует. → HTTP 404.
type ErrNotFound struct {
	Entity string
	ID     int64
}

func (e ErrNotFound) Error() string {
	return fmt.Sprintf("%s с id=%d не найден", e.Entity, e.ID)
}

// ErrValidation — нарушен инвариант сущности (некорректный ввод). → HTTP 400.
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
