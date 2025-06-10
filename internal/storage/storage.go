package storage

import (
	"database/sql"
	"fmt"
	"io"
	"log/slog"
	"os"

	_ "github.com/mattn/go-sqlite3"
)

var logger *slog.Logger
var db *sql.DB

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
	dbPath := "./../../internal/storage/recordings.db"

	// Удаляем старую БД, если нужно (раскомментируйте, если нужно сбросить БД при каждом запуске)
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
	var exists bool
	err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM recordings WHERE url = ?)", url).Scan(&exists)
	if err != nil {
		logger.Error("Failed to check URL existence", "error", err)
		return false, err
	}
	return exists, nil
}
func AddURL(url string, shortURL string) error {
	exists, err := URLExists(url)
	if err != nil {
		return err
	}
	if exists {
		logger.Info("URL already exists", "url", url)
		return nil // или вернуть ошибку, если нужно
	}

	query := `INSERT INTO recordings (url, shortURL) VALUES (?, ?)`
	_, err = db.Exec(query, url, shortURL)
	if err != nil {
		logger.Error("Failed to add link", "error", err)
		return fmt.Errorf("failed to add link: %v", err)
	}

	logger.Info("Link added", "url", url, "short_url", shortURL)
	return nil
}
func GetOriginalURL(shortURL string) (string, error) {
	var url string
	row := db.QueryRow("SELECT url FROM recordings WHERE shortURL = ?", shortURL)
	err := row.Scan(&url)

	switch {
	case err == sql.ErrNoRows:
		logger.Warn("Short URL not found", "short_url", shortURL)
		return "", nil // или return "", ErrShortURLNotFound
	case err != nil:
		logger.Error("Failed to scan row", "error", err)
		return "", err
	}

	return url, nil
}
