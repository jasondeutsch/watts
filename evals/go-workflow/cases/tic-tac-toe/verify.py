"""Independent terminal acceptance checks; does not edit the implementation."""
import itertools
import json
import pathlib
import re
import subprocess
import sys
import tempfile

project = pathlib.Path(sys.argv[1]).resolve()
lines = [(1, 2, 3), (4, 5, 6), (7, 8, 9), (1, 4, 7),
         (2, 5, 8), (3, 6, 9), (1, 5, 9), (3, 5, 7)]
results = []
with tempfile.TemporaryDirectory(prefix="watts-tictactoe-") as directory:
    binary = pathlib.Path(directory) / "game"
    subprocess.run(["go", "build", "-o", str(binary), "."], cwd=project, check=True)

    def check(name, moves, pattern=None):
        try:
            run = subprocess.run([str(binary)], input=moves, text=True,
                                 capture_output=True, timeout=5, cwd=project)
            output = run.stdout + run.stderr
            passed = run.returncode == 0 and (pattern is None or re.search(pattern, output, re.I))
            results.append({"name": name, "pass": bool(passed),
                            "exit_code": run.returncode, "output": output})
        except subprocess.TimeoutExpired:
            results.append({"name": name, "pass": False, "error": "did not terminate"})

    for winner in "XO":
        for line in lines:
            needed = 2 if winner == "X" else 3
            opponents = next(c for c in itertools.combinations(set(range(1, 10)) - set(line), needed)
                             if not any(set(win) <= set(c) for win in lines))
            moves = []
            for index, cell in enumerate(line):
                if winner == "O":
                    moves.append(opponents[index])
                moves.append(cell)
                if winner == "X" and index < 2:
                    moves.append(opponents[index])
            check(f"{winner}-win-{line}", "\n".join(map(str, moves)) + "\n",
                  rf"(?:{winner}[^\n]*(?:win|victor)|(?:winner|victor)[^\n]*{winner})")
    check("draw", "1\n2\n3\n5\n4\n6\n8\n7\n9\n", r"draw|tie")
    check("invalid-input-retains-turn", "bad\n0\n10\n1\n1\n4\n2\n5\n3\n",
          r"(?:X[^\n]*(?:win|victor)|(?:winner|victor)[^\n]*X)")
    check("empty-eof", "")
    check("partial-eof", "1\n2\n")
print(json.dumps(results, indent=2))
sys.exit(0 if all(result["pass"] for result in results) else 1)
