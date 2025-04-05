#!/usr/bin/env python3
import json
import re
import sys
import requests
PUSHGATEWAY_URL = "http://31.220.102.41:9091/metrics/job/shadow_sim"
def count_control_messages(log_path):
    """
    Parse a Rust libp2p log file (one JSON object per line) and
    count the occurrences of GRAFT, PRUNE, IHAVE, and IWANT messages.
    """
    counters = {
        "GRAFT": 0,
        "PRUNE": 0,
        "IHAVE": 0,
        "IWANT": 0
    }
    
    with open(log_path, "r", encoding="utf-8") as f:
        for line_number, line in enumerate(f, start=1):
            line = line.strip()
            if not line:
                continue
            
            # Parse JSON
            try:
                data = json.loads(line)
            except json.JSONDecodeError:
                # Skip malformed lines
                continue
            
            # Extract the 'message' field from the "fields" object
            fields = data.get("fields", {})
            msg = fields.get("message", "")
            
            # Convert to uppercase once to handle e.g. "Sending GRAFT" or "sending graft"
            msg_up = msg.upper()
            
            # Look for control-message keywords
            if "GRAFT" in msg_up:
                counters["GRAFT"] += 1
            if "PRUNE" in msg_up:
                counters["PRUNE"] += 1
            if "IHAVE" in msg_up:
                counters["IHAVE"] += 1
            if "IWANT" in msg_up:
                counters["IWANT"] += 1

    return counters

def main():
#    log_file = "rs-ps-shadow-data-1000.stdout"
#    results = count_control_messages(log_file)
#    print("Control message counts:")
#    for k, v in results.items():
#        print(f"  {k}: {v}")
#
    for line in sys.stdin:
        if "message" in line:
            match = re.search("Message already received", line)
            data = """# HELP shadow_messages_sent The number of messages sent.
# TYPE shadow_messages_sent counter
shadow_messages_sent{node_id="test"} 42
"""
            resp = requests.post(PUSHGATEWAY_URL, data=data)
            if resp.status_code not in [200, 202]:
                print("Pushgateway error:", resp.status_code, resp.text)

if __name__ == "__main__":
    main()

