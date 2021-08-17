package repo

import (
	"database/sql"
	"loginsvc/config"

	_ "github.com/mattn/go-sqlite3"
)

type SqliteLoginRepository struct {
	db *sql.DB
}

func GetSqliteLoginRepository() *SqliteLoginRepository {
	connStr := config.GetSqliteConnectionString()
	db, _ := sql.Open("sqlite3", connStr)
	return &SqliteLoginRepository{db}
}

func (repo *SqliteLoginRepository) Name(n string) (string, error) {
	var name string
	err := repo.db.QueryRow("SELECT sid FROM users WHERE name = ?;", n).Scan(&name)
	if err != nil {
		return "", err
	}
	return name, nil
}
