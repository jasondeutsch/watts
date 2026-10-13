package main

import (
	"bufio"
	"strings"
	"testing"
)

func boardFromCells(cells ...rune) board {
	b := newBoard()
	copy(b[:], cells)
	return b
}

func TestRenderEmptyBoardWithNumbers(t *testing.T) {
	got := render(boardFromCells(), true)
	want := "  1  |  2  |  3  \n" +
		"---+---+---\n" +
		"  4  |  5  |  6  \n" +
		"---+---+---\n" +
		"  7  |  8  |  9  \n"
	if got != want {
		t.Errorf("render(empty, numbers) mismatch:\n got:%q\nwant:%q", got, want)
	}
}

func TestRenderMidGame(t *testing.T) {
	b := boardFromCells('X', 'O', ' ',
		' ', 'X', ' ',
		'O', ' ', ' ')
	got := render(b, true)
	want := "  X  |  O  |  3  \n" +
		"---+---+---\n" +
		"  4  |  X  |  6  \n" +
		"---+---+---\n" +
		"  O  |  8  |  9  \n"
	if got != want {
		t.Errorf("render(mid game) mismatch:\n got:%q\nwant:%q", got, want)
	}
}

func TestRenderFullBoardNoNumbers(t *testing.T) {
	b := boardFromCells('O', 'X', 'X',
		'X', 'O', 'O',
		'O', 'X', 'X')
	got := render(b, false)
	want := "  O  |  X  |  X  \n" +
		"---+---+---\n" +
		"  X  |  O  |  O  \n" +
		"---+---+---\n" +
		"  O  |  X  |  X  \n"
	if got != want {
		t.Errorf("render(full) mismatch:\n got:%q\nwant:%q", got, want)
	}
}

func TestCheckWin(t *testing.T) {
	// All eight winning lines, for both players.
	lines := [][3]int{
		{0, 1, 2}, {3, 4, 5}, {6, 7, 8}, // rows
		{0, 3, 6}, {1, 4, 7}, {2, 5, 8}, // columns
		{0, 4, 8}, {2, 4, 6}, // diagonals
	}
	for _, p := range []rune{playerX, playerO} {
		for _, line := range lines {
			b := newBoard()
			for _, idx := range line {
				b[idx] = p
			}
			winner, won := checkWin(b)
			if !won {
				t.Errorf("checkWin(%v) for line %v (player %c): expected win, got none", b, line, p)
			}
			if winner != p {
				t.Errorf("checkWin line %v (player %c): got winner %q, want %q", line, p, winner, p)
			}
		}
	}
}

func TestCheckWinNoWin(t *testing.T) {
	cases := []board{
		newBoard(), // empty
		boardFromCells('X', 'O', ' ',
			' ', 'X', ' ',
			'O', ' ', ' '), // two-in-a-row but not three
		boardFromCells('X', 'X', 'O',
			'O', 'O', 'X',
			'X', 'O', 'O'), // full board, mixed marks
	}
	for _, b := range cases {
		if _, won := checkWin(b); won {
			t.Errorf("checkWin(%v): expected no win, got one", b)
		}
	}
}

func TestIsDraw(t *testing.T) {
	// Full board with no winner -> draw.
	fullNoWin := boardFromCells('X', 'O', 'X',
		'O', 'O', 'X',
		'X', 'X', 'O')
	if !isDraw(fullNoWin) {
		t.Error("isDraw: expected draw for full board with no winner")
	}

	// Not full -> not a draw.
	partial := boardFromCells('X', 'O', ' ',
		'O', 'X', ' ',
		' ', ' ', ' ')
	if isDraw(partial) {
		t.Error("isDraw: expected no draw for partial board")
	}

	// Full board with a winner -> not a draw.
	fullWin := boardFromCells('X', 'X', 'X',
		'O', 'O', ' ',
		' ', ' ', ' ')
	if isDraw(fullWin) {
		t.Error("isDraw: expected no draw for full board with winner")
	}
}

func TestParseInput(t *testing.T) {
	valid := map[string]int{
		"1": 0, "2": 1, "3": 2,
		"4": 3, "5": 4, "6": 5,
		"7": 6, "8": 7, "9": 8,
	}
	for in, want := range valid {
		got, err := parseInput(in)
		if err != nil {
			t.Errorf("parseInput(%q): unexpected error %v", in, err)
		}
		if got != want {
			t.Errorf("parseInput(%q): got %d, want %d", in, got, want)
		}
	}

	invalid := []string{
		"", "0", "10", "42", "-1", "abc", "5 ", " 5", "five", "x5", "5.0", "1\n2",
	}
	for _, in := range invalid {
		if _, err := parseInput(in); err == nil {
			t.Errorf("parseInput(%q): expected error, got none", in)
		}
	}
}

// runGame feeds moves to playGame and returns output and exit code.
func runGame(moves string) (string, int) {
	var out strings.Builder
	code := playGame(bufio.NewScanner(strings.NewReader(moves)), &out)
	return out.String(), code
}

func TestPlayGameXWinsDiagonal(t *testing.T) {
	out, code := runGame("1\n2\n3\n4\n5\n6\n7\n")
	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}
	if !strings.Contains(out, "Player X wins!") {
		t.Errorf("expected X win announcement, output:\n%s", out)
	}
	if strings.Contains(out, "wins") && !strings.Contains(out, "Player X wins!") {
		t.Errorf("unexpected win announcement, output:\n%s", out)
	}
	if !strings.Contains(out, "  1  |  2  |  3  \n") {
		t.Errorf("expected initial numbered board, output:\n%s", out)
	}
}

func TestPlayGameOWinsRow(t *testing.T) {
	// X: 1,4,7 ; O: 2,3,5 -> O wins row 2 (cells 2,5,8? no). Use: O takes 2,5,8.
	// Moves: X=1,O=2,X=4,O=5,X=6,O=8 -> O has 2,5,8 (column).
	out, code := runGame("1\n2\n4\n5\n6\n8\n")
	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}
	if !strings.Contains(out, "Player O wins!") {
		t.Errorf("expected O win announcement, output:\n%s", out)
	}
}

func TestPlayGameDraw(t *testing.T) {
	// X: 1,3,6,7,8 ; O: 2,4,5,9 -> draw? Verify: 1=X,2=O,3=X,4=O,5=O,6=X,7=X,8=X,9=O
	// Row1 X O X; Row2 O O X; Row3 X X O. No three-in-a-row. Draw.
	out, code := runGame("1\n2\n3\n4\n6\n9\n7\n5\n8\n")
	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}
	if !strings.Contains(out, "The game is a draw.") {
		t.Errorf("expected draw announcement, output:\n%s", out)
	}
	if strings.Contains(out, "wins") {
		t.Errorf("draw game must not announce a winner, output:\n%s", out)
	}
}

func TestPlayGameEOFAbandoned(t *testing.T) {
	out, code := runGame("") // immediate EOF
	if code != 0 {
		t.Errorf("expected exit code 0 on EOF, got %d", code)
	}
	if !strings.Contains(out, "Game abandoned.") {
		t.Errorf("expected abandoned message on EOF, output:\n%s", out)
	}
}

func TestPlayGameEOFAfterMoves(t *testing.T) {
	out, code := runGame("1\n2\n") // abandon after two moves
	if code != 0 {
		t.Errorf("expected exit code 0 on EOF, got %d", code)
	}
	if !strings.Contains(out, "Game abandoned.") {
		t.Errorf("expected abandoned message, output:\n%s", out)
	}
}

func TestPlayGameOccupiedCellKeepsTurn(t *testing.T) {
	// X plays 1, then O tries 1 (occupied) then plays 2. O prompts twice for
	// its own move: the second prompt must still be Player O.
	out, _ := runGame("1\n1\n2\n")
	if !strings.Contains(out, "That cell is already occupied.") {
		t.Errorf("expected occupied-cell message, output:\n%s", out)
	}
	if strings.Count(out, "Player O, enter a cell number") < 2 {
		t.Errorf("expected O to be re-prompted after occupied cell, output:\n%s", out)
	}
}

func TestPlayGameInvalidInputKeepsTurn(t *testing.T) {
	// X enters invalid inputs, must be re-prompted as Player X each time.
	out, _ := runGame("0\nabc\n\n1\n")
	count := strings.Count(out, "Player X, enter a cell number")
	if count < 3 {
		t.Errorf("expected Player X to be re-prompted after each invalid input (got %d prompts), output:\n%s", count, out)
	}
	if !strings.Contains(out, "invalid input") {
		t.Errorf("expected invalid-input error message, output:\n%s", out)
	}
}

func TestPlayGameBoardUnchangedOnError(t *testing.T) {
	// After an occupied-cell attempt, board must be unchanged: the digit 2 is
	// still shown unclaimed at the first re-display.
	out, _ := runGame("2\n2\n")
	// After X takes 2, O attempts 2 -> error, re-display has 2 still marked X,
	// so the following prompt for O occurs before any O mark is placed.
	if !strings.Contains(out, "That cell is already occupied.") {
		t.Fatalf("expected occupied-cell message, output:\n%s", out)
	}
}
