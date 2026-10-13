"""Capture public CLI status without treating a nonzero failure state as absent JSON."""
import datetime,json,subprocess
from pathlib import Path
root=Path('/Users/j/watts_project/test')
run=Path(__file__).resolve().parent
task='tasks/2026-10-12-tic-tac-toe-web-ui-eval'
result=subprocess.run(['/Users/j/watts_project/watts/bin/wctl','task','status',task,'--json'],cwd=root,capture_output=True,text=True)
(run/'status.json').write_text(result.stdout)
(run/'status-stderr.txt').write_text(result.stderr)
(run/'status-exit-code.txt').write_text(str(result.returncode)+'\n')
if not result.stdout.strip():
 print(json.dumps({'query_available':False,'exit_code':result.returncode,'error':result.stderr.strip()}));raise SystemExit(result.returncode)
view=json.loads(result.stdout)
identity=json.loads((run/'execution.json').read_text())
identity.update({'workflow_id':view['workflow_id'],'run_id':view['run_id'],'observed_at':datetime.datetime.now(datetime.UTC).isoformat(),'status':view['state']['status'],'current':view['state'].get('current')})
(run/'execution.json').write_text(json.dumps(identity,indent=2)+'\n')
print(json.dumps({'status':view['state']['status'],'current':view['state'].get('current'),'stages':[{key:s.get(key) for key in ('name','status','attempts','last_error')} for s in view['state']['stages']]}))
