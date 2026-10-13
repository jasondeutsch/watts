"""Independent black-box checks for the planned local game's HTTP interface."""
import concurrent.futures
import json
import sys
import urllib.error
import urllib.request
from pathlib import Path

base=sys.argv[1].rstrip('/')
output=Path(sys.argv[2])
checks=[]

def request(path, payload=None, raw=None):
 data=raw if raw is not None else (json.dumps(payload).encode() if payload is not None else None)
 req=urllib.request.Request(base+path,data=data,headers={'Content-Type':'application/json'} if data is not None else {})
 try:
  with urllib.request.urlopen(req,timeout=5) as response:
   code,body=response.status,response.read().decode()
 except urllib.error.HTTPError as error:
  code,body=error.code,error.read().decode()
 try: value=json.loads(body)
 except ValueError: value=body
 return code,value

def state():
 code,value=request('/state'); assert code==200 and isinstance(value,dict),(code,value)
 return value

def cells(value):
 board=value['board'];assert len(board)==3 and all(len(row)==3 for row in board),board
 board=[mark for row in board for mark in row]
 return [chr(mark).strip() if isinstance(mark,int) else str(mark).strip() for mark in board]

def reset():
 code,value=request('/reset',{});assert code==200,(code,value)
 return state()

def check(name, function):
 try:
  function();checks.append({'name':name,'pass':True})
 except Exception as error:
  checks.append({'name':name,'pass':False,'error':str(error)})

def initial():
 value=reset();assert cells(value)==['']*9,value
 assert value['currentPlayer']=='X' and value['gameOver'] is False,value
check('initial-state',initial)

def turn_and_retrieval():
 reset()
 for cell,player,following in [(1,'X','O'),(4,'O','X')]:
  code,value=request('/move',{'cell':cell-1});assert code==200,(code,value)
  value=state();assert cells(value)[cell-1]==player and value['currentPlayer']==following,value
 before=state()
 code,html=request('/');assert code==200 and isinstance(html,str),(code,html)
 assert state()==before,'Reloading the HTML changed the game'
check('alternation-and-refresh-retrieval',turn_and_retrieval)

def invalid_moves():
 reset();assert request('/move',{'cell':0})[0]==200
 before=state()
 for raw in [b'{',b'{}',b'null',b'[]',b'{"cell":-1}',b'{"cell":9}',b'{"cell":0}',b'{"cell":1.5}',b'{"cell":"2"}']:
  code,_=request('/move',raw=raw);assert code>=400,(raw,code)
  assert state()==before,raw
check('invalid-payloads-and-occupied-cell-do-not-mutate',invalid_moves)

# Reuse the independently established legal sequences from the prior terminal eval.
import itertools
lines=[(1,2,3),(4,5,6),(7,8,9),(1,4,7),(2,5,8),(3,6,9),(1,5,9),(3,5,7)]
def winner(moves):
 board={move:('X' if index%2==0 else 'O') for index,move in enumerate(moves)}
 for line in lines:
  if len({board.get(cell) for cell in line})==1 and board.get(line[0]):return board[line[0]]
 return None

def sequence(line,player):
 other=[cell for cell in range(1,10) if cell not in line]
 for target in itertools.permutations(line):
  for distract in itertools.permutations(other,2 if player=='X' else 3):
   moves=[]
   for i in range(3):
    if player=='O':moves.append(distract[i])
    moves.append(target[i])
    if player=='X' and i<2:moves.append(distract[i])
   if all(winner(moves[:n]) is None for n in range(1,len(moves))) and winner(moves)==player:return moves
 raise AssertionError('No legal winning sequence')

def completed(moves,outcome):
 reset()
 for cell in moves:
  code,_=request('/move',{'cell':cell-1});assert code==200,(cell,code)
 final=state();assert final['gameOver'] is True,final
 assert final['winner']==(outcome[0] if outcome.endswith('_wins') else ''),final
 if outcome=='draw':assert all(cells(final)),final
 code,_=request('/move',{'cell':next((cell for cell in range(1,10) if cell not in moves),1)-1})
 assert code>=400 and state()==final,'Post-completion move changed state'
 assert cells(reset())==['']*9

for player in ['X','O']:
 for line in lines:
  check(f'{player}-win-{line}',lambda line=line,player=player:completed(sequence(line,player),player+'_wins'))
check('draw-completion-and-reset',lambda:completed([1,2,3,5,4,6,8,7,9],'draw'))

def concurrency():
 reset()
 with concurrent.futures.ThreadPoolExecutor(max_workers=8) as pool:
  codes=list(pool.map(lambda _:request('/move',{'cell':0})[0],range(8)))
 assert codes.count(200)==1,codes
 value=state();assert cells(value)==['X']+['']*8 and value['currentPlayer']=='O',value
 reset()
check('concurrent-conflicting-moves-serialized',concurrency)

output.write_text(json.dumps(checks,indent=2)+'\n')
print(f'{sum(check["pass"] for check in checks)}/{len(checks)} HTTP checks passed')
sys.exit(0 if all(check['pass'] for check in checks) else 1)
