package repository

import (
	"database/sql"
)

type Postgres struct {
	db *sql.DB
}

func NewPostgres(dsn string) (*Postgres, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}

	return &Postgres{db: db}, nil
}

func (p *Postgres) GetDB() *sql.DB {
	return p.db
}

func (p *Postgres) Ping() error {
	return p.db.Ping()
}

func (p *Postgres) Close() error {
	return p.db.Close()
}
