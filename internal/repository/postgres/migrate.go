package postgres

import (
	"github.com/pressly/goose/v3"
	"go.uber.org/zap"
)

func Migrate(url string, log *zap.Logger) error {
	dbStd, err := goose.OpenDBWithDriver("postgres", url)
	if err != nil {
		log.Error("Failed to connect goose to the database", zap.Error(err))
		return err
	}

	if err := goose.Up(dbStd, "migrations"); err != nil {
		log.Error("Failed to run database migrations", zap.Error(err))
		return err
	}

	return nil
}
