package repo

import (
	"database/sql"
	"loginsvc/config"

	_ "github.com/go-sql-driver/mysql"
)

type MySQLLoginRepo struct {
	db *sql.DB
}

func GetMySQLLoginRepo() *MySQLLoginRepo {
	connStr := config.GetMysqliteConnectionString()
	db, err := sql.Open("mysql", connStr)
	if err != nil {
		panic(err)
	}
	return &MySQLLoginRepo{db}
}

func (repo *MySQLLoginRepo) Name(n string) (string, error) {
	var name string
	err := repo.db.QueryRow("SELECT sid FROM users WHERE name = ?;", n).Scan(&name)
	if err != nil {
		return "", err
	}
	return name, nil
}
