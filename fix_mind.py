import sys
sys.stdout.reconfigure(encoding='utf-8')
import requests, json

s = requests.Session()
resp = s.post('http://127.0.0.1:15153/api/auth/login', json={'email':'macie@requiem.cfd','password':'Cw231936!'})
print('Login:', resp.status_code)

# Get current mind state
resp = s.get('http://127.0.0.1:15153/api/mind')
data = resp.json()

# Archive misleading beliefs
for b in data['beliefs']:
    print(f"Belief: {b['subject']} | conf={b['confidence']}")
    # Archive beliefs about not having memory
    if 'no access' in b['content'].lower() or 'cannot retrieve' in b['content'].lower() or 'no memory' in b['content'].lower() or 'no internal' in b['content'].lower() or 'data access' in b['content'].lower():
        resp = s.post('http://127.0.0.1:15153/api/mind/beliefs/archive', json={'belief_id': b['id']})
        print(f'  Archived: {resp.status_code}')

# Archive misleading opinions
for o in data['opinions']:
    print(f"Opinion: {o['subject']} | strength={o['strength']}")
    if 'informed' in o['content'].lower() or 'does not realize' in o['content'].lower():
        resp = s.post('http://127.0.0.1:15153/api/mind/opinions/archive', json={'opinion_id': o['id']})
        print(f'  Archived: {resp.status_code}')

# Check mind after
resp = s.get('http://127.0.0.1:15153/api/mind')
data = resp.json()
print('\nAfter archiving:')
for key, val in data.items():
    count = len(val) if isinstance(val, list) else val
    print(f'  {key}: {count}')
