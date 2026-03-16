package database

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"time"

	_ "github.com/lib/pq"
)

// DB — экспортируемый тип для использования в других пакетах
type DB = sql.DB

// Config содержит конфигурацию подключения к базе данных
type Config struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
	SSLMode  string
}

// NewConfig создаёт конфигурацию из переменных окружения
func NewConfig() *Config {
	return &Config{
		Host:     getEnv("DB_HOST", "localhost"),
		Port:     getEnv("DB_PORT", "5432"),
		User:     getEnv("DB_USER", "postgres"),
		Password: getEnv("DB_PASSWORD", "password"),
		DBName:   getEnv("DB_NAME", "ssubench"),
		SSLMode:  getEnv("DB_SSLMODE", "disable"),
	}
}

// Connect подключается к базе данных с таймаутами и настройками пула
// Добавлен client_encoding=UTF8 для корректной работы с кириллицей
func Connect() (*sql.DB, error) {
	cfg := NewConfig()

	// Строка подключения с client_encoding=UTF8 для поддержки кириллицы
	// и connect_timeout для быстрого обнаружения проблем с подключением
	connStr := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s client_encoding=UTF8 connect_timeout=10",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.DBName, cfg.SSLMode,
	)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Проверка подключения с таймаутом 5 секунд
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Настройка пула соединений
	db.SetMaxOpenConns(25)                  // Максимальное количество открытых соединений
	db.SetMaxIdleConns(5)                   // Максимальное количество неактивных соединений
	db.SetConnMaxLifetime(30 * time.Minute) // Максимальное время жизни соединения

	return db, nil
}

// getEnv возвращает значение переменной окружения или значение по умолчанию
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
