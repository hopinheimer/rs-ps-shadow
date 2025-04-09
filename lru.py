from collections import OrderedDict

class LRUCache:
    def __init__(self, capacity: int):
        self.cache = OrderedDict()
        self.capacity = capacity

    def __getitem__(self, key):
        if key not in self.cache:
            raise KeyError(key)
        # Move the key to the end to mark it as recently used
        self.cache.move_to_end(key)
        return self.cache[key]

    def __setitem__(self, key, value):
        if key in self.cache:
            # Update value and mark as recently used
            self.cache.move_to_end(key)
        self.cache[key] = value
        if len(self.cache) > self.capacity:
            # Evict the least recently used item
            self.cache.popitem(last=False)

    def __delitem__(self, key):
        del self.cache[key]

    def __contains__(self, key):
        return key in self.cache

    def __len__(self):
        return len(self.cache)

    def __iter__(self):
        return iter(self.cache)

    def items(self):
        return self.cache.items()

    def keys(self):
        return self.cache.keys()

    def values(self):
        return self.cache.values()

    def get(self, key, default=None):
        try:
            return self[key]
        except KeyError:
            return default

    def clear(self):
        self.cache.clear()

    def __repr__(self):
        return f"{self.__class__.__name__}({dict(self.cache)})"

