package main

import (
	"fmt"
	"github.com/joho/godotenv"
	"log"
	"myapp/internal/config"
	"net/http"
	"os"
	"time"
)

const version = "1.0.0"

var loadConfig = config.LoadConfig()

type application struct {
	config   config.Config
	infoLog  *log.Logger
	errorLog *log.Logger
	version  string
}

func init() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found")
	}
}

func main() {
	infoLog := log.New(os.Stdout, "INFO\t", log.Ldate|log.Ltime)
	errorLog := log.New(os.Stdout, "ERROR\t", log.Ldate|log.Ltime|log.Lshortfile)

	app := &application{
		config:   loadConfig,
		infoLog:  infoLog,
		errorLog: errorLog,
		version:  version,
	}
	app.CreateDirIfNotExist("./invoices")

	err := app.serve()
	if err != nil {
		log.Fatal(err)
	}
}

func (app *application) serve() error {
	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", loadConfig.MicroPort),
		Handler:           app.routes(),
		IdleTimeout:       30 * time.Second,
		ReadTimeout:       10 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      5 * time.Second,
	}

	app.infoLog.Println(fmt.Sprintf("Starting invoice microservice on port %d", loadConfig.MicroPort))
	return srv.ListenAndServe()
}
