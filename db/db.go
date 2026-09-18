package db

import (
    "database/sql"
    "log"
    "os"
    "path/filepath"
    "strings"

    "github.com/joho/godotenv"
    _ "modernc.org/sqlite"
)

var DB *sql.DB

func InitDB() {
    if err := godotenv.Load(); err != nil {
        log.Println("Note: No .env file found, using system environment variables")
    }

    dbPath := os.Getenv("DB_PATH")
    if dbPath == "" {
        dbPath = "data/house_data.db"
    }

    // Crea la cartella parent se non esiste
    dir := filepath.Dir(dbPath)
    if err := os.MkdirAll(dir, 0755); err != nil {
        log.Fatalf("Failed to create db directory: %v", err)
    }

    var err error
    DB, err = sql.Open("sqlite", dbPath+"?_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)")
    if err != nil {
        log.Fatalf("Failed to connect to database: %v", err)
    }

    schema := `
    CREATE TABLE IF NOT EXISTS flatmates (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        name TEXT NOT NULL UNIQUE,
        password TEXT NOT NULL,
        movie_counter INTEGER DEFAULT 0
    );

    CREATE TABLE IF NOT EXISTS movies (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        flatmate_id INTEGER NOT NULL,
        title TEXT NOT NULL,
        watched BOOLEAN DEFAULT 0,
        FOREIGN KEY (flatmate_id) REFERENCES flatmates(id) ON DELETE CASCADE
    );

    CREATE TABLE IF NOT EXISTS want_to_watch (
        flatmate_id INTEGER NOT NULL,
        movie_id INTEGER NOT NULL,
        wants BOOLEAN DEFAULT 1,
        PRIMARY KEY (flatmate_id, movie_id),
        FOREIGN KEY (flatmate_id) REFERENCES flatmates(id) ON DELETE CASCADE,
        FOREIGN KEY (movie_id) REFERENCES movies(id) ON DELETE CASCADE
    );

    CREATE TABLE IF NOT EXISTS watched_history (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        movie_id INTEGER NOT NULL,
        suggested_by_id INTEGER NOT NULL,
        watched_at DATETIME DEFAULT CURRENT_TIMESTAMP,
        FOREIGN KEY (movie_id) REFERENCES movies(id) ON DELETE CASCADE,
        FOREIGN KEY (suggested_by_id) REFERENCES flatmates(id) ON DELETE CASCADE
    );
    `

    if _, err := DB.Exec(schema); err != nil {
        log.Fatalf("Failed to create schema: %v", err)
    }

    seedFlatmates()
}

func seedFlatmates() {
    rawNames := os.Getenv("FLATMATES")
    housePassword := os.Getenv("PASSWORD")

    if rawNames == "" {
        log.Println("No FLATMATES set in environment")
        return
    }

    names := strings.Split(rawNames, ",")

    // Usa ON CONFLICT per aggiornare la password se il nome esiste già nel DB
    stmt, err := DB.Prepare(`
        INSERT INTO flatmates (name, password) VALUES (?, ?)
        ON CONFLICT(name) DO UPDATE SET password = excluded.password
    `)
    if err != nil {
        log.Fatalf("Failed to prepare seed statement: %v", err)
    }
    defer stmt.Close()

    for _, name := range names {
        trimmedName := strings.TrimSpace(name)
        if trimmedName != "" {
                _, err := stmt.Exec(trimmedName, housePassword)
                if err != nil {
                        log.Printf("Failed to seed user %s: %v", trimmedName, err)
                }
        }
    }
    log.Printf("Flatmates seeded/updated from environment: %v", names)
}