package repository

import (
	"context"

	"github.com/vallola/expenses/internal/domain"
)

type Repository interface {
	UpsertUser(ctx context.Context, id int64, name string) error
	GetUsers(ctx context.Context) ([]domain.User, error)
	CreateExpense(ctx context.Context, e *domain.Expense) (*domain.Expense, error)
	ListExpenses(ctx context.Context, filter domain.ExpenseFilter) ([]domain.Expense, error)
	FindExpense(ctx context.Context, filter domain.ExpenseFilter) (*domain.Expense, error)
	UpdateExpense(ctx context.Context, e *domain.Expense) error
	DeleteExpense(ctx context.Context, id int64) error
}
