#!/usr/bin/env python3
"""Refuse rollback to a host that cannot enforce live quality restrictions."""
import json,os,sys,urllib.request
base=os.environ.get('CPA_MANAGEMENT_URL','http://127.0.0.1:8317').rstrip('/')
key=os.environ.get('CPA_MANAGEMENT_KEY')
if not key:sys.exit('Set CPA_MANAGEMENT_KEY in the process environment; it is never logged.')
request=urllib.request.Request(base+'/v8/management/credentials/quality-pauses',headers={'Authorization':'Bearer '+key})
try:
    with urllib.request.urlopen(request) as response:data=json.load(response)
except Exception:sys.exit('BLOCKED: cannot verify authoritative pause state. Keep patched host and traffic stopped.')
actions=data.get('actions',data.get('pauses'))
if not isinstance(actions,list):sys.exit('BLOCKED: unknown authority response schema.')
if actions:sys.exit(f'BLOCKED: {len(actions)} durable quality pauses remain. Keep patched host or explicitly resolve each restriction using its authenticated CAS release endpoint.')
print('No active quality pauses. Stop the plugin and host before switching binaries; retain both databases and configuration backups.')
