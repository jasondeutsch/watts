# Tic-Tac-Toe

A Go implementation of tic-tac-toe with both terminal and web interfaces.

## Running

### Terminal mode (default)

```
go run .
```

Or after building:

```
go build -o ttt .
./ttt
```

### Web mode

```
go run . -addr=localhost:8080
```

Or after building:

```
./ttt -addr=localhost:8080
```

The web server listens on the address specified by `-addr`. By default, it only listens on localhost (loopback). To expose it to other machines on your network, change the address:

```
go run . -addr=0.0.0.0:8080
```

## Playing

- **Terminal**: Enter a cell number (1-9) when prompted. The board displays position numbers on empty cells.
- **Web**: Open http://localhost:8080 in a browser. Click any empty cell to place your mark. Click "Reset Game" to start a new game.

## Testing

```
go test ./...
```