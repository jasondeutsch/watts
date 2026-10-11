package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// renderBoard returns a string representation of the board.
func renderBoard(cells [9]string) string {
	var b strings.Builder
	for row := 0; row < 3; row++ {
		if row > 0 {
			b.WriteString("---+---+---\n")
		}
		for col := 0; col < 3; col++ {
			idx := row*3 + col
			b.WriteString(" ")
			b.WriteString(cells[idx])
			if col < 2 {
				b.WriteString(" |")
			}
		}
		b.WriteString("\n")
	}
	return b.String()
}

// validMove checks if cell (1-9) is a valid move.
// Returns true and an empty string if valid, or false and an error message.
func validMove(cells [9]string, cell int) (bool, string) {
	if cell < 1 || cell > 9 {
		return false, "Invalid cell: must be a number from 1 to 9"
	}
	idx := cell - 1
	if cells[idx] == "X" || cells[idx] == "O" {
		return false, fmt.Sprintf("Cell %d is already taken", cell)
	}
	return true, ""
}

// checkWin checks if a player has won. Returns the winning mark and true, or
// empty string and false.
func checkWin(cells [9]string) (string, bool) {
	// All eight winning lines: rows, columns, diagonals.
	lines := [8][3]int{
		{0, 1, 2}, // row 1
		{3, 4, 5}, // row 2
		{6, 7, 8}, // row 3
		{0, 3, 6}, // col 1
		{1, 4, 7}, // col 2
		{2, 5, 8}, // col 3
		{0, 4, 8}, // diagonal \
		{2, 4, 6}, // diagonal /
	}
	for _, line := range lines {
		a, b, c := cells[line[0]], cells[line[1]], cells[line[2]]
		// All three must be the same mark (X or O), not a number.
		if a == b && b == c && (a == "X" || a == "O") {
			return a, true
		}
	}
	return "", false
}

// isDraw returns true if the board is full with no winner.
func isDraw(cells [9]string) bool {
	for _, c := range cells {
		if c != "X" && c != "O" {
			return false
		}
	}
	// A full board with a winner is not a draw. The game loop calls checkWin
	// first, so this only matters for isDraw's standalone contract.
	_, won := checkWin(cells)
	return !won
}

// playGame runs a complete game reading from r and writing to w.
// Returns 0 on normal termination and on EOF.
func playGame(r io.Reader, w io.Writer) int {
	cells := [9]string{"1", "2", "3", "4", "5", "6", "7", "8", "9"}
	player := "X"
	scanner := bufio.NewScanner(r)

	// Print initial board and first prompt.
	_, _ = fmt.Fprint(w, renderBoard(cells))
	_, _ = fmt.Fprintf(w, "Player %s (1-9): ", player)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		cell, err := strconv.Atoi(line)
		if err != nil {
			// Non-numeric input.
			_, _ = fmt.Fprintln(w, "Invalid cell: must be a number from 1 to 9")
			_, _ = fmt.Fprintf(w, "Player %s (1-9): ", player)
			continue
		}

		ok, msg := validMove(cells, cell)
		if !ok {
			_, _ = fmt.Fprintln(w, msg)
			_, _ = fmt.Fprintf(w, "Player %s (1-9): ", player)
			continue
		}

		// Apply the move.
		cells[cell-1] = player
		_, _ = fmt.Fprint(w, renderBoard(cells))

		// Check for win.
		if winner, won := checkWin(cells); won {
			_, _ = fmt.Fprintf(w, "Player %s wins!\n", winner)
			return 0
		}

		// Check for draw.
		if isDraw(cells) {
			_, _ = fmt.Fprint(w, "It's a draw!\n")
			return 0
		}

		// Switch player.
		if player == "X" {
			player = "O"
		} else {
			player = "X"
		}
		_, _ = fmt.Fprintf(w, "Player %s (1-9): ", player)
	}

	// EOF reached.
	_, _ = fmt.Fprintln(w, "Goodbye.")
	return 0
}

// main is the program entry point.
func main() {
	os.Exit(playGame(os.Stdin, os.Stdout))
}
