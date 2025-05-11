import json
from typing import Dict, Any, Optional, Type
from abc import ABC, abstractmethod
from datetime import datetime
from durability.influx_point import InfluxPoint


def parse_timestamp(ts: str):
    return datetime.fromisoformat(ts.replace("Z", "+00:00"))

class JsonHandler(ABC):
    """Interface for handling specific JSON data structures."""
    
    @abstractmethod
    def can_handle(self, data: Dict[Any, Any]) -> bool:
        """
        Determine if this handler can process the given JSON data.
        
        Args:
            data: The parsed JSON data
            
        Returns:
            True if this handler can process the data, False otherwise
        """
        pass
    
    @abstractmethod
    def process(self, data: Dict[Any, Any]) -> Any:
        """
        Process the JSON data.
        
        Args:
            data: The parsed JSON data
            
        Returns:
            The processing result
        """
        pass


class JsonProcessor:
    """
    Processes JSON strings using registered handlers.
    """
    
    def __init__(self):
        self.handlers = []
    
    def register_handler(self, handler: JsonHandler) -> None:
        """
        Register a new handler.
        
        Args:
            handler: The handler to register
        """
        self.handlers.append(handler)
    
    def process_string(self, input_string: str) -> Any:
        """
        Process a JSON string using registered handlers.
        
        Args:
            input_string: The JSON string to process
            
        Returns:
            The result from the first matching handler, or the original data if no handler matches
        """
        try:
            data = json.loads(input_string)
            # print(data)
            
            # Find the first handler that can handle this data
            for handler in self.handlers:
                if handler.can_handle(data):
                    return handler.process(data)
            
            print("no handlers found!")
            # No handler found
            # return data
            
        except json.JSONDecodeError as e:
            raise ValueError(f"Invalid JSON string: {e}")



class GoHandler(JsonHandler):
    def can_handle(self, data: Dict[Any, Any]) -> bool:
        return isinstance(data, dict) and data.get("libp2p") == "golang" 
    
    def process(self, data: Dict[Any, Any]) -> Any:
        timestamp = parse_timestamp(data.get("timestamp"))
        

class RustHandler(JsonHandler):
    def can_handle(self, data: Dict[Any, Any]) -> bool:
        return isinstance(data, dict) and "fields" in data
    
    def process(self, data: Dict[Any, Any]) -> Any:
        timestamp = parse_timestamp(data.get("timestamp"))
        fields = data.get("fields", {})
        node_id = 0
        spans = data.get("spans", [])
        
        for span in spans:
            if "node_id" in span:
                node_id = int(span["node_id"])
                break
        
        msg_text = fields.get("message", "")

        if "Message already received" in msg_text:
            msg_id = fields.get("message_id")
            if msg_id:
                return InfluxPoint(
                    measurement="duplicate_message_event",
                    labels={
                        "node": node_id,
                        "event": "received",
                        "msg_id": str(msg_id)
                    },
                    value=1,
                    log_timestamp=timestamp
                )

        elif "Put message in duplicate_cache" in msg_text:
            msg_id = fields.get("message_id")

            return InfluxPoint(
                measurement="message_delivery_time",
                labels={
                    "node": node_id,
                    "event": "received",
                    "msg_id": str(msg_id)
                },
                value=1,
                log_timestamp=timestamp
            )
