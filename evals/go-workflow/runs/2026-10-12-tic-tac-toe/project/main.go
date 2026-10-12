package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

// board represents the 3x3 tic-tac-toe board.
// Cells are indexed 0-8, corresponding to positions 1-9.
// Empty cells hold the space rune ' '.
type board [9]rune

const (
	playerX = 'X'
	playerO = 'O'
	empty   = ' '
)

// newBoard returns a board with all cells empty.
func newBoard() board {
	b := board{}
	for i := range b {
		b[i] = empty
	}
	return b
}

// render produces an ASCII grid representation of the board.
// When showNumbers is true, empty cells display their position number (1-9);
// otherwise empty cells display a space.
func render(b board, showNumbers bool) string {
	var sb strings.Builder
	for i := 0; i < 9; i += 3 {
		row := b[i : i+3]
		// Build the three cell contents
		cells := make([]string, 3)
		for j, c := range row {
			if c == empty {
				if showNumbers {
					cells[j] = fmt.Sprintf(" %d ", i+j+1)
				} else {
					cells[j] = "   "
				}
			} else {
				cells[j] = fmt.Sprintf(" %c ", c)
			}
		}
		fmt.Fprintf(&sb, " %s | %s | %s \n", cells[0], cells[1], cells[2])
		if i < 6 {
			sb.WriteString("---+---+---\n")
		}
	}
	return sb.String()
}

func main() {
	os.Exit(playGame(bufio.NewScanner(os.Stdin), os.Stdout))
}

// playGame runs the full game loop, reading moves from scanner and writing
// output to out. It returns the process exit code: 0 for every normal
// termination (win, draw, or EOF / abandoned game).
func playGame(scanner *bufio.Scanner, out io.Writer) int {
	b := newBoard()
	player := playerX

	// Print initial board with numbers and prompt
	_, _ = fmt.Fprint(out, render(b, true))
	_, _ = fmt.Fprintf(out, "Player %c, enter a cell number (1-9): ", player)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		pos, err := parseInput(line)
		if err != nil {
			_, _ = fmt.Fprintln(out, err)
			_, _ = fmt.Fprint(out, render(b, true))
			_, _ = fmt.Fprintf(out, "Player %c, enter a cell number (1-9): ", player)
			continue
		}

		if b[pos] != empty {
			_, _ = fmt.Fprintln(out, "That cell is already occupied.")
			_, _ = fmt.Fprint(out, render(b, true))
			_, _ = fmt.Fprintf(out, "Player %c, enter a cell number (1-9): ", player)
			continue
		}

		b[pos] = player

		// Check for win or draw
		if winner, won := checkWin(b); won {
			_, _ = fmt.Fprint(out, render(b, false))
			_, _ = fmt.Fprintf(out, "Player %c wins!\n", winner)
			return 0
		}
		if isDraw(b) {
			_, _ = fmt.Fprint(out, render(b, false))
			_, _ = fmt.Fprintln(out, "The game is a draw.")
			return 0
		}

		// Toggle player
		if player == playerX {
			player = playerO
		} else {
			player = playerX
		}

		_, _ = fmt.Fprint(out, render(b, true))
		_, _ = fmt.Fprintf(out, "Player %c, enter a cell number (1-9): ", player)
	}

	// EOF
	if scanner.Err() == nil {
		_, _ = fmt.Fprintln(out, "Game abandoned.")
	} else {
		_, _ = fmt.Fprintf(out, "Input error: %v\n", scanner.Err())
	}
	return 0
}

// parseInput converts a trimmed input string to a board position (0-8).
func parseInput(s string) (int, error) {
	if s == "" {
		return 0, fmt.Errorf("input cannot be empty")
	}
	if len(s) > 1 {
		return 0, fmt.Errorf("invalid input: %q is not a number between 1 and 9", s)
	}
	if s[0] < '1' || s[0] > '9' {
		return 0, fmt.Errorf("invalid input: %q is not a number between 1 and 9", s)
	}
	return int(s[0] - '1'), nil
}

// checkWin returns the winning player and true if any of the eight
// winning lines (rows, columns, diagonals) is fully occupied by one player.
func checkWin(b board) (winner rune, won bool) {
	lines := [8][3]int{
		{0, 1, 2}, {3, 4, 5}, {6, 7, 8}, // rows
		{0, 3, 6}, {1, 4, 7}, {2, 5, 8}, // columns
		{0, 4, 8}, {2, 4, 6}, // diagonals
	}
	for _, l := range lines {
		if b[l[0]] != empty && b[l[0]] == b[l[1]] && b[l[1]] == b[l[2]] {
			return b[l[0]], true
		}
	}
	return 0, false
}

// isDraw returns true when the board is full with no winner.
func isDraw(b board) bool {
	for _, c := range b {
		if c == empty {
			return false
		}
	}
	_, won := checkWin(b)
	return !won
}
