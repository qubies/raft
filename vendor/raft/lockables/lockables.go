package lockables

// lockables are threadsafe primatives with some basic functionality.

import "sync"

//Counter is a simple threadsafe incrementable counter
type Counter struct {
	sync.RWMutex
	val int
}

//Inc increments the counter
func (C *Counter) Inc() {
	C.Lock()
	defer C.Unlock()
	C.val++
}

//Get returns the value of the coutner
func (C *Counter) Get() int {
	C.RLock()
	defer C.RUnlock()
	return C.val
}

//Bool is a threadsafe boolean
type Bool struct {
	sync.RWMutex
	val bool
}

//Set sets the value of the bool
func (B *Bool) Set(val bool) {
	B.Lock()
	defer B.Unlock()
	B.val = val
}

//Get returns the state of the bool
func (B *Bool) Get() bool {
	B.RLock()
	defer B.RUnlock()
	return B.val
}

type StrMap struct {
	TheMap map[string]string
	mux    sync.RWMutex
}

func NewStrMap() *StrMap {
	newMap := new(StrMap)
	newMap.TheMap = make(map[string]string)
	return newMap
}
func (m *StrMap) Add(k, v string) {
	m.mux.Lock()
	defer m.mux.Unlock()
	m.TheMap[k] = v
}

func (m *StrMap) Exists(k string) bool {
	m.mux.RLock()
	defer m.mux.RUnlock()
	_, ok := m.TheMap[k]
	return ok
}

func (m *StrMap) Get() map[string]string {
	newMap := make(map[string]string)
	m.mux.RLock()
	defer m.mux.RUnlock()
	for k, v := range m.TheMap {
		newMap[k] = v
	}
	return newMap
}

func (m *StrMap) Lock() {
	m.mux.Lock()
}

func (m *StrMap) Unlock() {
	m.mux.Unlock()
}
