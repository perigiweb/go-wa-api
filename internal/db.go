package internal

import (
	"database/sql"
	"time"
	"log"

	_ "github.com/jackc/pgx/v5/stdlib"
)

//var dbConn *sql.DB

func DBConnect() *sql.DB {
	dsn, _ := GetEnvString("DB_DSN")
	maxOpenConn, _ := GetEnvInt("DB_MAX_OPEN_CONNECTION")
	maxIdleConn, _ := GetEnvInt("DB_MAX_IDLE_CONNECTION")

	dbConn, err := sql.Open("pgx", dsn)

	if err != nil {
		log.Fatal(err)
	}

	dbConn.SetConnMaxLifetime(time.Minute * 3)
	dbConn.SetMaxOpenConns(maxOpenConn)
	dbConn.SetMaxIdleConns(maxIdleConn)

	//defer dbConn.Close()

	return dbConn
}