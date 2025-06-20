package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/redis/go-redis/v9"
)

var (
	logger *slog.Logger
	db     *sql.DB
	rdb    *redis.Client
	ctx    = context.Background()
)

func SetDB(database *sql.DB) {
	db = database
}

func init() {
	// Открываем файл для записи логов
	logFile, err := openLogFile("storage.log")
	if err != nil {
		panic("failed to open log file: " + err.Error())
	}

	// Создаем обработчик, который пишет в MultiWriter (файл + stdout)
	multiWriter := io.MultiWriter(os.Stdout, logFile)
	handler := slog.NewJSONHandler(multiWriter, &slog.HandlerOptions{
		Level: slog.LevelDebug, // записываем все уровни от Debug и выше
	})

	// Добавляем общий контекст ко всем логам этого пакета
	logger = slog.New(handler).With("source", "storage")
}

// openLogFile открывает файл для логгирования
func openLogFile(filename string) (*os.File, error) {
	return os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
}

// Connect подключается к базе данных
func Connect() (*sql.DB, error) {
	// Путь к файлу БД
	dbPath := "/home/boar/go-projects/url-shortener/internal/storage/recordings.db"

	// os.Remove(dbPath)

	// Открываем (или создаём) БД
	database, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to SQLite: %v", err)
	}

	// Проверяем подключение
	if err := database.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping SQLite: %v", err)
	}

	db = database
	logger.Info("SQLite database connected")

	// Создаем Redis клиент
	rdb = redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "", // no password set
		DB:       0,  // use default DB
	})

	return db, nil
}

// CreateTable создаёт таблицу recordings, если её нет
func CreateTable(db *sql.DB) error {
	query := `
        CREATE TABLE IF NOT EXISTS recordings (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            url TEXT NOT NULL,
            shortURL TEXT NOT NULL,
            created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
        );
    `

	if _, err := db.Exec(query); err != nil {
		logger.Error("Failed to create table", "error", err)
		return fmt.Errorf("failed to create table: %v", err)
	}
	if _, err := db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_url ON recordings(url)"); err != nil {
		logger.Error("Failed to create unique index on url", "error", err)
		return err
	}
	logger.Info("Table 'recordings' is ready")
	return nil
}
func URLExists(url string) (bool, error) {
	// 1. Проверяем кэш (Redis)
	shortURL, err := rdb.Get(ctx, url).Result()
	if err == nil && shortURL != "" {
		return true, nil // URL найден в кэше
	}

	// 2. Если ошибка (кроме "ключ не найден"), логируем и продолжаем
	if err != nil && err != redis.Nil {
		logger.Error("Failed to get short URL from Redis", "error", err)
		// Не возвращаем ошибку, продолжаем проверку в БД
	}

	// 3. Проверяем основную БД
	var exists bool
	err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM recordings WHERE url = ?)", url).Scan(&exists)
	if err != nil {
		logger.Error("Failed to check URL existence in DB", "error", err)
		return false, err
	}

	// 4. Если URL найден в БД, сохраняем в кэш (опционально)
	if exists {
		err = rdb.Set(ctx, url, "exists", 24*time.Hour).Err() // TTL = 1 день
		if err != nil {
			logger.Error("Failed to cache URL", "error", err)
		}
	}

	return exists, nil
}
func AddURL(url string, shortURL string) error {
	// 1. Проверяем существование URL (с кэшированием)
	exists, err := URLExists(url)
	if err != nil {
		return fmt.Errorf("URL check failed: %w", err)
	}
	if exists {
		logger.Info("URL already exists", "url", url)
		return nil
	}

	// 2. Начинаем транзакцию в БД
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback() // Откатываем при ошибке

	// 3. Вставляем данные с проверкой на дубликаты
	query := `INSERT INTO recordings (url, shortURL) VALUES (?, ?)
              ON CONFLICT(url) DO NOTHING`
	res, err := tx.Exec(query, url, shortURL)
	if err != nil {
		return fmt.Errorf("DB insert failed: %w", err)
	}

	// 4. Проверяем, была ли вставка
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check affected rows: %w", err)
	}
	if rowsAffected == 0 {
		logger.Info("URL was inserted by another request", "url", url)
		return nil
	}

	// 5. Кэшируем оба направления (url→shortURL и shortURL→url)
	err = rdb.Set(ctx, url, shortURL, 24*time.Hour).Err()
	if err != nil {
		logger.Error("Failed to cache url→shortURL", "error", err)
	}

	err = rdb.Set(ctx, shortURL, url, 24*time.Hour).Err()
	if err != nil {
		logger.Error("Failed to cache shortURL→url", "error", err)
	}

	// 6. Фиксируем транзакцию
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("transaction commit failed: %w", err)
	}

	logger.Info("Link added successfully",
		"url", url,
		"short_url", shortURL)
	return nil
}
func GetOriginalURL(shortURL string) (string, error) {
	// 1. Сначала проверяем кэш
	if cachedURL, err := rdb.Get(ctx, shortURL).Result(); err == nil {
		logger.Debug("Cache hit for short URL", "short_url", shortURL)
		return cachedURL, nil
	} else if err != redis.Nil {
		logger.Error("Redis error", "error", err)
		// Продолжаем запрос к БД несмотря на ошибку Redis
	}

	// 2. Если нет в кэше, идём в БД
	var url string
	err := db.QueryRow(
		"SELECT url FROM recordings WHERE shortURL = ?",
		shortURL,
	).Scan(&url)

	switch {
	case errors.Is(err, sql.ErrNoRows):
		logger.Warn("Short URL not found in database", "short_url", shortURL)
		return "", fmt.Errorf("short URL not found")
	case err != nil:
		logger.Error("Database query failed", "error", err)
		return "", fmt.Errorf("database error: %w", err)
	}

	// 3. Сохраняем в кэш перед возвратом
	if err := rdb.Set(ctx, shortURL, url, 24*time.Hour).Err(); err != nil {
		logger.Error("Failed to cache URL", "error", err)
		// Не возвращаем ошибку - основная операция успешна
	}

	logger.Debug("Retrieved original URL",
		"short_url", shortURL,
		"original_url", maskURL(url))
	return url, nil
}

// Вспомогательная функция для маскирования длинных URL в логах
func maskURL(url string) string {
	if len(url) > 50 {
		return url[:30] + "..." + url[len(url)-15:]
	}
	return url
}
