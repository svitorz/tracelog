package repository

// Postgres representa o adapter de persistência em PostgreSQL.
type Postgres struct {
	dsn string
}

func NewPostgres(dsn string) *Postgres {
	return &Postgres{dsn: dsn}
}
