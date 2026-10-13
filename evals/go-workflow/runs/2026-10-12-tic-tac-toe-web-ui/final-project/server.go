package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
)

//go:embed index.html
var embeddedFiles embed.FS

// gameState holds the authoritative game state, protected by a mutex.
type gameState struct {
	mu            sync.Mutex
	board         board
	currentPlayer rune
	winner        rune
	gameOver      bool
}

// stateResponse is the JSON response for game state.
type stateResponse struct {
	Board         [][]string `json:"board"`
	CurrentPlayer string     `json:"currentPlayer"`
	Winner        string     `json:"winner"`
	GameOver      bool       `json:"gameOver"`
}

// ErrorResponse is an error response body.
type ErrorResponse struct {
	Error string `json:"error"`
}

// moveRequest is the JSON body for a move.
type moveRequest struct {
	Cell *int `json:"cell"`
}

// newGameState creates a new game state with an empty board.
func newGameState() *gameState {
	return &gameState{
		board:         newBoard(),
		currentPlayer: playerX,
	}
}

// toStateResponse converts the current game state to a JSON-serializable response.
func (gs *gameState) toStateResponse() stateResponse {
	board := make([][]string, 3)
	for i := 0; i < 3; i++ {
		board[i] = make([]string, 3)
		for j := 0; j < 3; j++ {
			cell := gs.board[i*3+j]
			if cell == empty {
				board[i][j] = " "
			} else {
				board[i][j] = string(cell)
			}
		}
	}
	currentPlayer := string(gs.currentPlayer)
	winner := ""
	if gs.winner != 0 {
		winner = string(gs.winner)
	}
	return stateResponse{
		Board:         board,
		CurrentPlayer: currentPlayer,
		Winner:        winner,
		GameOver:      gs.gameOver,
	}
}

// startWebServer starts the HTTP server on the given address.
func startWebServer(addr string) {
	gs := newGameState()

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		data, err := embeddedFiles.ReadFile("index.html")
		if err != nil {
			log.Printf("error reading embedded index.html: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	})

	mux.HandleFunc("/state", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "method not allowed"})
			return
		}
		gs.mu.Lock()
		resp := gs.toStateResponse()
		gs.mu.Unlock()
		writeJSON(w, http.StatusOK, resp)
	})

	mux.HandleFunc("/move", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "method not allowed"})
			return
		}

		var req moveRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid JSON body"})
			return
		}

		if req.Cell == nil {
			writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "missing 'cell' field"})
			return
		}

		cell := *req.Cell

		gs.mu.Lock()
		defer gs.mu.Unlock()

		// Check if game is over
		if gs.gameOver {
			writeJSON(w, http.StatusBadRequest, gs.toStateResponse())
			return
		}

		// Validate cell range (0-8)
		if cell < 0 || cell > 8 {
			writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: fmt.Sprintf("cell %d is out of range (0-8)", cell)})
			return
		}

		// Check if cell is occupied
		if gs.board[cell] != empty {
			writeJSON(w, http.StatusBadRequest, gs.toStateResponse())
			return
		}

		// Place the mark
		gs.board[cell] = gs.currentPlayer

		// Check for win or draw
		if winner, won := checkWin(gs.board); won {
			gs.winner = winner
			gs.gameOver = true
			writeJSON(w, http.StatusOK, gs.toStateResponse())
			return
		}
		if isDraw(gs.board) {
			gs.gameOver = true
			writeJSON(w, http.StatusOK, gs.toStateResponse())
			return
		}

		// Toggle player
		if gs.currentPlayer == playerX {
			gs.currentPlayer = playerO
		} else {
			gs.currentPlayer = playerX
		}

		writeJSON(w, http.StatusOK, gs.toStateResponse())
	})

	mux.HandleFunc("/reset", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "method not allowed"})
			return
		}

		gs.mu.Lock()
		gs.board = newBoard()
		gs.currentPlayer = playerX
		gs.winner = 0
		gs.gameOver = false
		resp := gs.toStateResponse()
		gs.mu.Unlock()

		writeJSON(w, http.StatusOK, resp)
	})

	// Wrap with logging middleware
	handler := loggingMiddleware(mux)

	log.Printf("Starting web server on %s", addr)
	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

// loggingMiddleware logs each request with method, path, and status code.
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lrw := &loggingResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(lrw, r)
		log.Printf("%s %s %d", r.Method, r.URL.Path, lrw.statusCode)
	})
}

type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (lrw *loggingResponseWriter) WriteHeader(code int) {
	lrw.statusCode = code
	lrw.ResponseWriter.WriteHeader(code)
}

// wirteJSON writes a JSON response with the given status code.
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
