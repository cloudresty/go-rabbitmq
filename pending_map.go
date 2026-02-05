package rabbitmq

import (
	"hash/fnv"
	"sync"
)

// numShards defines the number of shards for the pending message map.
// 32 shards provide a good balance between lock contention reduction and memory overhead.
// With 32 shards, contention is reduced by approximately 32x compared to a single mutex.
const numShards = 32

// shardedPendingMap is a concurrent-safe map for pending messages using sharding.
// It reduces lock contention by splitting data across multiple shards,
// each protected by its own mutex.
type shardedPendingMap struct {
	shards [numShards]pendingShard
}

// pendingShard represents a single shard of the pending map.
type pendingShard struct {
	mu            sync.RWMutex
	byMessageID   map[string]*pendingMessage
	byDeliveryTag map[uint64]*pendingMessage
}

// newShardedPendingMap creates a new sharded pending map.
func newShardedPendingMap() *shardedPendingMap {
	m := &shardedPendingMap{}
	for i := range numShards {
		m.shards[i].byMessageID = make(map[string]*pendingMessage)
		m.shards[i].byDeliveryTag = make(map[uint64]*pendingMessage)
	}
	return m
}

// getShard returns the shard index for a given message ID.
func (m *shardedPendingMap) getShard(messageID string) int {
	h := fnv.New32a()
	_, _ = h.Write([]byte(messageID))
	return int(h.Sum32() % numShards)
}

// getShardByDeliveryTag returns the shard index for a given delivery tag.
// We use a simple modulo since delivery tags are sequential integers.
func (m *shardedPendingMap) getShardByDeliveryTag(deliveryTag uint64) int {
	return int(deliveryTag % numShards)
}

// Store stores a pending message in the appropriate shards.
// Returns true if stored successfully, false if messageID already exists.
func (m *shardedPendingMap) Store(messageID string, deliveryTag uint64, pending *pendingMessage) bool {
	msgShard := m.getShard(messageID)
	tagShard := m.getShardByDeliveryTag(deliveryTag)

	// Lock both shards if different, always in order to avoid deadlock
	if msgShard == tagShard {
		m.shards[msgShard].mu.Lock()
		defer m.shards[msgShard].mu.Unlock()

		if _, exists := m.shards[msgShard].byMessageID[messageID]; exists {
			return false
		}
		m.shards[msgShard].byMessageID[messageID] = pending
		m.shards[msgShard].byDeliveryTag[deliveryTag] = pending
	} else {
		// Lock in consistent order to prevent deadlock
		first, second := msgShard, tagShard
		if first > second {
			first, second = second, first
		}
		m.shards[first].mu.Lock()
		m.shards[second].mu.Lock()
		defer m.shards[first].mu.Unlock()
		defer m.shards[second].mu.Unlock()

		if _, exists := m.shards[msgShard].byMessageID[messageID]; exists {
			return false
		}
		m.shards[msgShard].byMessageID[messageID] = pending
		m.shards[tagShard].byDeliveryTag[deliveryTag] = pending
	}
	return true
}

// LoadByMessageID retrieves a pending message by message ID.
func (m *shardedPendingMap) LoadByMessageID(messageID string) (*pendingMessage, bool) {
	shard := m.getShard(messageID)
	m.shards[shard].mu.RLock()
	pending, exists := m.shards[shard].byMessageID[messageID]
	m.shards[shard].mu.RUnlock()
	return pending, exists
}

// LoadByDeliveryTag retrieves a pending message by delivery tag.
func (m *shardedPendingMap) LoadByDeliveryTag(deliveryTag uint64) (*pendingMessage, bool) {
	shard := m.getShardByDeliveryTag(deliveryTag)
	m.shards[shard].mu.RLock()
	pending, exists := m.shards[shard].byDeliveryTag[deliveryTag]
	m.shards[shard].mu.RUnlock()
	return pending, exists
}

// Delete removes a pending message from both maps.
func (m *shardedPendingMap) Delete(messageID string, deliveryTag uint64) {
	msgShard := m.getShard(messageID)
	tagShard := m.getShardByDeliveryTag(deliveryTag)

	if msgShard == tagShard {
		m.shards[msgShard].mu.Lock()
		delete(m.shards[msgShard].byMessageID, messageID)
		delete(m.shards[msgShard].byDeliveryTag, deliveryTag)
		m.shards[msgShard].mu.Unlock()
	} else {
		first, second := msgShard, tagShard
		if first > second {
			first, second = second, first
		}
		m.shards[first].mu.Lock()
		m.shards[second].mu.Lock()
		delete(m.shards[msgShard].byMessageID, messageID)
		delete(m.shards[tagShard].byDeliveryTag, deliveryTag)
		m.shards[second].mu.Unlock()
		m.shards[first].mu.Unlock()
	}
}

// Count returns the total number of pending messages.
func (m *shardedPendingMap) Count() int64 {
	var count int64
	for i := range numShards {
		m.shards[i].mu.RLock()
		count += int64(len(m.shards[i].byMessageID))
		m.shards[i].mu.RUnlock()
	}
	return count
}

// Range iterates over all pending messages and calls f for each.
// If f returns false, iteration stops.
func (m *shardedPendingMap) Range(f func(messageID string, pending *pendingMessage) bool) {
	for i := range numShards {
		m.shards[i].mu.RLock()
		for msgID, pending := range m.shards[i].byMessageID {
			if !f(msgID, pending) {
				m.shards[i].mu.RUnlock()
				return
			}
		}
		m.shards[i].mu.RUnlock()
	}
}
