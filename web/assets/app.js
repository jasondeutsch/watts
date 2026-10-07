const $ = id => document.getElementById(id);
const human = value => (value || 'pending').replaceAll('_', ' ');
let selectedTask = '', selectedStage = '', source, snapshot, selectedLog = '';
let retryPending = false;
$('retry').onclick = async () => {
  if (retryPending) return;
  retryPending = true; $('retry').disabled = true;
  const task = selectedTask;
  try {
    const response = await fetch('/api/retry?task=' + encodeURIComponent(task), {method: 'POST'});
    if (!response.ok) throw new Error(await response.text());
    if (selectedTask === task) $('retry-message').textContent = 'Retry requested. Waiting for the worker…';
  } catch (error) {
    if (selectedTask === task) $('retry-message').textContent = 'Retry failed: ' + error.message;
  } finally { retryPending = false; $('retry').disabled = false; }
};
const element = (tag, text, className) => {
  const node = document.createElement(tag);
  if (text !== undefined) node.textContent = text;
  if (className) node.className = className;
  return node;
};

async function refreshTasks() {
  try {
    const response = await fetch('/api/tasks', {cache: 'no-store'});
    if (!response.ok) throw new Error(await response.text());
    const catalog = await response.json();
    $('project').textContent = catalog.project;
    $('count').textContent = catalog.tasks.length;
    $('tasks').replaceChildren(...catalog.tasks.map(task => {
      const button = element('button', task.name, task.path === selectedTask ? 'selected' : '');
      button.onclick = () => selectTask(task.path);
      return button;
    }));
    $('empty').hidden = catalog.tasks.length > 0;
    if (!catalog.tasks.some(task => task.path === selectedTask)) selectTask(catalog.tasks[0]?.path || '');
  } catch (error) {
    $('connection').textContent = 'Monitor unavailable';
    $('error').hidden = false;
    $('error').textContent = error.message;
  }
}

function selectTask(task) {
  source?.close(); source = undefined;
  selectedTask = task; selectedStage = ''; selectedLog = ''; snapshot = undefined;
  $('title').textContent = task.split('/').pop() || 'Select a task';
  $('status').textContent = task ? 'Connecting…' : '—';
  $('stages').replaceChildren(); $('connections').replaceChildren(); $('history').textContent = 'No attempts recorded.';
  $('log').textContent = 'No stage logs yet.'; $('details').textContent = 'Select a stage to inspect its configuration.';
  $('log-select').replaceChildren(); $('error').hidden = true; $('metadata').textContent = ''; $('updated').textContent = '';
  [...$('tasks').children].forEach(button => button.classList.toggle('selected', button.textContent === task.split('/').pop()));
  if (!task) return;
  source = new EventSource('/api/events?task=' + encodeURIComponent(task));
  source.addEventListener('snapshot', event => {
    snapshot = JSON.parse(event.data); render();
  });
  source.onerror = () => { $('connection').textContent = 'Stream disconnected · reconnecting'; $('updated').textContent = 'Displayed data may be stale'; };
}

function render() {
  const steps = snapshot.definition.steps || [], stages = snapshot.state.stages || [];
  $('connection').textContent = snapshot.error ? 'Live stream · status unavailable' : '● Live';
  $('title').textContent = selectedTask.split('/').pop();
  $('status').textContent = human(snapshot.state.status);
  $('metadata').textContent = snapshot.pinned ? 'Submitted workflow · ' + snapshot.workflow_id : 'Task workflow · not submitted';
  $('updated').textContent = 'Updated ' + new Date(snapshot.updated).toLocaleTimeString();
  const failedStage = stages.find(stage => stage.status === 'failed');
  const failure = failedStage ? `WORKFLOW FAILED · ${failedStage.name} · attempt ${failedStage.attempts}: ${failedStage.last_error || 'No error details recorded.'}` : '';
  $('error').hidden = !(snapshot.error || failure); $('error').textContent = snapshot.error || failure;
  $('retry-controls').hidden = snapshot.state.status !== 'waiting_retry' || !!snapshot.error;
  if (failedStage) {
    const log = (snapshot.logs || []).find(log => log.name === `${failedStage.name}-${failedStage.attempts}.log`);
    if (log) $('error').textContent += '\n\nAttempt output:\n' + log.text.slice(-4096);
  }
  if (!steps.some(step => step.name === selectedStage)) selectedStage = snapshot.state.current || steps[0]?.name || '';
  $('stages').replaceChildren(...steps.map(step => {
    const stage = stages.find(stage => stage.name === step.name);
    const status = stage?.status || (snapshot.error ? 'unknown' : 'pending');
    const button = element('button', undefined, 'stage ' + status + (selectedStage === step.name ? ' selected' : ''));
    button.dataset.stage = step.name;
    button.append(element('b', step.name), element('span', human(status) + ' · attempt ' + (stage?.attempts || 0)), element('small', step.model || step.capability || (step.human ? 'Human decision' : 'Step')));
    button.onclick = () => { selectedStage = step.name; render(); };
    return button;
  }));
  drawConnections();
  const step = steps.find(step => step.name === selectedStage), stage = stages.find(stage => stage.name === selectedStage);
  $('details').replaceChildren();
  if (step) {
    const dl = element('dl');
    Object.entries({Stage: step.name, Capability: step.capability || (step.human ? 'human' : 'agent'), Agent: step.agent || '—', Identity: step.identity || '—', Model: [step.provider, step.model].filter(Boolean).join('/') || '—', Inputs: (step.inputs || []).join(', ') || '—', Outputs: (step.outputs || []).join(', ') || '—'}).forEach(([label, value]) => dl.append(element('dt', label), element('dd', value)));
    $('details').append(dl);
    if (stage?.last_error) $('details').append(element('p', stage.last_error));
  }
  const history = snapshot.state.history || [];
  $('history').replaceChildren(...history.map(attempt => {
    const entry = element('div', undefined, 'history-entry');
    entry.append(element('b', `${attempt.step} · attempt ${attempt.attempt} · ${attempt.error ? 'failed' : attempt.outcome || 'complete'}`));
    if (attempt.error || attempt.feedback) entry.append(element('p', attempt.error || attempt.feedback));
    return entry;
  }));
  if (!history.length) $('history').textContent = snapshot.error ? 'History unavailable until Temporal responds.' : 'No attempts recorded.';
  const logs = snapshot.logs || [];
  if (!logs.some(log => log.name === selectedLog)) selectedLog = logs.findLast(log => log.name.startsWith((snapshot.state.current || selectedStage) + '-'))?.name || logs.at(-1)?.name || '';
  $('log-select').replaceChildren(...logs.map(log => { const option = element('option', log.name); option.value = log.name; return option; }));
  $('log-select').value = selectedLog;
  showLog();
}
function showLog() {
  const panel = $('log'), following = panel.scrollHeight - panel.scrollTop - panel.clientHeight < 40;
  panel.textContent = snapshot?.logs?.find(log => log.name === selectedLog)?.text || 'No output yet for this attempt.';
  if (following) panel.scrollTop = panel.scrollHeight;
}
$('log-select').onchange = event => { selectedLog = event.target.value; showLog(); };
refreshTasks();
setInterval(refreshTasks, 5000);

// Measure the wrapped cards so connections follow both rows and workflow routes.
function drawConnections() {
  const svg = $('connections');
  svg.replaceChildren();
  const ns = 'http://www.w3.org/2000/svg';
  const make = (tag, attributes) => {
    const node = document.createElementNS(ns, tag);
    Object.entries(attributes).forEach(([name, value]) => node.setAttribute(name, value));
    return node;
  };
  const defs = make('defs', {}), marker = make('marker', {id: 'arrow', viewBox: '0 0 10 10', refX: 9, refY: 5, markerWidth: 6, markerHeight: 6, orient: 'auto-start-reverse'});
  marker.append(make('path', {d: 'M 0 0 L 10 5 L 0 10 z', fill: '#8499af'}));
  defs.append(marker); svg.append(defs);
  const bounds = $('workflow').getBoundingClientRect();
  const cards = new Map([...$('stages').children].map(card => [card.dataset.stage, card.getBoundingClientRect()]));
  const steps = snapshot?.definition.steps || [];
  const connect = (from, to, label, rework) => {
    const a = cards.get(from), b = cards.get(to);
    if (!a || !b) return;
    let path;
    if (!rework && Math.abs(a.top - b.top) < 2 && b.left > a.left) {
      path = `M ${a.right-bounds.left} ${a.top+a.height/2-bounds.top} L ${b.left-bounds.left} ${b.top+b.height/2-bounds.top}`;
    } else {
      const x1 = a.left+a.width/2-bounds.left, x2 = b.left+b.width/2-bounds.left;
      const y1 = a.bottom-bounds.top, y2 = b.top-bounds.top;
      if (b.top > a.top && !rework) {
        const middle = (y1+y2)/2;
        path = `M ${x1} ${y1} V ${middle} H ${x2} V ${y2}`;
      } else {
        const gutter = Math.max(a.bottom,b.bottom)-bounds.top+18;
        path = `M ${x1+12} ${y1} V ${gutter} H ${x2-12} V ${b.bottom-bounds.top}`;
      }
    }
    const line = make('path', {d: path, class: 'connection'+(rework ? ' rework' : ''), 'marker-end': 'url(#arrow)'});
    const title = make('title', {}); title.textContent = `${from} → ${to}${label ? ' · '+label : ''}`;
    line.append(title); svg.append(line);
  };
  steps.forEach((step, index) => {
    const targets = Object.keys(step.transitions || {}).length ? Object.entries(step.transitions) : [['', step.next || steps[index+1]?.name]];
    targets.forEach(([outcome, target]) => connect(step.name, target, outcome, steps.findIndex(candidate => candidate.name === target) <= index));
    if (step.on_failure) connect(step.name, step.on_failure, 'failure', true);
  });
}
new ResizeObserver(drawConnections).observe($('stages'));
