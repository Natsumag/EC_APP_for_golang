package main

import (
	"flag"
	"fmt"
	"github.com/joho/godotenv"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"
)

const version = "1.0.0"

type config struct {
	port int
	smtp struct {
		host     string
		port     int
		username string
		password string
		frommail string
	}
	weburl string
}

type application struct {
	config   config
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
	var cfg config

	port, _ := strconv.Atoi(os.Getenv("MICRO_PORT"))
	smtpport, _ := strconv.Atoi(os.Getenv("SMTP_PORT"))
	flag.IntVar(&cfg.port, "port", port, "server port to listen on")
	flag.StringVar(&cfg.smtp.host, "smtphost", os.Getenv("SMTP_HOST"), "smtp host")
	flag.StringVar(&cfg.smtp.username, "smtpuser", os.Getenv("SMTP_USER"), "smtp user")
	flag.StringVar(&cfg.smtp.password, "smtppass", os.Getenv("SMTP_PASSWORD"), "smtp pass")
	flag.IntVar(&cfg.smtp.port, "smtpport", smtpport, "smtp port")
	flag.StringVar(&cfg.smtp.frommail, "frommail", os.Getenv("SMTP_FROM_MAIL"), "from mail address")
	flag.StringVar(&cfg.weburl, "web_url", os.Getenv("WEB_URL"), "web url")
	flag.Parse()

	infoLog := log.New(os.Stdout, "INFO\t", log.Ldate|log.Ltime)
	errorLog := log.New(os.Stdout, "ERROR\t", log.Ldate|log.Ltime|log.Lshortfile)

	app := &application{
		config:   cfg,
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
		Addr:              fmt.Sprintf(":%d", app.config.port),
		Handler:           app.routes(),
		IdleTimeout:       30 * time.Second,
		ReadTimeout:       10 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      5 * time.Second,
	}

	app.infoLog.Println(fmt.Sprintf("Starting invoice microservice on port %d", app.config.port))
	return srv.ListenAndServe()
}
