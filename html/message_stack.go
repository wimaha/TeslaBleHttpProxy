package html

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

type MessageStack []Message

// IsEmpty: check if stack is empty
func (s *MessageStack) IsEmpty() bool {
	return len(*s) == 0
}

// Push a new value onto the stack
func (s *MessageStack) Push(str Message) {
	*s = append(*s, str) // Simply append the new value to the end of the stack
}

// Prepend to Stack
func (s *MessageStack) Prepend(str Message) {
	*s = append([]Message{str}, *s...)
}

// Remove and return top element of stack. Return true if stack is empty.
func (s *MessageStack) Pop() (Message, bool) {
	if s.IsEmpty() {
		return Message{}, true
	} else {
		index := 0             // Get the index of the top most element.
		element := (*s)[index] // Index into the slice and obtain the element.
		*s = (*s)[index+1:]    // Remove it from the stack by slicing it off.
		return element, false
	}
}

func (s *MessageStack) PopAll() []Message {
	var all []Message
	for _, message := range *s {
		all = append(all, message)
	}
	*s = (*s)[:0]
	return all
}

var MainMessageStack MessageStack
