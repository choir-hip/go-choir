// conductor_jev_experiment.mjs — time and score Jev as the prompt-bar
// conductor: one choice question routes a prompt to a desktop app. Calls
// OpenRouter's Decisions endpoint directly with OPENROUTER_API_KEY (the same
// call the gateway's /provider/v1/judgments makes); the key is never printed.
//
//   node scripts/conductor_jev_experiment.mjs [repeats]
const key = process.env.OPENROUTER_API_KEY;
if (!key) throw new Error('OPENROUTER_API_KEY is required');
const repeats = Number(process.argv[2] || 1);

const apps = {
  texture: 'Work that needs thinking, writing, research or building: documents, analysis, answering questions, finding and reading papers, making software, games or apps.',
  mail: 'Reading, writing, replying to or sending email.',
  calendar: 'Viewing, creating or changing calendar events, meetings or schedules.',
  web_lens: 'Opening or viewing a specific web page or URL as it is live.',
  files: 'Browsing, opening, moving or organizing the owner\'s files.',
  podcast: 'Playing, listening to or finding podcast episodes or audio.',
  autopaper: 'Reading the news or the owner\'s personalized newspaper.',
};

const cases = [
  ['make a Minesweeper game', 'texture'],
  ['find a recent arXiv paper I could replicate cheaply and replicate it', 'texture'],
  ['write an email to Sarah about the event on Tuesday saying I\'m going to be 20 minutes late', 'mail'],
  ['what do I have on Thursday afternoon?', 'calendar'],
  ['move my 3pm with Dan to Friday', 'calendar'],
  ['open https://arxiv.org/abs/2310.06825', 'web_lens'],
  ['reply to the last email from my landlord', 'mail'],
  ['play the latest episode of my podcast', 'podcast'],
  ['what\'s in the news today?', 'autopaper'],
  ['find the PDF I downloaded yesterday', 'files'],
  ['summarize the tradeoffs between Raft and Paxos', 'texture'],
  ['draft a one-page plan for launching the product', 'texture'],
];

const question = {
  type: 'choice',
  instructions: 'Route the owner\'s prompt-bar request to the one desktop app that should handle it first.',
  criteria: apps,
  choices: Object.keys(apps),
};

async function route(prompt) {
  const started = performance.now();
  const response = await fetch('https://openrouter.ai/api/alpha/decisions', {
    method: 'POST',
    headers: { authorization: `Bearer ${key}`, 'content-type': 'application/json', accept: 'application/json' },
    body: JSON.stringify({ model: 'typesafe/jev-1.13', state: { prompt }, questions: { app: question } }),
  });
  const ms = Math.round(performance.now() - started);
  const body = await response.json().catch(() => null);
  const answer = body?.answers?.app;
  return { ms, status: response.status, choice: answer?.choice, p: answer?.probabilities?.[answer?.choice], error: response.ok ? undefined : JSON.stringify(body).slice(0, 200) };
}

const results = [];
for (let round = 0; round < repeats; round++) {
  for (const [prompt, expected] of cases) {
    const result = await route(prompt);
    result.correct = result.choice === expected;
    results.push({ prompt, expected, ...result });
    console.log(`${String(result.ms).padStart(5)} ms  ${result.correct ? 'ok  ' : 'MISS'}  ${String(result.choice).padEnd(9)} p=${result.p?.toFixed?.(2)}  ${prompt}${result.error ? '  ' + result.error : ''}`);
  }
}
const times = results.map((r) => r.ms).sort((a, b) => a - b);
const pct = (q) => times[Math.min(times.length - 1, Math.floor(q * times.length))];
console.log(JSON.stringify({
  calls: results.length,
  correct: results.filter((r) => r.correct).length,
  p50_ms: pct(0.5), p90_ms: pct(0.9), max_ms: times[times.length - 1], min_ms: times[0],
}));
