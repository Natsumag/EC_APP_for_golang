package main

import (
	"encoding/gob"
	"fmt"
	"github.com/alexedwards/scs/mysqlstore"
	"github.com/alexedwards/scs/v2"
	"github.com/joho/godotenv"
	"html/template"
	"log"
	"myapp/internal/config"
	"myapp/internal/driver"
	"myapp/internal/models"
	"net/http"
	"os"
	"time"
)

const version = "1.0.0"

var loadConfig = config.LoadConfig()
var session *scs.SessionManager

type application struct {
	config        config.Config
	infoLog       *log.Logger
	errorLog      *log.Logger
	templateCache map[string]*template.Template
	DB            models.DBModel
	Session       *scs.SessionManager
}

func (app *application) serve() error {
	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", loadConfig.WebPort),
		Handler:           app.routes(),
		IdleTimeout:       30 * time.Second,
		ReadTimeout:       10 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      5 * time.Second,
	}

	app.infoLog.Println(fmt.Sprintf("Starting HTTP server in %s mode on port %d", loadConfig.Env, loadConfig.WebPort))
	return srv.ListenAndServe()
}

func init() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found")
	}
}

func main() {
	gob.Register(TransactionData{})

	infoLog := log.New(os.Stdout, "INFO\t", log.Ldate|log.Ltime)
	errorLog := log.New(os.Stdout, "ERROR\t", log.Ldate|log.Ltime|log.Lshortfile)

	conn, err := driver.OpenDB(loadConfig.DB.DSN)
	if err != nil {
		errorLog.Fatal(err)
	}
	defer conn.Close()

	// set up session
	session = scs.New()
	session.Lifetime = 24 * time.Hour
	session.Store = mysqlstore.New(conn)

	tc := make(map[string]*template.Template)

	app := &application{
		config:        loadConfig,
		infoLog:       infoLog,
		errorLog:      errorLog,
		templateCache: tc,
		DB:            models.DBModel{DB: conn},
		Session:       session,
	}

	go app.ListenWebSocketChannel()

	err = app.serve()

	if err != nil {
		app.errorLog.Println(err)
		log.Fatal(err)
	}
}
