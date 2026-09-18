package handlers

import (
	"encoding/json"
	"net/http"

	"house-backend/db"
)

type Flatmate struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

func HandleListFlatmates(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	rows, err := db.DB.Query("SELECT id, name FROM flatmates ORDER BY name")
	if err != nil {
		http.Error(w, "Failed to fetch flatmates", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	flatmates := []Flatmate{}
	for rows.Next() {
		var f Flatmate
		if err := rows.Scan(&f.ID, &f.Name); err == nil {
			flatmates = append(flatmates, f)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(flatmates)
}