package main

import (
	"fmt"
	"log"
	"myapp/internal/config"
	"myapp/internal/driver"
	"myapp/internal/models"
	"net/http"
	"os"
	"time"
)

var loadConfig = config.LoadConfig()

type application struct {
	config   config.Config
	infoLog  *log.Logger
	errorLog *log.Logger
	DB       models.DBModel
}

func (app *application) serve() error {
	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", loadConfig.APIPort),
		Handler:           app.routes(),
		IdleTimeout:       30 * time.Second,
		ReadTimeout:       10 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      5 * time.Second,
	}

	app.infoLog.Println(fmt.Sprintf("Starting Backend server in %s mode on port %d", loadConfig.Env, loadConfig.APIPort))
	return srv.ListenAndServe()
}

func main() {
	infoLog := log.New(os.Stdout, "INFO\t", log.Ldate|log.Ltime)
	errorLog := log.New(os.Stdout, "ERROR\t", log.Ldate|log.Ltime|log.Lshortfile)
	conn, err := driver.OpenDB(loadConfig.DB.DSN)
	if err != nil {
		errorLog.Fatal(err)
	}
	defer conn.Close()

	app := &application{
		config:   loadConfig,
		infoLog:  infoLog,
		errorLog: errorLog,
		DB:       models.DBModel{DB: conn},
	}

	err = app.serve()
	if err != nil {
		log.Fatal(err)
	}
}
