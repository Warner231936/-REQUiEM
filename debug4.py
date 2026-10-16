import sys
sys.stdout.reconfigure(encoding='utf-8')
import requests, json

s = requests.Session()
s.post('http://127.0.0.1:15153/api/auth/login', json={'email':'macie@requiem.cfd','password':'Cw231936!'})

# Test chat
resp = s.post('http://127.0.0.1:15153/api/chat', json={'message':'What are your beliefs and preferences?', 'stream': True})
print('=== Chat about beliefs ===')
for line in resp.iter_lines():
    if line:
        data = json.loads(line)
        print('Response:', data.get('response', '')[:600])
