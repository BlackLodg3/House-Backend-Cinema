package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"house-backend/db"
)

type LoginRequest struct {
	Name     string `json:"name"`
	Password string `json:"password"`
}

type LoginResponse struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

func HandleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Fetch flatmate details and password directly from SQLite
	var id int
	var storedPassword string

	query := "SELECT id, password FROM flatmates WHERE LOWER(name) = LOWER(?)"
	err := db.DB.QueryRow(query, req.Name).Scan(&id, &storedPassword)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Flatmate not found", http.StatusNotFound)
			return
		}
		http.Error(w, "Database query error", http.StatusInternalServerError)
		return
	}

	// Verify password matching the stored user password
	if req.Password != storedPassword {
		http.Error(w, "Incorrect password", http.StatusUnauthorized)
		return
	}

	// Return user payload for persistent frontend state
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(LoginResponse{
		ID:   id,
		Name: req.Name,
	})
}