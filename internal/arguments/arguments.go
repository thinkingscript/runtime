package arguments

import "sort"

// Argument holds a named argument and its current value.
type Argument struct {
	Name  string
	Value string
}

// Store is a session-scoped container for named arguments.
type Store struct {
	args map[string]string
}

// NewStore creates an empty argument store.
func NewStore() *Store {
	return &Store{args: make(map[string]string)}
}

// Set registers or updates a named argument.
func (s *Store) Set(name, value string) {
	s.args[name] = value
}

// Get looks up an argument by name. Returns "" and false if not found.
func (s *Store) Get(name string) (string, bool) {
	v, ok := s.args[name]
	return v, ok
}

// Snapshot returns all arguments sorted by value length descending.
// Longest values first ensures correct pattern generation when values overlap.
func (s *Store) Snapshot() []Argument {
	out := make([]Argument, 0, len(s.args))
	for name, value := range s.args {
		out = append(out, Argument{Name: name, Value: value})
	}
	sort.Slice(out, func(i, j int) bool {
		return len(out[i].Value) > len(out[j].Value)
	})
	return out
}
