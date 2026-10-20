package service

import (
	"context"

	"github.com/vallola/expenses/internal/domain"
	"github.com/vallola/expenses/internal/repository"
)

type ExpenseService struct {
	repo repository.Repository
}

func NewExpenseService(repo repository.Repository) *ExpenseService {
	return &ExpenseService{repo: repo}
}

func (s *ExpenseService) SaveUser(ctx context.Context, id int64, name string) error {
	return s.repo.UpsertUser(ctx, id, name)
}

func (s *ExpenseService) GetUsers(ctx context.Context) ([]domain.User, error) {
	return s.repo.GetUsers(ctx)
}

func (s *ExpenseService) CreateExpense(ctx context.Context, e *domain.Expense) (*domain.Expense, error) {
	return s.repo.CreateExpense(ctx, e)
}

func (s *ExpenseService) ListExpenses(ctx context.Context, filter domain.ExpenseFilter) ([]domain.Expense, error) {
	return s.repo.ListExpenses(ctx, filter)
}

func (s *ExpenseService) FindExpense(ctx context.Context, filter domain.ExpenseFilter) (*domain.Expense, error) {
	return s.repo.FindExpense(ctx, filter)
}

func (s *ExpenseService) UpdateExpense(ctx context.Context, e *domain.Expense) error {
	return s.repo.UpdateExpense(ctx, e)
}

func (s *ExpenseService) DeleteExpense(ctx context.Context, id int64) error {
	return s.repo.DeleteExpense(ctx, id)
}
