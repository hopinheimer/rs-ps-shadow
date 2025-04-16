import sys
import re
import json
from datetime import datetime
from typing import Dict
from influxdb import InfluxDBClient
from lru import LRUCache

# LRU caches
duplicate = LRUCache(capacity=50)
published = LRUCache(capacity=50)
total = 1
current_nodes_reached = 1

# InfluxDB connection parameters
INFLUX_HOST = "localhost"
INFLUX_PORT = 8086
INFLUX_USER = "admin"
INFLUX_PASSWORD = "admin"
INFLUX_DBNAME = "metrics_db"

def parse_timestamp(ts: str):
    return datetime.fromisoformat(ts.replace("Z", "+00:00"))

def current_message_dissemination_rate():
    return (current_nodes_reached/total)*100

def get_influx_connection():
    """Create and return a connection to the InfluxDB database."""
    max_retries = 5
    retry_delay = 2  # seconds
    
    for attempt in range(max_retries):
        try:
            client = InfluxDBClient(
                host=INFLUX_HOST,
                port=INFLUX_PORT,
                username=INFLUX_USER,
                password=INFLUX_PASSWORD,
                database=INFLUX_DBNAME
            )
            return client
        except Exception as e:
            if attempt < max_retries - 1:
                print(f"Connection attempt {attempt+1} failed, retrying in {retry_delay} seconds...")
                import time
                time.sleep(retry_delay)
            else:
                print(f"Failed to connect to InfluxDB after {max_retries} attempts")
                raise

def initialize_db():
    """Create the InfluxDB database and retention policies."""
    client = InfluxDBClient(
        host=INFLUX_HOST,
        port=INFLUX_PORT,
        username=INFLUX_USER,
        password=INFLUX_PASSWORD
    )
    
    # Check if database exists, if not create it
    dbs = client.get_list_database()
    if {"name": INFLUX_DBNAME} not in dbs:
        client.create_database(INFLUX_DBNAME)
        print(f"Created database: {INFLUX_DBNAME}")
    
    # Switch to the database
    client.switch_database(INFLUX_DBNAME)
    
    # Create retention policy (optional)
    client.create_retention_policy(
        name='standard_retention',
        duration='30d',
        replication=1,
        database=INFLUX_DBNAME,
        default=True
    )
    
    print("Database initialized successfully")
    return client

def push_metric(metric: str, labels: Dict[str, str], value: float, log_timestamp: datetime):
    """Store metric in InfluxDB."""
    try:
        client = get_influx_connection()
        
        # Determine which measurement to use based on metric type
        if "duplicate_message_event" in metric:
            measurement = "duplicate_message_events"
        elif metric == "published_message_event":
            measurement = "published_message_events"
        elif metric == "message_delivery_time":
            measurement = "message_delivery_times"
        else:
            print(f"Unknown metric type: {metric}")
            return
        
        # Create the InfluxDB point
        point = {
            "measurement": measurement,
            "tags": {
                "node_id": labels.get('node', ''),
                "msg_id": labels.get('msg_id', ''),
                "event": labels.get('event', '')
            },
            "time": log_timestamp.isoformat(),
            "fields": {
                "value": value
            }
        }
        
        # Write the point to InfluxDB
        client.write_points([point])
        client.close()
    except Exception as e:
        print(f"Error pushing metric to InfluxDB: {e}")

def push_event(timestamp, fields, node_id):
    global duplicate
    global published
    global current_nodes_reached
    global total
    
    msg_text = fields.get("message", "")
    
    if "Message already received" in msg_text:
        print("message received already")
        msg_id = fields.get("message_id")
        duplicate[msg_id] = duplicate.get(msg_id, 0) + 1
        if msg_id:
            push_metric("duplicate_message_event", {
                "node": node_id,
                "event": "received",
                "msg_id": str(msg_id)
            }, 1, timestamp)  # Using 1 as value to indicate occurrence
            
    elif "Put message in duplicate_cache" in msg_text:
        print("pushing message in dup")
        current_nodes_reached += 1
        print(current_message_dissemination_rate())
        msg_id = fields.get("message_id")
        if msg_id not in published.keys():
            return
        publish_history = published[msg_id]
        print(published)
        push_metric("message_delivery_time", {
            "node": node_id,
            "event": "received",
            "msg_id": str(msg_id)
        }, (timestamp - publish_history).total_seconds()*1000, timestamp)
    
    elif "Published message" in msg_text:
        print("publishing message")
        msg_id = fields.get("message_id")
        published[msg_id] = timestamp
        if msg_id:
            push_metric("published_message_event", {
                "node": node_id,
                "event": "published",
                "msg_id": str(msg_id)
            }, 1, timestamp)  # Using 1 as value to indicate occurrence

# Initialize the database when the script starts
try:
    print("initializing metric params")
    total = 100
    initialize_db()
except Exception as e:
    print(f"Failed to initialize database: {e}")

# Process input lines
for line in sys.stdin:
    try:
        entry = json.loads(line)
        timestamp = parse_timestamp(entry.get("timestamp"))
        fields = entry.get("fields", {})
        node_id = 0
        spans = entry.get("spans", [])
        
        for span in spans:
            if "node_id" in span:
                node_id = int(span["node_id"])
                break
        push_event(timestamp, fields, node_id)
    except json.JSONDecodeError:
        print("skipping invalid JSON line", line)
