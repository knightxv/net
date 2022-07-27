package dchat

type Listener struct {
	Address     string `json:"address"`
	subscribers map[string]struct{}
}

func NewListener(address string) *Listener {
	return &Listener{
		Address:     address,
		subscribers: make(map[string]struct{}),
	}
}

func (l *Listener) AddSubscriber(address string) {
	l.subscribers[address] = struct{}{}
}

func (l *Listener) DelSubscriber(address string) {
	delete(l.subscribers, address)
}

func (l *Listener) GetSubscribers() map[string]struct{} {
	return l.subscribers
}

func (l *Listener) Empty() bool {
	return len(l.subscribers) == 0
}

type ListenerManager struct {
	listeners map[string]*Listener
}

func NewListenerManager() *ListenerManager {
	return &ListenerManager{
		listeners: make(map[string]*Listener),
	}
}

func (lm *ListenerManager) HasListener(address string) bool {
	_, ok := lm.listeners[address]
	return ok
}

func (lm *ListenerManager) GetListener(address string) *Listener {
	listener, ok := lm.listeners[address]
	if !ok {
		listener = &Listener{
			Address:     address,
			subscribers: make(map[string]struct{}),
		}
		lm.listeners[address] = listener
	}

	return listener
}

func (lm *ListenerManager) AddSubscriber(address string, subscriber string) {
	listener := lm.GetListener(address)
	listener.AddSubscriber(subscriber)
}

func (lm *ListenerManager) DelSubscriber(address string, subscriber string) {
	if lm.HasListener(address) {
		listener := lm.GetListener(address)
		listener.DelSubscriber(subscriber)
		if listener.Empty() {
			delete(lm.listeners, address)
		}
	}
}
