package codegen

import "strings"

// Code is a simple indenting line builder
type Code struct {
	indent int
	tab    string
	lines  []string
}

func NewCode(tab string) *Code {
	return &Code{tab: tab}
}

// L appends a line at the current indentation
func (c *Code) L(line string) *Code {
	c.lines = append(c.lines, strings.Repeat(c.tab, c.indent)+line)
	return c
}

// Ind changes the indentation
func (c *Code) Ind(delta int) *Code {
	c.indent += delta
	return c
}

func (c *Code) String() string {
	return strings.Join(c.lines, "\n")
}
