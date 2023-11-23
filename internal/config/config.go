package config

import (
	"github.com/joho/godotenv"
	"log"
	"os"
	"strconv"
)

type Config struct {
	Env       string
	WebPort   int
	APIPort   int
	MicroPort int
	WebURL    string
	APIURL    string
	MicroURL  string
	DB        struct{ DSN string }
	SMTP      struct {
		Host, Username, Password, FromMail string
		Port                               int
	}
	Stripe      struct{ Secret, Key string }
	SecretKey   string
	Status      map[string]int
	IsRecurring map[string]int
}

func init() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found")
	}
}

func LoadConfig() Config {
	webPort, _ := strconv.Atoi(os.Getenv("WEB_PORT"))
	apiPort, _ := strconv.Atoi(os.Getenv("API_PORT"))
	microPort, _ := strconv.Atoi(os.Getenv("MICRO_PORT"))
	smtpport, _ := strconv.Atoi(os.Getenv("SMTP_PORT"))
	dsn := os.Getenv("DB_USER") + ":" + os.Getenv("DB_PASSWORD") + "@tcp(" + os.Getenv("DB_HOST") + ":" + os.Getenv("DB_PORT") + ")/" + os.Getenv("DB_NAME") + "?parseTime=true&tls=false"

	return Config{
		Env:       os.Getenv("ENV"),
		WebPort:   webPort,
		APIPort:   apiPort,
		MicroPort: microPort,
		WebURL:    os.Getenv("WEB_URL"),
		APIURL:    os.Getenv("API_URL"),
		MicroURL:  os.Getenv("MICRO_URL"),
		DB: struct{ DSN string }{
			DSN: dsn,
		},
		SMTP: struct {
			Host, Username, Password, FromMail string
			Port                               int
		}{
			Host:     os.Getenv("SMTP_HOST"),
			Username: os.Getenv("SMTP_USER"),
			Password: os.Getenv("SMTP_PASSWORD"),
			FromMail: os.Getenv("SMTP_FROM_MAIL"),
			Port:     smtpport,
		},
		Stripe: struct {
			Secret, Key string
		}{
			Secret: os.Getenv("STRIPE_SECRET"),
			Key:    os.Getenv("STRIPE_KEY"),
		},
		SecretKey: os.Getenv("SECRETKEY"),
		Status: map[string]int{
			"Cleared":   1,
			"Refunded":  2,
			"Cancelled": 3,
		},
		IsRecurring: map[string]int{
			"NoRecurring": 0,
			"Recurring":   1,
		},
	}
}
