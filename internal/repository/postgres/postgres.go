package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/vallola/expenses/internal/domain"
	"go.uber.org/zap"
)

type Postgres struct {
	db 	*pgxpool.Pool
	log	*zap.Logger
}

func NewPostgres(db *pgxpool.Pool) *Postgres {
	return &Postgres{db: db}
}

func (r *Postgres) UpsertUser(ctx context.Context, id int64, name string) error {
	query := `INSERT INTO users (id, name) VALUES ($1, $2) ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name`
	
	_, err := r.db.Exec(ctx, query, id, name)
	
	if err != nil {
		r.log.Error("Failed to upsert user record", zap.Error(err))
	}

	return err
}

func (r *Postgres) GetUsers(ctx context.Context) ([]domain.User, error) {
	query := `SELECT id, name FROM users ORDER BY name ASC`
	
	rows, err := r.db.Query(ctx, query)
	
	if err != nil {
		r.log.Error("Failed to select users", zap.Error(err))
		return nil, err
	}
	defer rows.Close()

	var users []domain.User
	for rows.Next() {
		var u domain.User
		if err := rows.Scan(&u.ID, &u.Name); err != nil {
			r.log.Error("Failed to scan user rows", zap.Error(err))
			return nil, err
		}
		users = append(users, u)
	}

	return users, nil
}

func (r *Postgres) CreateExpense(ctx context.Context, e *domain.Expense) (*domain.Expense, error) {
	query := `
		INSERT INTO expenses (user_id, amount, category, spent_at, comment, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, NOW(), NOW())
		RETURNING id, created_at, updated_at`

	err := r.db.QueryRow(ctx, query, e.UserID, e.Amount, e.Category, e.SpentAt, e.Comment).Scan(&e.ID, &e.CreatedAt, &e.UpdatedAt)

	if err != nil {
		r.log.Error("Failed to create an expense", zap.Error(err))
		return nil, err
	}

	return e, err
}

func (r *Postgres) ListExpenses(ctx context.Context, filter domain.ExpenseFilter) ([]domain.Expense, error) {
	query := `
        SELECT id, user_id, amount, category, spent_at, comment, created_at, updated_at 
        FROM expenses 
        WHERE 1=1`
    
	args := make([]interface{}, 0)
	var argCount int

	if filter.UserID != nil {
		argCount++
        query += fmt.Sprintf(" AND user_id = $%d", argCount)
		args = append(args, *filter.UserID)
	} 

    if filter.StartDate != nil {
        argCount++
        query += fmt.Sprintf(" AND spent_at >= $%d", argCount)
        args = append(args, *filter.StartDate)
    }

    if filter.EndDate != nil {
        argCount++
        query += fmt.Sprintf(" AND spent_at <= $%d", argCount)
        args = append(args, *filter.EndDate)
    }

    if filter.Category != nil && *filter.Category != "" {
        argCount++
        query += fmt.Sprintf(" AND category = $%d", argCount)
        args = append(args, *filter.Category)
    }

	if filter.Comment != nil && *filter.Comment != "" {
        argCount++
        query += fmt.Sprintf(" AND comment ILIKE $%d", argCount)
        args = append(args, "%"+*filter.Comment+"%")
    }

    query += " ORDER BY spent_at DESC"

	rows, err := r.db.Query(ctx, query, args...)

	if err != nil {
		r.log.Error("Failed to query expenses", zap.Error(err))
		return nil, err
	}
	defer rows.Close()

	var expenses []domain.Expense
	for rows.Next() {
		var e domain.Expense
		if err := rows.Scan(&e.ID, &e.UserID, &e.Amount, &e.Category, &e.SpentAt, &e.Comment, &e.CreatedAt, &e.UpdatedAt); err != nil {
			r.log.Error("Failed to scan expense records", zap.Error(err))
			return nil, err
		}
		expenses = append(expenses, e)
	}

	return expenses, nil
}

func (r *Postgres) FindExpense(ctx context.Context, filter domain.ExpenseFilter) (*domain.Expense, error) {
	expenses, err := r.ListExpenses(ctx, filter)

	if err != nil {
		return nil, err
	}

	if len(expenses) == 0 {
		return nil, nil
	}

	return &expenses[0], nil
}

func (r *Postgres) UpdateExpense(ctx context.Context, e *domain.Expense) error {
	query := `
		UPDATE expenses 
		SET amount = $1, category = $2, spent_at = $3, comment = $4, updated_at = NOW() 
		WHERE id = $5 AND user_id = $6`

	_, err := r.db.Exec(ctx, query, e.Amount, e.Category, e.SpentAt, e.Comment, e.ID, e.UserID)

	if err != nil {
		r.log.Error("Failed to update an expense", zap.Error(err))
	}

	return err
}

func (r *Postgres) DeleteExpense(ctx context.Context, id int64) error {
	query := `DELETE FROM expenses WHERE id = $1`
	
	_, err := r.db.Exec(ctx, query, id)
	
	if err != nil {
		r.log.Error("Failed to delete an expense", zap.Error(err))
	}

	return err
}
