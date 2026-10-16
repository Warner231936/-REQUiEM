import sys
sys.stdout.reconfigure(encoding='utf-8')
import requests, json

s = requests.Session()
s.post('http://127.0.0.1:15153/api/auth/login', json={'email':'macie@requiem.cfd','password':'Cw231936!'})

# Test SSE connection with heartbeats
resp = s.get('http://127.0.0.1:15153/api/events/stream', stream=True, headers={'Accept':'text/event-stream'}, timeout=35)
print('SSE status:', resp.status_code)

# Read heartbeats for 35 seconds
import time
count = 0
start = time.time()
for line in resp.iter_lines():
    count += 1
    elapsed = time.time() - start
    print(f'[{elapsed:.0f}s] Line:', line[:80] if line else '(empty)')
    if count > 3:
        break

# Test chat
resp = s.post('http://127.0.0.1:15153/api/chat', json={'message':'What are your beliefs and preferences?', 'stream': True})
print('\n=== Chat ===')
for line in resp.iter_lines():
    if line:
        data = json.loads(line)
        print('Response:', data.get('response', '')[:400])
