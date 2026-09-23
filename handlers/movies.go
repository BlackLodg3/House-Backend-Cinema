package handlers

import (
	"encoding/json"
	"net/http"
	"log"

	"house-backend/db"
)

type NewMovieRequest struct {
	UserID    int    `json:"user_id"`
	MovieName string `json:"movie_name"`
}

type NewMovieResponse struct {
	ID        int64  `json:"id"`
	Title     string `json:"title"`
	AddedBy   int    `json:"added_by"`
	Message   string `json:"message"`
}

type ToggleWatchRequest struct {
	UserID  int  `json:"user_id"`
	MovieID int  `json:"movie_id"`
	Wants   bool `json:"wants"`
}

type ToggleWatchResponse struct {
	UserID  int  `json:"user_id"`
	MovieID int  `json:"movie_id"`
	Wants   bool `json:"wants"`
	Message string `json:"message"`
}

type MovieDetail struct {
	ID        int      `json:"id"`
	Title     string   `json:"title"`
	AddedBy   string   `json:"added_by"`
	Watched   bool     `json:"watched"`
	VotedYes  []string `json:"voted_yes"`
	VotedNo   []string `json:"voted_no"`
}

type WatchMoviesRequest struct {
	UserID int `json:"user_id"`
}

type WatchMoviesResponse struct {
	UserID  int           `json:"user_id"`
	Movies  []MovieDetail `json:"movies"`
	Message string        `json:"message"`
}

type SpinPoolRequest struct {
	PresentUserIDs []int `json:"present_user_ids"`
	StrictAttendance bool `json:"strict_attendance"`
}

type SpinPoolResponse struct {
	Movies  []MovieDetail `json:"movies"`
	Count   int           `json:"count"`
	Message string        `json:"message"`
}

type MovieWinnerRequest struct {
	MovieID        int   `json:"movie_id"`
	PresentUserIDs []int `json:"present_user_ids"` // Lista degli ID presenti alla serata
}

type MovieWinnerResponse struct {
	MovieID       int    `json:"movie_id"`
	SuggestedByID int    `json:"suggested_by_id"`
	Message       string `json:"message"`
}


func HandleNewMovie(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req NewMovieRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	if req.MovieName == "" || req.UserID <= 0 {
		http.Error(w, "Missing movie_name or user_id", http.StatusBadRequest)
		return
	}

	// 1. Begin a transaction
	tx, err := db.DB.Begin()
	if err != nil {
		http.Error(w, "Failed to start database transaction", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback() // Safe to call; no-op if transaction was already committed

	// 2. Insert the new movie into the movies table
	res, err := tx.Exec("INSERT INTO movies (flatmate_id, title) VALUES (?, ?)", req.UserID, req.MovieName)
	if err != nil {
		http.Error(w, "Failed to insert movie", http.StatusInternalServerError)
		return
	}

	movieID, err := res.LastInsertId()
	if err != nil {
		http.Error(w, "Failed to retrieve inserted movie ID", http.StatusInternalServerError)
		return
	}

	// 3. Automatically insert a 'wants = 1' row for EVERY flatmate
	wantToWatchQuery := `
		INSERT INTO want_to_watch (flatmate_id, movie_id, wants)
		SELECT id, ?, 1 FROM flatmates
	`
	_, err = tx.Exec(wantToWatchQuery, movieID)
	if err != nil {
		http.Error(w, "Failed to set default watch status for flatmates", http.StatusInternalServerError)
		return
	}

	// 4. Commit the transaction
	if err := tx.Commit(); err != nil {
		http.Error(w, "Failed to commit transaction", http.StatusInternalServerError)
		return
	}

	// 5. Send success response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(NewMovieResponse{
		ID:        movieID,
		Title:     req.MovieName,
		AddedBy:   req.UserID,
		Message:   "Movie added",
	})
}

func HandleToggleWatchStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ToggleWatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	if req.UserID <= 0 || req.MovieID <= 0 {
		http.Error(w, "Missing user_id or movie_id", http.StatusBadRequest)
		return
	}

	wantsVal := 0
	if req.Wants {
		wantsVal = 1
	}

	query := `
		INSERT INTO want_to_watch (flatmate_id, movie_id, wants)
		VALUES (?, ?, ?)
		ON CONFLICT(flatmate_id, movie_id) DO UPDATE SET wants = excluded.wants
	`

	_, err := db.DB.Exec(query, req.UserID, req.MovieID, wantsVal)
	if err != nil {
		http.Error(w, "Failed to update watch status", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ToggleWatchResponse{
		UserID:  req.UserID,
		MovieID: req.MovieID,
		Wants:   req.Wants,
		Message: "Watch status updated successfully",
	})
}

func HandleGetWatchMovies(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var userID int
	if r.Method == http.MethodPost {
		var req struct {
			UserID int `json:"user_id"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		userID = req.UserID
	}

	// Recupera TUTTI i film, includendo il flag 'watched'
	movieRows, err := db.DB.Query(`
		SELECT m.id, m.title, f.name, m.watched 
		FROM movies m
		JOIN flatmates f ON m.flatmate_id = f.id
		ORDER BY m.watched ASC, m.id DESC
	`)
	if err != nil {
		http.Error(w, "Failed to fetch movies", http.StatusInternalServerError)
		return
	}
	defer movieRows.Close()

	movies := []MovieDetail{}

	for movieRows.Next() {
		var m MovieDetail
		m.VotedYes = []string{}
		m.VotedNo = []string{}

		var watchedInt int
		if err := movieRows.Scan(&m.ID, &m.Title, &m.AddedBy, &watchedInt); err != nil {
			http.Error(w, "Failed to scan movie row", http.StatusInternalServerError)
			return
		}
		m.Watched = watchedInt == 1

		// Query voti
		voteRows, err := db.DB.Query(`
			SELECT f.name, w.wants
			FROM want_to_watch w
			JOIN flatmates f ON w.flatmate_id = f.id
			WHERE w.movie_id = ?
		`, m.ID)

		if err == nil {
			for voteRows.Next() {
				var voterName string
				var wants bool

				if err := voteRows.Scan(&voterName, &wants); err != nil {
					log.Printf("Error scanning vote for movie %d: %v", m.ID, err)
					continue
				}

				if wants {
					m.VotedYes = append(m.VotedYes, voterName)
				} else {
					m.VotedNo = append(m.VotedNo, voterName)
				}
			}
			voteRows.Close()
		}

		movies = append(movies, m)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(WatchMoviesResponse{
		UserID:  userID,
		Movies:  movies,
		Message: "Movies loaded successfully",
	})
}

func HandleSpinPool(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req SpinPoolRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	if len(req.PresentUserIDs) == 0 {
		http.Error(w, "At least one present user ID required", http.StatusBadRequest)
		return
	}

	// 1. Fetch all unwatched movies
	movieRows, err := db.DB.Query(`
		SELECT m.id, m.title, f.name 
		FROM movies m
		JOIN flatmates f ON m.flatmate_id = f.id
		WHERE m.watched = 0
	`)
	if err != nil {
		http.Error(w, "Failed to query movies", http.StatusInternalServerError)
		return
	}
	defer movieRows.Close()

	// Map present IDs for fast O(1) lookup
	presentMap := make(map[int]bool)
	for _, id := range req.PresentUserIDs {
		presentMap[id] = true
	}

	eligibleMovies := []MovieDetail{}

	for movieRows.Next() {
		var m MovieDetail
		m.VotedYes = []string{}
		m.VotedNo = []string{}

		if err := movieRows.Scan(&m.ID, &m.Title, &m.AddedBy); err != nil {
			continue
		}

		// 2. Fetch votes for this movie
		voteRows, err := db.DB.Query(`
			SELECT flatmate_id, wants 
			FROM want_to_watch 
			WHERE movie_id = ?
		`, m.ID)
		if err != nil {
			continue
		}

		hasPresentNo := false
		hasAbsentYes := false

		for voteRows.Next() {
			var flatmateID, wants int
			if err := voteRows.Scan(&flatmateID, &wants); err != nil {
				continue
			}

			isPresent := presentMap[flatmateID]

			// Check conditions based on presence
			if isPresent && wants == 0 {
				hasPresentNo = true
			}
			if !isPresent && wants == 1 {
				hasAbsentYes = true
			}
		}
		voteRows.Close()

		// 3. Filter based on selected mode
		if hasPresentNo {
			// Rule 1: Always exclude if a present member voted NO
			continue
		}

		if req.StrictAttendance && hasAbsentYes {
			// Rule 2 (Specialized): Exclude if an absent member wanted to watch it
			continue
		}

		eligibleMovies = append(eligibleMovies, m)
	}

	// 4. Return eligible movies pool for frontend wheel animation
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(SpinPoolResponse{
		Movies:  eligibleMovies,
		Count:   len(eligibleMovies),
		Message: "Spin pool generated successfully",
	})
}

func HandleMovieWinner(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req MovieWinnerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	if req.MovieID <= 0 || len(req.PresentUserIDs) == 0 {
		http.Error(w, "Missing movie_id or present_user_ids", http.StatusBadRequest)
		return
	}

	// 1. Inizio transazione
	tx, err := db.DB.Begin()
	if err != nil {
		http.Error(w, "Failed to start database transaction", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	// 2. Recupera chi ha suggerito il film
	var suggestedByID int
	err = tx.QueryRow("SELECT flatmate_id FROM movies WHERE id = ?", req.MovieID).Scan(&suggestedByID)
	if err != nil {
		http.Error(w, "Movie not found", http.StatusNotFound)
		return
	}

	// 3. Salva nella cronologia dei film visti
	_, err = tx.Exec("INSERT INTO watched_history (movie_id, suggested_by_id) VALUES (?, ?)", req.MovieID, suggestedByID)
	if err != nil {
		http.Error(w, "Failed to insert into history", http.StatusInternalServerError)
		return
	}

	// 4. Segna il film come visto nello storico generale
	_, err = tx.Exec("UPDATE movies SET watched = 1 WHERE id = ?", req.MovieID)
	if err != nil {
		http.Error(w, "Failed to update movie status", http.StatusInternalServerError)
		return
	}

	// 5. Imposta wants = 0 SOLO per i coinquilini PRESENTI quella sera
	for _, userID := range req.PresentUserIDs {
		_, err = tx.Exec(`
			UPDATE want_to_watch 
			SET wants = 0 
			WHERE movie_id = ? AND flatmate_id = ?
		`, req.MovieID, userID)
		if err != nil {
			http.Error(w, "Failed to update watch status for present flatmates", http.StatusInternalServerError)
			return
		}
	}

	// 6. Commit della transazione
	if err := tx.Commit(); err != nil {
		http.Error(w, "Failed to commit transaction", http.StatusInternalServerError)
		return
	}

	// 7. Risposta di successo
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(MovieWinnerResponse{
		MovieID:       req.MovieID,
		SuggestedByID: suggestedByID,
		Message:       "Watch status updated only for present flatmates",
	})
}