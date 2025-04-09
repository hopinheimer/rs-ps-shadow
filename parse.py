import sys
import re
import requests
from datetime import datetime
from typing import Dict
import json
from lru import LRUCache

#duplicate: Dict[str, int] = {}
#published: Dict[str, datetime] = {}
duplicate =  LRUCache(capacity=50)
published = LRUCache(capacity=50)
peer_message :Dict[int,LRUCache] = {}


PUSHGATEWAY_URL = "http://31.220.102.41:9091/metrics/job/shadow_sim"

class PublishedMessage:
    def __init__(self, msg_hash: str, node_id: int, data: bytes):
        self.msg_hash = msg_hash
        self.node_id = node_id
        self.data = data
        self.timestamp = datetime.now()

class Ticker:
    def __init__(self):
        self.event = "test"

def parse_timestamp(ts: str):
    return datetime.fromisoformat(ts.replace("Z","+00:00"))

def push_metric(metric: str, labels: Dict[str, str], value: float):
    label_string = ",".join(f'{k}="{v}"' for k, v in labels.items())
    body = f"# TYPE {metric} gauge\n{metric}{{{label_string}}} {value}\n"
    resp = requests.post(PUSHGATEWAY_URL, data=body, headers={"Content-Type": "text/plain"})
    if resp.status_code not in (200, 202):
        print("Pushgateway error:", resp.status_code, resp.text)


def push_event(timestamp, fields, node_id):
    global duplicate
    global published
    data_lines = []
    msg_text = fields.get("message", "")
   
    if "Message already received" in msg_text:
        msg_id = fields.get("message_id")
        duplicate[msg_id] = duplicate.get(msg_id, 0) + 1
        if msg_id:
            push_metric(f"duplicate_message_event_node{node_id}", {
                "node": node_id,
                "event": "received",
                "msg_id": str(msg_id)
            }, duplicate[msg_id])

            peer_message[f"node{node_id}"] = peer_message.get(

            push_metric(f"duplicate_message_event", {
                "event": "received",
                "msg_id": str(msg_id)
            }, duplicate[msg_id])
            
            if msg_id in published:
                publish_history = published[msg_id]
                print(published)

                push_metric("message_delivery_time", {
                        "event": "received",
                        "msg_id": str(msg_id)
                }, (timestamp - publish_history).total_seconds()*1000)

    elif "Published message" in msg_text:
        msg_id = fields.get("message_id")
        published[msg_id] = timestamp
        if msg_id:
            push_metric("published_message_event", {
                "node": node_id,
                "event": "published",
                "msg_id": str(msg_id)
            }, len(published.keys()))
        
ticker = Ticker()

for line in sys.stdin:
    try:
        entry = json.loads(line)
        timestamp = parse_timestamp(entry.get("timestamp"))
        fields = entry.get("fields",{})
        push_event(timestamp,fields, 0)
    except json.JSONDecodeError:
        print("skipping invalid JSON line", line)

