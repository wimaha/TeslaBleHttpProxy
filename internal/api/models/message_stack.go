package models

import "sync"

type MessageType int

// Declare related constants for each weekday starting with index 1
const (
	Error   MessageType = iota + 1 // EnumIndex = 1
	Success                        // EnumIndex = 2
	Info                           // EnumIndex = 3
)

// String - Creating common behavior - give the type a String function
func (w MessageType) String() string {
	return [...]string{"Error", "Success", "Info"}[w-1]
}

// EnumIndex - Creating common behavior - give the type a EnumIndex function
func (w MessageType) EnumIndex() int {
	return int(w)
}

type Message struct {
	Title   string
	Message string
	Type    MessageType
}

// MessageStack is safe for concurrent use; HTTP handlers push and pop in parallel.
type MessageStack struct {
	mu       sync.Mutex
	messages []Message
}

// IsEmpty: check if stack is empty
func (s *MessageStack) IsEmpty() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.messages) == 0
}

// Push a new value onto the stack
func (s *MessageStack) Push(str Message) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages = append(s.messages, str) // Simply append the new value to the end of the stack
}

// Prepend to Stack
func (s *MessageStack) Prepend(str Message) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages = append([]Message{str}, s.messages...)
}

// Remove and return top element of stack. Return true if stack is empty.
func (s *MessageStack) Pop() (Message, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.messages) == 0 {
		return Message{}, true
	}
	element := s.messages[0]
	s.messages = s.messages[1:]
	return element, false
}

func (s *MessageStack) PopAll() []Message {
	s.mu.Lock()
	defer s.mu.Unlock()
	all := s.messages
	s.messages = nil
	return all
}

var MainMessageStack MessageStack
