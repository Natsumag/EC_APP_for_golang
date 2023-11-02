package main

import (
	"flag"
	"fmt"
	"github.com/joho/godotenv"
	"log"
	"myapp/internal/driver"
	"myapp/internal/models"
	"net/http"
	"os"
	"strconv"
	"time"
)

const version = "1.0.0"

type config struct {
	port int
	env  string
	db   struct {
		dsn string
	}
	stripe struct {
		secret string
		key    string
	}
	smtp struct {
		host     string
		port     int
		username string
		password string
		frommail string
	}
	secretkey string
	weburl    string
}

type application struct {
	config   config
	infoLog  *log.Logger
	errorLog *log.Logger
	version  string
	DB       models.DBModel
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

	app.infoLog.Println(fmt.Sprintf("Starting Backend server in %s mode on port %d", app.config.env, app.config.port))
	return srv.ListenAndServe()
}

func init() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found")
	}
}

func main() {
	var cfg config

	port, _ := strconv.Atoi(os.Getenv("API_PORT"))
	smtpport, _ := strconv.Atoi(os.Getenv("SMTP_PORT"))
	dsn := os.Getenv("DB_USER") + ":" + os.Getenv("DB_PASSWORD") + "@tcp(" + os.Getenv("DB_HOST") + ":" + os.Getenv("DB_PORT") + ")/" + os.Getenv("DB_NAME") + "?parseTime=true&tls=false"
	flag.IntVar(&cfg.port, "port", port, "server port to listen on")
	flag.StringVar(&cfg.env, "env", "development", "Application environment {development|production|maintenance}")
	flag.StringVar(&cfg.db.dsn, "dsn", dsn, "DSN")
	flag.StringVar(&cfg.smtp.host, "smtphost", os.Getenv("SMTP_HOST"), "smtp host")
	flag.StringVar(&cfg.smtp.username, "smtpuser", os.Getenv("SMTP_USER"), "smtp user")
	flag.StringVar(&cfg.smtp.password, "smtppass", os.Getenv("SMTP_PASSWORD"), "smtp pass")
	flag.IntVar(&cfg.smtp.port, "smtpport", smtpport, "smtp port")
	flag.StringVar(&cfg.smtp.frommail, "frommail", os.Getenv("SMTP_FROM_MAIL"), "from mail address")
	flag.StringVar(&cfg.secretkey, "secretkey", os.Getenv("SECRETKEY"), "secret key")
	flag.StringVar(&cfg.weburl, "web_url", os.Getenv("WEB_URL"), "web url")
	flag.Parse()

	cfg.stripe.key = os.Getenv("STRIPE_KEY")
	cfg.stripe.secret = os.Getenv("STRIPE_SECRET")

	infoLog := log.New(os.Stdout, "INFO\t", log.Ldate|log.Ltime)
	errorLog := log.New(os.Stdout, "ERROR\t", log.Ldate|log.Ltime|log.Lshortfile)
	conn, err := driver.OpenDB(cfg.db.dsn)
	if err != nil {
		errorLog.Fatal(err)
	}
	defer conn.Close()

	app := &application{
		config:   cfg,
		infoLog:  infoLog,
		errorLog: errorLog,
		version:  version,
		DB:       models.DBModel{DB: conn},
	}

	err = app.serve()
	if err != nil {
		log.Fatal(err)
	}

}
