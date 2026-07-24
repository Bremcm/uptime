// Package domain описывает основные сущности сервиса — то, чем оперирует
// программа, на языке предметной области. Здесь нет ни базы, ни HTTP, ни JSON:
// только чистые понятия. Всё остальное зависит от domain, а domain — ни от чего.
package domain

// Monitor — это настройка слежения за одним ресурсом.
type Monitor struct {
	URL             string
	Name            string
	UserID          int64
	IntervalSeconds int
	Enabled         bool
}
