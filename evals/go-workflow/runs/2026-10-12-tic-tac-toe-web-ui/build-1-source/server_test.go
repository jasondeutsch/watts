package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// setupServer creates a new server with a fresh game state for testing.
func setupServer() *httptest.Server {
	gs := newGameState()
	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<html></html>"))
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
		if gs.gameOver {
			writeJSON(w, http.StatusBadRequest, gs.toStateResponse())
			return
		}
		if cell < 0 || cell > 8 {
			writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: fmt.Sprintf("cell %d is out of range (0-8)", cell)})
			return
		}
		if gs.board[cell] != empty {
			writeJSON(w, http.StatusBadRequest, gs.toStateResponse())
			return
		}
		gs.board[cell] = gs.currentPlayer
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

	return httptest.NewServer(mux)
}

func parseStateResponse(t *testing.T, body string) stateResponse {
	t.Helper()
	var sr stateResponse
	if err := json.Unmarshal([]byte(body), &sr); err != nil {
		t.Fatalf("failed to parse state response: %v\nbody: %s", err, body)
	}
	return sr
}

func parseErrorResponse(t *testing.T, body string) ErrorResponse {
	t.Helper()
	var er ErrorResponse
	if err := json.Unmarshal([]byte(body), &er); err != nil {
		t.Fatalf("failed to parse error response: %v\nbody: %s", err, body)
	}
	return er
}

func makeMove(t *testing.T, srv *httptest.Server, cell int) (*http.Response, string) {
	t.Helper()
	payload := fmt.Sprintf("{\"cell\":%d}", cell)
	resp, err := http.Post(srv.URL+"/move", "application/json", strings.NewReader(payload))
	if err != nil {
		t.Fatalf("POST /move failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	var body strings.Builder
	_, _ = io.Copy(&body, resp.Body)
	return resp, body.String()
}

func getState(t *testing.T, srv *httptest.Server) (*http.Response, string) {
	t.Helper()
	resp, err := http.Get(srv.URL + "/state")
	if err != nil {
		t.Fatalf("GET /state failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	var body strings.Builder
	_, _ = io.Copy(&body, resp.Body)
	return resp, body.String()
}

func postReset(t *testing.T, srv *httptest.Server) (*http.Response, string) {
	t.Helper()
	resp, err := http.Post(srv.URL+"/reset", "application/json", nil)
	if err != nil {
		t.Fatalf("POST /reset failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	var body strings.Builder
	_, _ = io.Copy(&body, resp.Body)
	return resp, body.String()
}

func TestGetState_Initial(t *testing.T) {
	srv := setupServer()
	defer srv.Close()

	resp, body := getState(t, srv)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	state := parseStateResponse(t, body)
	if state.CurrentPlayer != "X" {
		t.Errorf("expected currentPlayer X, got %s", state.CurrentPlayer)
	}
	if state.Winner != "" {
		t.Errorf("expected no winner, got %s", state.Winner)
	}
	if state.GameOver {
		t.Error("expected gameOver false")
	}
	for _, row := range state.Board {
		for _, cell := range row {
			if strings.TrimSpace(cell) != "" {
				t.Errorf("expected empty cell, got %q", cell)
			}
		}
	}
}

func TestGetState_AfterMoves(t *testing.T) {
	srv := setupServer()
	defer srv.Close()

	resp, _ := makeMove(t, srv, 0)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 for valid move, got %d", resp.StatusCode)
	}

	_, body := getState(t, srv)
	state := parseStateResponse(t, body)
	if state.Board[0][0] != "X" {
		t.Errorf("expected X at (0,0), got %q", state.Board[0][0])
	}
	if state.CurrentPlayer != "O" {
		t.Errorf("expected currentPlayer O, got %s", state.CurrentPlayer)
	}
}

func TestGetState_RefreshHydration(t *testing.T) {
	srv := setupServer()
	defer srv.Close()

	_, body1 := getState(t, srv)
	_, body2 := getState(t, srv)
	if body1 != body2 {
		t.Error("state changed between getState calls")
	}

	makeMove(t, srv, 0)
	_, body3 := getState(t, srv)
	_, body4 := getState(t, srv)
	if body3 != body4 {
		t.Error("state changed between consecutive GET /state calls")
	}
}

func TestGetState_AfterFailedRequest(t *testing.T) {
	srv := setupServer()
	defer srv.Close()

	payload := "not json"
	resp, err := http.Post(srv.URL+"/move", "application/json", strings.NewReader(payload))
	if err != nil {
		t.Fatalf("POST /move failed: %v", err)
	}
	_ = resp.Body.Close()

	_, body := getState(t, srv)
	state := parseStateResponse(t, body)
	if state.CurrentPlayer != "X" {
		t.Errorf("expected X still to move, got %s", state.CurrentPlayer)
	}
}

func TestMove_ValidCell(t *testing.T) {
	srv := setupServer()
	defer srv.Close()

	resp, body := makeMove(t, srv, 0)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	state := parseStateResponse(t, body)
	if state.Board[0][0] != "X" {
		t.Errorf("expected X at (0,0), got %q", state.Board[0][0])
	}
	if state.CurrentPlayer != "O" {
		t.Errorf("expected O to move, got %s", state.CurrentPlayer)
	}
	if state.GameOver {
		t.Error("expected game not over")
	}
}

func TestMove_OccupiedCell(t *testing.T) {
	srv := setupServer()
	defer srv.Close()

	makeMove(t, srv, 0)
	resp, body := makeMove(t, srv, 0)
	if resp.StatusCode == http.StatusOK {
		t.Error("expected non-200 for occupied cell")
	}
	state := parseStateResponse(t, body)
	if state.Board[0][0] != "X" {
		t.Errorf("expected X at (0,0), got %q", state.Board[0][0])
	}
	if state.CurrentPlayer != "O" {
		t.Errorf("expected O still to move, got %s", state.CurrentPlayer)
	}
}

func TestMove_OutOfRange(t *testing.T) {
	srv := setupServer()
	defer srv.Close()

	resp, body := makeMove(t, srv, 99)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
	errResp := parseErrorResponse(t, body)
	if errResp.Error == "" {
		t.Error("expected error message")
	}
}

func TestMove_BadPayload(t *testing.T) {
	srv := setupServer()
	defer srv.Close()

	payload := "no{ json"
	resp, err := http.Post(srv.URL+"/move", "application/json", strings.NewReader(payload))
	if err != nil {
		t.Fatalf("POST failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for bad payload, got %d", resp.StatusCode)
	}
}

func TestMove_BadPayloadMissingCell(t *testing.T) {
	srv := setupServer()
	defer srv.Close()

	payload := "{\"foo\":\"bar\"}"
	resp, err := http.Post(srv.URL+"/move", "application/json", strings.NewReader(payload))
	if err != nil {
		t.Fatalf("POST failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for missing cell, got %d", resp.StatusCode)
	}
	var body strings.Builder
	_, _ = io.Copy(&body, resp.Body)
	errResp := parseErrorResponse(t, body.String())
	if errResp.Error != "missing 'cell' field" {
		t.Errorf("expected missing cell error, got %q", errResp.Error)
	}
}

func TestMove_StringValue(t *testing.T) {
	srv := setupServer()
	defer srv.Close()

	payload := "{\"cell\":\"abc\"}"
	resp, err := http.Post(srv.URL+"/move", "application/json", strings.NewReader(payload))
	if err != nil {
		t.Fatalf("POST failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for string cell, got %d", resp.StatusCode)
	}
}

func TestMove_AfterWin(t *testing.T) {
	srv := setupServer()
	defer srv.Close()

	makeMove(t, srv, 0)               // X -> 0
	makeMove(t, srv, 3)               // O -> 3
	makeMove(t, srv, 1)               // X -> 1
	makeMove(t, srv, 4)               // O -> 4
	resp, body := makeMove(t, srv, 2) // X -> 2 wins!
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 for winning move, got %d", resp.StatusCode)
	}
	state := parseStateResponse(t, body)
	if !state.GameOver {
		t.Error("expected game over after win")
	}
	if state.Winner != "X" {
		t.Errorf("expected winner X, got %s", state.Winner)
	}

	resp, body = makeMove(t, srv, 5)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for move after win, got %d", resp.StatusCode)
	}
	state = parseStateResponse(t, body)
	if !state.GameOver {
		t.Error("expected gameOver still true")
	}
}

func TestMove_AfterDraw(t *testing.T) {
	srv := setupServer()
	defer srv.Close()

	// Draw pattern: X:0,2,5,6,7  O:1,3,4,8
	// Resulting board is a draw with no winner
	makeMove(t, srv, 0)               // X
	makeMove(t, srv, 1)               // O
	makeMove(t, srv, 2)               // X
	makeMove(t, srv, 3)               // O
	makeMove(t, srv, 5)               // X
	makeMove(t, srv, 4)               // O
	makeMove(t, srv, 6)               // X
	makeMove(t, srv, 8)               // O
	resp, body := makeMove(t, srv, 7) // X final move
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 for draw move, got %d", resp.StatusCode)
	}
	state := parseStateResponse(t, body)
	if !state.GameOver {
		t.Error("expected game over after draw")
	}
	if state.Winner != "" {
		t.Errorf("expected no winner for draw, got %s", state.Winner)
	}

	resp, _ = makeMove(t, srv, 0)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for move after draw, got %d", resp.StatusCode)
	}
}

func TestReset(t *testing.T) {
	srv := setupServer()
	defer srv.Close()

	makeMove(t, srv, 0)
	makeMove(t, srv, 3)

	resp, body := postReset(t, srv)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 for reset, got %d", resp.StatusCode)
	}
	state := parseStateResponse(t, body)
	if state.CurrentPlayer != "X" {
		t.Errorf("expected X to move after reset, got %s", state.CurrentPlayer)
	}
	if state.GameOver {
		t.Error("expected gameOver false after reset")
	}
	if state.Winner != "" {
		t.Errorf("expected no winner after reset, got %s", state.Winner)
	}
	for _, row := range state.Board {
		for _, cell := range row {
			if strings.TrimSpace(cell) != "" {
				t.Errorf("expected empty board after reset, got %q", cell)
			}
		}
	}
}
func TestReset_Twice(t *testing.T) {
	srv := setupServer()
	defer srv.Close()

	makeMove(t, srv, 0)
	postReset(t, srv)
	resp, body := postReset(t, srv)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 for second reset, got %d", resp.StatusCode)
	}
	state := parseStateResponse(t, body)
	if state.CurrentPlayer != "X" {
		t.Errorf("expected X to move after second reset, got %s", state.CurrentPlayer)
	}
}

func TestHomePage(t *testing.T) {
	srv := setupServer()
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatalf("GET / failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	ctype := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ctype, "text/html") {
		t.Errorf("expected text/html content type, got %s", ctype)
	}
}

func TestHomePage404(t *testing.T) {
	srv := setupServer()
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/nonexistent")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 for unknown path, got %d", resp.StatusCode)
	}
}

func TestGetState_NoResetOnRefresh(t *testing.T) {
	srv := setupServer()
	defer srv.Close()

	makeMove(t, srv, 0)
	_, body1 := getState(t, srv)
	_, body2 := getState(t, srv)
	if body1 != body2 {
		t.Error("GET /state returned different results on consecutive calls")
	}
}

func TestMove_AfterWin_MovesRejected(t *testing.T) {
	srv := setupServer()
	defer srv.Close()

	// X wins: cells 0,1,2 (top row); O plays 3,4
	makeMove(t, srv, 0)            // X
	makeMove(t, srv, 3)            // O
	makeMove(t, srv, 1)            // X
	makeMove(t, srv, 4)            // O
	resp, _ := makeMove(t, srv, 2) // X wins
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 for win, got %d", resp.StatusCode)
	}

	for _, cell := range []int{5, 6, 7, 8, 0} {
		resp, _ = makeMove(t, srv, cell)
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("expected 400 for move after win at cell %d, got %d", cell, resp.StatusCode)
		}
	}
}
