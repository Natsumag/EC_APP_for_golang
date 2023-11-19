package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
)

func main() {
	dbURL := ConstructDatabaseURL()
	log.Println("DATABASE_URL:", dbURL)

	if err := WriteDatabaseConfig(dbURL); err != nil {
		log.Fatalf("Failed to write database.yml: %v", err)
	}

	cmd := exec.Command("soda", "migrate", "up")
	cmd.Env = append(os.Environ(),
		"DATABASE_URL="+dbURL,
	)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout

	if err := cmd.Run(); err != nil {
		log.Fatalf("Soda migration failed: %v", err)
	}
}

func WriteDatabaseConfig(dbURL string) error {
	configContent := fmt.Sprintf("development:\n  dialect: mysql\n  url: %s\n", dbURL)
	return ioutil.WriteFile("database.yml", []byte(configContent), 0644)
}

func ConstructDatabaseURL() string {
	dbUser := GetEnv("DB_USER")
	dbPass := GetEnv("DB_PASSWORD")
	dbHost := GetEnv("DB_HOST")
	dbName := GetEnv("DB_NAME")

	return "mysql://" + dbUser + ":" + dbPass + "@tcp(" + dbHost + ":3306)/" + dbName + "?charset=utf8&parseTime=True&loc=Local&multiStatements=true"
}

func GetEnv(key string) string {
	value := os.Getenv(key)
	if value == "" {
		log.Fatalf("Environment variable %s not set", key)
	}
	return value
}
