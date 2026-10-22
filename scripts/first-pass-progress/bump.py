#!/usr/bin/env python3
"""bump.py ITEM_ID SLICE_SUBSTRING STAGE EVIDENCE [--at ISO]: set one slice's stage and log it.

The data file is $PROGRESS_JSON, else the repo's first-pass progress file."""
import json, os, sys, datetime
p = os.environ.get('PROGRESS_JSON') or os.path.join(os.path.dirname(os.path.abspath(__file__)), '..', '..', 'docs', 'definitions', 'choir-launch-first-pass-progress.json')
d = json.load(open(p))
iid, sub, stage, ev = sys.argv[1], sys.argv[2].lower(), int(sys.argv[3]), sys.argv[4]
at = sys.argv[sys.argv.index('--at') + 1] if '--at' in sys.argv else datetime.datetime.now(datetime.timezone.utc).strftime('%Y-%m-%dT%H:%MZ')
hits = [(i, s) for i in d['items'] if i['id'] == iid for s in i['slices'] if sub in s['name'].lower()]
if len(hits) != 1:
    sys.exit('%s/%s matched %d slices: %s' % (iid, sub, len(hits), [s['name'] for i in d['items'] if i['id'] == iid for s in i['slices']]))
i, s = hits[0]
if stage > s['stage']:
    s.setdefault('history', []).append({'stage': stage, 'at': at})
s['stage'] = stage
s['evidence'] = ev
d['as_of'] = datetime.datetime.now(datetime.timezone.utc).strftime('%Y-%m-%dT%H:%MZ')
json.dump(d, open(p, 'w'), indent=1)
print(iid, s['name'], '->', stage)
