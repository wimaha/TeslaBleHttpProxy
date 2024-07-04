package control

type Command struct {
	Command string
	Vin     string
	Body    map[string]interface{}
}

type Stack []Command

// IsEmpty: check if stack is empty
func (s *Stack) IsEmpty() bool {
	return len(*s) == 0
}

// Push a new value onto the stack
func (s *Stack) Push(str Command) {
	*s = append(*s, str) // Simply append the new value to the end of the stack
}

// Prepend to Stack
func (s *Stack) Prepend(str Command) {
	*s = append([]Command{str}, *s...)
}

// Remove and return top element of stack. Return true if stack is empty.
func (s *Stack) Pop() (Command, bool) {
	if s.IsEmpty() {
		return Command{}, true
	} else {
		index := 0             // Get the index of the top most element.
		element := (*s)[index] // Index into the slice and obtain the element.
		*s = (*s)[index+1:]    // Remove it from the stack by slicing it off.
		return element, false
	}
}
