"""Shared helpers for the claims layer's offline study (E0) and the desks'
say-do eval (E6a): the omp model runner with a content-addressed cache,
JSON extraction from model text, and the claimslab command.

Models run only through the owner's omp CLI (no paid OpenRouter models):
  omp -p --mode json --model <model> --no-session --no-tools
Every call is cached by sha256(model, prompt) under DATA/cache, so a rerun
reproduces the study without new calls and the cache is the audit trail.
"""
import hashlib
import json
import os
import re
import subprocess
import time

DATA = os.path.expanduser(os.environ.get("CLAIMS_DATA", "~/.config/choir-qa/traces/claims"))
REPO = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
LUNA = "openai-codex/gpt-6-luna"
DEEPSEEK = "opencode-go/deepseek-v4.1-flash"
GEMINI = "google-antigravity/gemini-3.8-flash"
ALLOWED = {LUNA, DEEPSEEK, GEMINI}

# list prices (internal/llmcost/pricing.go), USD per million tokens: in, out
# gemini-3.8-flash is not in llmcost; its cost is omp-reported only.
PRICE = {LUNA: (0.10, 0.50), DEEPSEEK: (0.30, 1.20)}


def sha(text):
    return hashlib.sha256(text.encode()).hexdigest()


def omp(model, prompt, timeout=900, retries=3):
    """Run one prompt; return {text, usage, served, seconds, cached}."""
    if model not in ALLOWED:
        raise ValueError("model not allowed: " + model)
    os.makedirs(os.path.join(DATA, "cache"), exist_ok=True)
    key = sha(model + "\n" + prompt)
    path = os.path.join(DATA, "cache", key + ".json")
    if os.path.exists(path):
        out = json.load(open(path))
        out["cached"] = True
        return out
    last = ""
    for attempt in range(retries):
        t0 = time.time()
        try:
            proc = subprocess.run(["omp", "-p", "--mode", "json", "--model", model, "--no-session", "--no-tools"],
                                  input=prompt, capture_output=True, text=True, timeout=timeout)
        except subprocess.TimeoutExpired:
            last = "timeout"
            continue
        text, usage, served = None, {}, None
        for line in proc.stdout.splitlines():
            try:
                ev = json.loads(line)
            except ValueError:
                continue
            if ev.get("type") == "message_end" and (ev.get("message") or {}).get("role") == "assistant":
                msg = ev["message"]
                text = "".join(c.get("text", "") for c in msg.get("content") or [] if c.get("type") == "text")
                usage = msg.get("usage") or {}
                served = f'{msg.get("provider")}/{msg.get("model")}'
                if msg.get("stopReason") == "error":
                    last = msg.get("errorMessage") or "error"
                    text = None
        if text:
            out = {"model": model, "served": served, "text": text, "usage": usage, "seconds": round(time.time() - t0, 1)}
            json.dump(out, open(path, "w"), ensure_ascii=False)
            out["cached"] = False
            return out
        last = last or (proc.stderr or proc.stdout)[-300:]
        time.sleep(5 * (attempt + 1))
    raise RuntimeError(f"omp {model} failed: {last}")


def parse_json(text):
    """The first JSON object or array in model text (code fences allowed)."""
    t = text.strip()
    m = re.search(r"```(?:json)?\s*(.*?)```", t, re.S)
    if m:
        t = m.group(1).strip()
    for opener in ("{", "["):
        i = t.find(opener)
        if i < 0:
            continue
        depth, instr, esc = 0, False, False
        closer = "}" if opener == "{" else "]"
        for j in range(i, len(t)):
            ch = t[j]
            if instr:
                if esc:
                    esc = False
                elif ch == "\\":
                    esc = True
                elif ch == '"':
                    instr = False
                continue
            if ch == '"':
                instr = True
            elif ch == opener:
                depth += 1
            elif ch == closer:
                depth -= 1
                if depth == 0:
                    try:
                        return json.loads(t[i:j + 1])
                    except ValueError:
                        break
    raise ValueError("no JSON in model output: " + text[:200])


def list_cost(model, usage, overhead_in=0):
    """Cost at list prices, optionally net of the omp harness's own input."""
    if model not in PRICE:
        return None
    pin, pout = PRICE[model]
    inp = max(0, (usage.get("input") or 0) + (usage.get("cacheRead") or 0) - overhead_in)
    return inp * pin / 1e6 + (usage.get("output") or 0) * pout / 1e6


def claimslab(cmd, payload):
    proc = subprocess.run(["go", "run", "./internal/claimslab/cmd/claimslab", cmd], input=json.dumps(payload),
                          capture_output=True, text=True, cwd=REPO, timeout=600)
    if proc.returncode != 0:
        raise RuntimeError("claimslab: " + proc.stderr[-500:])
    return json.loads(proc.stdout)
