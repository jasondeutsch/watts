"""Report retained stage usage; absent counters and cost remain unknown."""
import json
from pathlib import Path
run=Path(__file__).resolve().parent
task=Path('/Users/j/watts_project/test/tasks/2026-10-12-tic-tac-toe-web-ui-eval')
records=[]
for file in sorted((task/'.watts-state/steps').glob('*/usage.json')):
 item=json.loads(file.read_text());records.append({'attempt':file.parent.name,**item})
totals={}
for key in ['input_tokens','output_tokens','cache_read_tokens','cache_write_tokens','cost_usd']:
 values=[record.get(key) for record in records]
 totals[key]=sum(values) if values and all(value is not None for value in values) else None
if totals['input_tokens'] is not None and totals['output_tokens'] is not None:
 totals['reported_input_plus_output']=totals['input_tokens']+totals['output_tokens']
summary={'completed_attempt_records':records,'totals':totals,'notes':['Cache-read tokens are included in input; do not add them again.','Active or interrupted attempts may have incomplete or absent usage.','Missing cost is not zero.']}
(run/'usage-summary.json').write_text(json.dumps(summary,indent=2)+'\n')
print(json.dumps(totals))
