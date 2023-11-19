package main

import (
	"embed"
	"fmt"
	"log"
	"myapp/internal/config"
	"myapp/internal/mailer"
	"net/http"
	"os"
	"time"
)

//go:embed email-templates/*
var emailTemplatesFS embed.FS
var loadConfig = config.LoadConfig()

type application struct {
	config   config.Config
	infoLog  *log.Logger
	errorLog *log.Logger
}

func main() {
	infoLog := log.New(os.Stdout, "INFO\t", log.Ldate|log.Ltime)
	errorLog := log.New(os.Stdout, "ERROR\t", log.Ldate|log.Ltime|log.Lshortfile)
	mailer.SetEmailTemplatesFS(emailTemplatesFS)

	app := &application{
		config:   loadConfig,
		infoLog:  infoLog,
		errorLog: errorLog,
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
