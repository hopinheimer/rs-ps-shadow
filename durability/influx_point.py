from typing import Dict
from datetime import datetime

class InfluxPoint:
    def __init__(self, measurement: str, labels: Dict[str, str], value: float, log_timestamp: datetime):
        self.measurement = measurement
        self.labels = labels
        self.value = value
        self.log_timestamp = log_timestamp
    
    def to_dict(self):
        return {
            "measurement": self.measurement,
            "labels": self.labels,
            "value": self.value,
            "log_timestamp": self.log_timestamp
        }

    def __str__(self):
        return f"InfluxPoint(measurement={self.measurement}, labels={self.labels}, value={self.value}, log_timestamp={self.log_timestamp})"


# def push_metric(metric: InfluxPoint, connection: InfluxDBClient):    
#     connection.write_points([metric.to_dict()])
