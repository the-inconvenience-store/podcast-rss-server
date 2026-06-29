package storage

import (
	"bytes"
	"fmt"
	"io"
	"sync"
	"time"
)

type ObjectInfo struct {
	Size        int64
	ContentType string
	ModTime     time.Time
}

type ReadSeekCloser interface {
	io.Reader
	io.Seeker
	io.Closer
}

type Storage interface {
	Put(key string, body io.Reader, info ObjectInfo) error
	Get(key string) (ReadSeekCloser, error)
	Stat(key string) (ObjectInfo, error)
	Delete(key string) error
}

type Memory struct {
	mu      sync.RWMutex
	objects map[string]memoryObject
}

type memoryObject struct {
	data []byte
	info ObjectInfo
}

func NewMemory() *Memory {
	return &Memory{objects: make(map[string]memoryObject)}
}

func (m *Memory) Put(key string, body io.Reader, info ObjectInfo) error {
	data, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	if info.Size == 0 {
		info.Size = int64(len(data))
	}
	if info.ModTime.IsZero() {
		info.ModTime = time.Now().UTC()
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.objects[key] = memoryObject{data: data, info: info}
	return nil
}

func (m *Memory) Get(key string) (ReadSeekCloser, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	obj, ok := m.objects[key]
	if !ok {
		return nil, fmt.Errorf("object not found")
	}
	return readSeekCloser{Reader: bytes.NewReader(obj.data)}, nil
}

func (m *Memory) Stat(key string) (ObjectInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	obj, ok := m.objects[key]
	if !ok {
		return ObjectInfo{}, fmt.Errorf("object not found")
	}
	return obj.info, nil
}

func (m *Memory) Delete(key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.objects, key)
	return nil
}

type readSeekCloser struct {
	*bytes.Reader
}

func (readSeekCloser) Close() error {
	return nil
}
