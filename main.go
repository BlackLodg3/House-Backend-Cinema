package main

import (
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"
	"house-backend/db"
	"house-backend/handlers"
)


func enableCORS(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		next(w, r)
	}
}

func main() {

	if err := godotenv.Load(); err != nil {
			log.Println("ℹNo .env file found, using system environment variables")
	}

	PORT := os.Getenv("PORT")
	if PORT == "" {
		return
	}
	db.InitDB()
	defer db.DB.Close()

	// Auth Route
	http.HandleFunc("/api/login", enableCORS(handlers.HandleLogin))


	http.HandleFunc("/api/flatmates", enableCORS(handlers.HandleListFlatmates))


	// Application Routes
	http.HandleFunc("/api/movies/new", enableCORS(handlers.HandleNewMovie))
	http.HandleFunc("/api/movies/toggle-status", enableCORS(handlers.HandleToggleWatchStatus))
	http.HandleFunc("/api/movies/watch-list", enableCORS(handlers.HandleGetWatchMovies))
	http.HandleFunc("/api/movies/spin-pool", enableCORS(handlers.HandleSpinPool))
	http.HandleFunc("/api/movies/winner", enableCORS(handlers.HandleMovieWinner))
	http.HandleFunc("/api/movies/length", enableCORS(handlers.HandleUpdateMovieLength))

	log.Println("🚀 Server running on http://localhost:" + PORT)
	if err := http.ListenAndServe(":" + PORT, nil); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}