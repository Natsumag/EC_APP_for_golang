package config

import (
	"os"
	"strconv"
)

type Config struct {
	Port   int
	Env    string
	DB     struct{ DSN string }
	Stripe struct{ Secret, Key string }
	SMTP   struct {
		Host, Username, Password, FromMail string
		Port                               int
	}
	SecretKey   string
	WebURL      string
	APIURL      string
	MicroURL    string
	Status      map[string]int
	IsRecurring map[string]int
}

func LoadConfig() Config {
	port, _ := strconv.Atoi(os.Getenv("API_PORT"))
	smtpport, _ := strconv.Atoi(os.Getenv("SMTP_PORT"))
	dsn := os.Getenv("DB_USER") + ":" + os.Getenv("DB_PASSWORD") + "@tcp(" + os.Getenv("DB_HOST") + ":" + os.Getenv("DB_PORT") + ")/" + os.Getenv("DB_NAME") + "?parseTime=true&tls=false"

	return Config{
		Port: port,
		Env:  "development", // Default value if not provided
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
		SecretKey: os.Getenv("SECRETKEY"),
		WebURL:    os.Getenv("WEB_URL"),
		APIURL:    os.Getenv("API_URL"),
		MicroURL:  os.Getenv("MICRO_URL"),
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
