from parser import JsonProcessor, JsonHandler, RustHandler, GoHandler
import sys
import json
from durability import *
from util import *

def main():
    processor = JsonProcessor()

    processor.register_handler(RustHandler()) 
    processor.register_handler(GoHandler()) 

    for line in sys.stdin:
        try:
            point = processor.process_string(line)
        except json.JSONDecodeError:
            print("skipping invalid JSON line", line)

        if point is not None : print(f"point: {point}")
    # push_metric(point)

if __name__ == "__main__":
    main()