"""Independent terminal acceptance checks. Run with the compiled game path."""
import itertools
import json
import re
import subprocess
import sys
from pathlib import Path

LINES = [(1,2,3),(4,5,6),(7,8,9),(1,4,7),(2,5,8),(3,6,9),(1,5,9),(3,5,7)]

def winner(moves):
    cells = {move: ('X' if index % 2 == 0 else 'O') for index, move in enumerate(moves)}
    for line in LINES:
        if len({cells.get(cell) for cell in line}) == 1 and cells.get(line[0]):
            return cells[line[0]]
    return None

def winning_sequence(line, player):
    other = [cell for cell in range(1,10) if cell not in line]
    for target in itertools.permutations(line):
        for distract in itertools.permutations(other, 2 if player == 'X' else 3):
            moves = []
            for i in range(3):
                if player == 'O': moves.append(distract[i])
                moves.append(target[i])
                if player == 'X' and i < 2: moves.append(distract[i])
            if all(winner(moves[:n]) is None for n in range(1,len(moves))) and winner(moves) == player:
                return moves
    raise AssertionError('No legal test sequence')

checks = []
def check(name, text, expected):
    try:
        p = subprocess.run([sys.argv[1]], input=text, text=True, capture_output=True, timeout=5)
        output = p.stdout + p.stderr
        passed = p.returncode == 0 and expected(output)
        checks.append({'name':name,'pass':passed,'exit_code':p.returncode,'input':text,'output':output})
    except subprocess.TimeoutExpired:
        checks.append({'name':name,'pass':False,'failure':'did not terminate within 5 seconds'})

def wins(player):
    return lambda out: bool(re.search(rf'(?:\b{player}\b[^\n]*win|win[^\n]*\b{player}\b)',out,re.I))

for player in ['X','O']:
    for line in LINES:
        moves = winning_sequence(line,player)
        check(f'{player}-win-{line}', ''.join(f'{m}\n' for m in moves), wins(player))
check('draw','1\n2\n3\n5\n4\n6\n8\n7\n9\n',lambda out: bool(re.search(r'draw|tie',out,re.I)))
# Invalid inputs precede an X-winning sequence. A skipped turn changes the result.
check('invalid-retains-turn','0\n10\nhello\n1\n1\n4\n2\n5\n3\n',wins('X'))
check('empty-EOF','',lambda out: not re.search(r'win|draw|tie',out,re.I))
check('partial-EOF','1\n4\n',lambda out: not re.search(r'win|draw|tie',out,re.I))
Path(sys.argv[2]).write_text(json.dumps(checks,indent=2)+'\n')
print(f'{sum(c["pass"] for c in checks)}/{len(checks)} terminal checks passed')
sys.exit(0 if all(c['pass'] for c in checks) else 1)
