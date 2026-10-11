package main

import (
	"bytes"
	"strings"
	"testing"
)

// Board renderings, pinned as exact literals so any change to the UI contract
// in renderBoard fails these tests.

const boardEmpty = ` 1 | 2 | 3
---+---+---
 4 | 5 | 6
---+---+---
 7 | 8 | 9
`

const boardX1 = ` X | 2 | 3
---+---+---
 4 | 5 | 6
---+---+---
 7 | 8 | 9
`

const boardX1O9 = ` X | 2 | 3
---+---+---
 4 | 5 | 6
---+---+---
 7 | 8 | O
`

const boardX12O9 = ` X | X | 3
---+---+---
 4 | 5 | 6
---+---+---
 7 | 8 | O
`

const boardX12O89 = ` X | X | 3
---+---+---
 4 | 5 | 6
---+---+---
 7 | O | O
`

const boardX123O89 = ` X | X | X
---+---+---
 4 | 5 | 6
---+---+---
 7 | O | O
`

const drawBoardO3 = ` X | 2 | O
---+---+---
 4 | 5 | 6
---+---+---
 7 | 8 | 9
`

const drawBoardX2 = ` X | X | O
---+---+---
 4 | 5 | 6
---+---+---
 7 | 8 | 9
`

const drawBoardO5 = ` X | X | O
---+---+---
 4 | O | 6
---+---+---
 7 | 8 | 9
`

const drawBoardX7 = ` X | X | O
---+---+---
 4 | O | 6
---+---+---
 X | 8 | 9
`

const drawBoardO4 = ` X | X | O
---+---+---
 O | O | 6
---+---+---
 X | 8 | 9
`

const drawBoardX6 = ` X | X | O
---+---+---
 O | O | X
---+---+---
 X | 8 | 9
`

const drawBoardO8 = ` X | X | O
---+---+---
 O | O | X
---+---+---
 X | O | 9
`

const drawBoardFinal = ` X | X | O
---+---+---
 O | O | X
---+---+---
 X | O | X
`

// User-facing message literals, pinned as exact strings.
const (
	errInvalidCellMsg = "Invalid cell: must be a number from 1 to 9"
	errInvalidCell    = errInvalidCellMsg + "\n"
	promptX           = "Player X (1-9): "
	promptO           = "Player O (1-9): "
)

// mkBoard builds a [9]string board from nine cell values in keypad order.
func mkBoard(cells ...string) [9]string {
	var b [9]string
	copy(b[:], cells)
	return b
}

// boardWithLine returns a fresh board whose given cells are all marked.
func boardWithLine(mark string, line [3]int) [9]string {
	cells := mkBoard("1", "2", "3", "4", "5", "6", "7", "8", "9")
	for _, i := range line {
		cells[i] = mark
	}
	return cells
}

func TestRenderBoard(t *testing.T) {
	t.Run("empty board shows numbers 1-9", func(t *testing.T) {
		cells := mkBoard("1", "2", "3", "4", "5", "6", "7", "8", "9")
		if got := renderBoard(cells); got != boardEmpty {
			t.Errorf("renderBoard(empty) mismatch:\ngot:  %q\nwant: %q", got, boardEmpty)
		}
	})

	t.Run("mixed board shows marks and remaining numbers", func(t *testing.T) {
		cells := mkBoard("X", "2", "3", "4", "O", "6", "7", "8", "9")
		want := ` X | 2 | 3
---+---+---
 4 | O | 6
---+---+---
 7 | 8 | 9
`
		if got := renderBoard(cells); got != want {
			t.Errorf("renderBoard(mixed) mismatch:\ngot:  %q\nwant: %q", got, want)
		}
	})
}

func TestValidMove(t *testing.T) {
	open := mkBoard("1", "2", "3", "4", "5", "6", "7", "8", "9")

	t.Run("valid cells 1-9 accepted", func(t *testing.T) {
		for cell := 1; cell <= 9; cell++ {
			ok, msg := validMove(open, cell)
			if !ok || msg != "" {
				t.Errorf("validMove(open, %d) = (%v, %q), want (true, \"\")", cell, ok, msg)
			}
		}
	})

	t.Run("out-of-range rejected", func(t *testing.T) {
		for _, cell := range []int{-1, 0, 10, 11} {
			ok, msg := validMove(open, cell)
			if ok || msg != errInvalidCellMsg {
				t.Errorf("validMove(open, %d) = (%v, %q), want (false, %q)", cell, ok, msg, errInvalidCellMsg)
			}
		}
	})

	t.Run("occupied cell rejected with distinct message", func(t *testing.T) {
		cells := mkBoard("X", "2", "3", "4", "O", "6", "7", "8", "9")
		for _, cell := range []int{1, 5} {
			wantMsg := "Cell " + string(rune('0'+cell)) + " is already taken"
			ok, msg := validMove(cells, cell)
			if ok || msg != wantMsg {
				t.Errorf("validMove(cells, %d) = (%v, %q), want (false, %q)", cell, ok, msg, wantMsg)
			}
			if msg == errInvalidCellMsg {
				t.Errorf("occupied message %q must differ from invalid-cell message %q", msg, errInvalidCellMsg)
			}
		}
		// An open cell on the same board is still valid.
		if ok, _ := validMove(cells, 2); !ok {
			t.Errorf("validMove(cells, 2) = false, want true")
		}
	})
}

func TestCheckWin(t *testing.T) {
	assertLine := func(t *testing.T, mark string, line [3]int) {
		t.Helper()
		got, won := checkWin(boardWithLine(mark, line))
		if !won || got != mark {
			t.Errorf("checkWin(%s on %v) = (%q, %v), want (%q, true)", mark, line, got, won, mark)
		}
	}

	t.Run("row1-X", func(t *testing.T) { assertLine(t, "X", [3]int{0, 1, 2}) })
	t.Run("row1-O", func(t *testing.T) { assertLine(t, "O", [3]int{0, 1, 2}) })
	t.Run("row2-X", func(t *testing.T) { assertLine(t, "X", [3]int{3, 4, 5}) })
	t.Run("row2-O", func(t *testing.T) { assertLine(t, "O", [3]int{3, 4, 5}) })
	t.Run("row3-X", func(t *testing.T) { assertLine(t, "X", [3]int{6, 7, 8}) })
	t.Run("row3-O", func(t *testing.T) { assertLine(t, "O", [3]int{6, 7, 8}) })
	t.Run("col1-X", func(t *testing.T) { assertLine(t, "X", [3]int{0, 3, 6}) })
	t.Run("col1-O", func(t *testing.T) { assertLine(t, "O", [3]int{0, 3, 6}) })
	t.Run("col2-X", func(t *testing.T) { assertLine(t, "X", [3]int{1, 4, 7}) })
	t.Run("col2-O", func(t *testing.T) { assertLine(t, "O", [3]int{1, 4, 7}) })
	t.Run("col3-X", func(t *testing.T) { assertLine(t, "X", [3]int{2, 5, 8}) })
	t.Run("col3-O", func(t *testing.T) { assertLine(t, "O", [3]int{2, 5, 8}) })
	t.Run("diag1-X", func(t *testing.T) { assertLine(t, "X", [3]int{0, 4, 8}) })
	t.Run("diag1-O", func(t *testing.T) { assertLine(t, "O", [3]int{0, 4, 8}) })
	t.Run("diag2-X", func(t *testing.T) { assertLine(t, "X", [3]int{2, 4, 6}) })
	t.Run("diag2-O", func(t *testing.T) { assertLine(t, "O", [3]int{2, 4, 6}) })

	t.Run("in-progress board has no winner", func(t *testing.T) {
		cells := mkBoard("X", "2", "3", "4", "O", "6", "7", "8", "9")
		if got, won := checkWin(cells); won || got != "" {
			t.Errorf("checkWin(in-progress) = (%q, %v), want (\"\", false)", got, won)
		}
	})

	t.Run("full board with no winner", func(t *testing.T) {
		cells := mkBoard("X", "X", "O", "O", "O", "X", "X", "O", "X")
		if got, won := checkWin(cells); won || got != "" {
			t.Errorf("checkWin(blocked) = (%q, %v), want (\"\", false)", got, won)
		}
	})
}

func TestIsDraw(t *testing.T) {
	t.Run("full board with no winner", func(t *testing.T) {
		cells := mkBoard("X", "X", "O", "O", "O", "X", "X", "O", "X")
		if !isDraw(cells) {
			t.Errorf("isDraw(full, no winner) = false, want true")
		}
	})

	t.Run("board with a winner", func(t *testing.T) {
		// Full board where X completes the first row.
		cells := mkBoard("X", "X", "X", "O", "O", "X", "O", "X", "O")
		if isDraw(cells) {
			t.Errorf("isDraw(full, X wins) = true, want false")
		}
	})

	t.Run("in-progress board with free cells", func(t *testing.T) {
		cells := mkBoard("X", "2", "3", "4", "O", "6", "7", "8", "9")
		if isDraw(cells) {
			t.Errorf("isDraw(in-progress) = true, want false")
		}
	})
}

// TestPlayGame drives the full game loop through scripted stdin and asserts the
// exact transcript and exit code, without subprocess compilation.
func TestPlayGame(t *testing.T) {
	run := func(t *testing.T, input, want string) {
		t.Helper()
		var out bytes.Buffer
		if code := playGame(strings.NewReader(input), &out); code != 0 {
			t.Errorf("playGame returned exit code %d, want 0", code)
		}
		if got := out.String(); got != want {
			t.Errorf("output mismatch:\ngot:  %q\nwant: %q", got, want)
		}
	}

	t.Run("complete X win", func(t *testing.T) {
		// Moves: X 1, O 9, X 2, O 8, X 3 -> X completes the first row.
		input := "1\n9\n2\n8\n3\n"
		want := boardEmpty + promptX + boardX1 +
			promptO + boardX1O9 +
			promptX + boardX12O9 +
			promptO + boardX12O89 +
			promptX + boardX123O89 + "Player X wins!\n"
		run(t, input, want)
	})

	t.Run("complete draw", func(t *testing.T) {
		// Moves: X 1, O 3, X 2, O 5, X 7, O 4, X 6, O 8, X 9 -> board full, no winner.
		input := "1\n3\n2\n5\n7\n4\n6\n8\n9\n"
		want := boardEmpty + promptX + boardX1 +
			promptO + drawBoardO3 +
			promptX + drawBoardX2 +
			promptO + drawBoardO5 +
			promptX + drawBoardX7 +
			promptO + drawBoardO4 +
			promptX + drawBoardX6 +
			promptO + drawBoardO8 +
			promptX + drawBoardFinal + "It's a draw!\n"
		run(t, input, want)
	})

	t.Run("non-numeric input recovers on same turn", func(t *testing.T) {
		input := "abc\n1\n"
		want := boardEmpty + promptX + errInvalidCell +
			promptX + boardX1 +
			promptO + "Goodbye.\n"
		run(t, input, want)
	})

	t.Run("out-of-range input recovers on same turn", func(t *testing.T) {
		input := "12\n1\n"
		want := boardEmpty + promptX + errInvalidCell +
			promptX + boardX1 +
			promptO + "Goodbye.\n"
		run(t, input, want)
	})

	t.Run("occupied cell recovers on same turn", func(t *testing.T) {
		input := "1\n1\n9\n"
		want := boardEmpty + promptX + boardX1 +
			promptO + "Cell 1 is already taken\n" +
			promptO + boardX1O9 +
			promptX + "Goodbye.\n"
		run(t, input, want)
	})

	t.Run("EOF at first prompt", func(t *testing.T) {
		want := boardEmpty + promptX + "Goodbye.\n"
		run(t, "", want)
	})
}
