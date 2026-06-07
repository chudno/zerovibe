// Package web — HTTP/HTML транспорт на HTMX. Зависит от usecase и domain.
//
// КЛЮЧЕВОЙ HTMX-ПАТТЕРН (запомни для генерации):
//   - GET /            — отдаёт ПОЛНУЮ страницу (layout + список).
//   - POST /notes      — создаёт и возвращает ОДИН ФРАГМЕНТ (template "note"),
//                        htmx вставляет его в начало #notes (hx-swap="afterbegin").
//   - DELETE /notes/ID — удаляет и возвращает ПУСТОТУ, htmx убирает элемент
//                        (hx-swap="outerHTML" по самому элементу).
// То есть мутации возвращают ровно тот кусок HTML, который меняется, — не всю
// страницу. Это суть HTMX: сервер рендерит фрагменты, клиент их подменяет.
package web

import (
	"embed"
	"errors"
	"html/template"
	"net/http"
	"strconv"

	"github.com/chudno/zerovibe/internal/domain"
	"github.com/chudno/zerovibe/internal/usecase"
)

//go:embed templates/*.html
var templatesFS embed.FS

// staticFS — клиентские ассеты (htmx и т.п.), вшитые в бинарь. Раздаются по
// /static/, чтобы не зависеть от внешнего CDN в рантайме (важно для работы в РФ
// и доступности: unpkg может быть недоступен). Добавляешь файл сюда —
// автоматически доступен по /static/<имя>.
//
//go:embed static/*
var staticFS embed.FS

// pageData — данные для рендера полной страницы.
type pageData struct {
	Title string
	Notes []domain.Note
}

// Server держит зависимости транспорта: шаблоны и сервисы usecase.
type Server struct {
	tmpl  *template.Template
	notes *usecase.NoteService
}

// NewServer парсит шаблоны и собирает HTTP-обработчики.
func NewServer(notes *usecase.NoteService) (*Server, error) {
	tmpl, err := template.ParseFS(templatesFS, "templates/*.html")
	if err != nil {
		return nil, err
	}
	return &Server{tmpl: tmpl, notes: notes}, nil
}

// Routes возвращает http.Handler со всеми маршрутами.
// Используем method-pattern роутинг stdlib (Go 1.22+): "METHOD /path/{id}".
func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", s.handleIndex)
	mux.HandleFunc("POST /notes", s.handleCreate)
	mux.HandleFunc("DELETE /notes/{id}", s.handleDelete)
	mux.HandleFunc("GET /healthz", s.handleHealth)
	// Статика (htmx и пр.) из вшитого staticFS: пути вида /static/htmx.min.js.
	mux.Handle("GET /static/", http.FileServerFS(staticFS))
	return mux
}

// handleIndex — полная страница со списком заметок.
func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	// Только корень: иначе stdlib-mux ловит "/" как catch-all → 404 для прочего.
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	notes, err := s.notes.List(r.Context())
	if err != nil {
		s.fail(w, err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.tmpl.ExecuteTemplate(w, "layout", pageData{Title: "Заметки", Notes: notes}); err != nil {
		s.fail(w, err)
	}
}

// handleCreate создаёт заметку и возвращает HTML-фрагмент новой заметки.
func (s *Server) handleCreate(w http.ResponseWriter, r *http.Request) {
	n, err := s.notes.Create(r.Context(), r.FormValue("title"), r.FormValue("body"))
	if err != nil {
		s.fail(w, err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.tmpl.ExecuteTemplate(w, "note", n); err != nil {
		s.fail(w, err)
	}
}

// handleDelete удаляет заметку; при успехе возвращает пустой 200 (htmx уберёт узел).
func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "некорректный id", http.StatusBadRequest)
		return
	}
	if err := s.notes.Delete(r.Context(), id); err != nil {
		s.fail(w, err)
		return
	}
	w.WriteHeader(http.StatusOK) // пустое тело — htmx удалит элемент по hx-swap
}

// handleHealth — проба для оркестратора/прокси.
func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

// fail мапит доменные ошибки в HTTP-коды. Единая точка обработки ошибок
// транспорта (образец: новые доменные ошибки добавлять сюда).
func (s *Server) fail(w http.ResponseWriter, err error) {
	var notFound domain.ErrNotFound
	var validation domain.ErrValidation
	switch {
	case errors.As(err, &notFound):
		http.Error(w, err.Error(), http.StatusNotFound)
	case errors.As(err, &validation):
		http.Error(w, err.Error(), http.StatusBadRequest)
	default:
		http.Error(w, "внутренняя ошибка", http.StatusInternalServerError)
	}
}
