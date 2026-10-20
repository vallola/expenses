package http

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"text/template"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/vallola/expenses/internal/domain"
	"github.com/vallola/expenses/internal/service"
	"go.uber.org/zap"
	"moul.io/chizap"
)

type Server struct {
	service *service.ExpenseService
	router  *chi.Mux
	log		*zap.Logger
}

func NewServer(svc *service.ExpenseService, log *zap.Logger) *Server {
	s := &Server{
		service: svc,
		router:  chi.NewRouter(),
		log: log,
	}
	s.setupRoutes()
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

func (s *Server) setupRoutes() {
	s.router.Use(chizap.New(s.log, &chizap.Opts{
		WithReferer:   true,
		WithUserAgent: true,
	}))
	s.router.Use(middleware.Recoverer)

	s.router.Get("/", s.handleDashboard)

	s.router.Route("/api", func(r chi.Router) {
		r.Get("/users", s.listUsers)
		r.Get("/expenses", s.listExpenses)
		r.Post("/expenses", s.createExpense)
		r.Put("/expenses/{id}", s.updateExpense)
		r.Delete("/expenses/{id}", s.deleteExpense)
	})
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.ParseFiles("templates/dashboard.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	tmpl.Execute(w, nil)
}

func (s *Server) listUsers(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()
	
	users, err := s.service.GetUsers(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(users)
}

func (s *Server) listExpenses(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()
	filter := domain.ExpenseFilter{}

	if uidStr := r.URL.Query().Get("user_id"); uidStr != "" {
		if uid, err := strconv.ParseInt(uidStr, 10, 64); err == nil {
			filter.UserID = &uid
		}
	}
	if cat := r.URL.Query().Get("category"); cat != "" {
		filter.Category = &cat
	}

	if startStr := r.URL.Query().Get("start_date"); startStr != "" {
		if t, err := time.Parse(time.RFC3339, startStr); err == nil {
			filter.StartDate = &t
		} else if t, err := time.Parse("2006-01-02T15:04", startStr); err == nil {
			filter.StartDate = &t
		}
	}

	if endStr := r.URL.Query().Get("end_date"); endStr != "" {
		if t, err := time.Parse(time.RFC3339, endStr); err == nil {
			filter.EndDate = &t
		} else if t, err := time.Parse("2006-01-02T15:04", endStr); err == nil {
			filter.EndDate = &t
		}
	}

	expenses, err := s.service.ListExpenses(ctx, filter)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(expenses)
}

func (s *Server) createExpense(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()
	var e domain.Expense

	if err := json.NewDecoder(r.Body).Decode(&e); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if e.SpentAt.IsZero() {
		e.SpentAt = time.Now()
	}

	created, err := s.service.CreateExpense(ctx, &e)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(created)
}

func (s *Server) updateExpense(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid expense ID", http.StatusBadRequest)
		return
	}

	var e domain.Expense
	if err := json.NewDecoder(r.Body).Decode(&e); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	e.ID = id

	if err := s.service.UpdateExpense(ctx, &e); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "updated"})
}

func (s *Server) deleteExpense(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid expense ID", http.StatusBadRequest)
		return
	}

	if err := s.service.DeleteExpense(ctx, id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
}
