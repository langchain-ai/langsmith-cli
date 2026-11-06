package ui

import (
	"slices"
	"unicode"
)

// stringCursor is a single-line text being edited and the position of its
// cursor, as cliclack's `StringCursor` is.
type stringCursor struct {
	value  []rune
	cursor int
}

func newStringCursor(text string) *stringCursor {
	return &stringCursor{value: []rune(text)}
}

func (c *stringCursor) String() string {
	return string(c.value)
}

func (c *stringCursor) isEmpty() bool {
	return len(c.value) == 0
}

// insert inserts r at the cursor.
func (c *stringCursor) insert(r rune) {
	c.value = slices.Insert(c.value, c.cursor, r)
	c.cursor++
}

func (c *stringCursor) moveLeft() {
	if c.cursor > 0 {
		c.cursor--
	}
}

func (c *stringCursor) moveRight() {
	if c.cursor < len(c.value) {
		c.cursor++
	}
}

func (c *stringCursor) moveHome() {
	c.cursor = 0
}

func (c *stringCursor) moveEnd() {
	c.cursor = len(c.value)
}

// moveLeftByWord moves the cursor to the start of the word it is in or after.
func (c *stringCursor) moveLeftByWord() {
	jumps := c.wordJumps()
	index := 0
	for index < len(jumps) && jumps[index] < c.cursor {
		index++
	}
	c.cursor = jumps[max(index-1, 0)]
}

// moveRightByWord moves the cursor to the start of the next word.
func (c *stringCursor) moveRightByWord() {
	jumps := c.wordJumps()
	for _, jump := range jumps {
		if jump > c.cursor {
			c.cursor = jump
			return
		}
	}
	c.cursor = jumps[len(jumps)-1]
}

// deleteLeft deletes the character left of the cursor.
func (c *stringCursor) deleteLeft() {
	if c.cursor > 0 {
		c.value = slices.Delete(c.value, c.cursor-1, c.cursor)
		c.cursor--
	}
}

// deleteRight deletes the character under the cursor.
func (c *stringCursor) deleteRight() {
	if c.cursor < len(c.value) {
		c.value = slices.Delete(c.value, c.cursor, c.cursor+1)
	}
}

// deleteWordToTheLeft deletes back to the start of the word the cursor is in
// or after.
func (c *stringCursor) deleteWordToTheLeft() {
	end := c.cursor
	c.moveLeftByWord()
	c.value = slices.Delete(c.value, c.cursor, end)
}

// split returns the text left of the cursor, the character under it (a space
// at the end), and the text right of it.
func (c *stringCursor) split() (left, at, right string) {
	left, at = string(c.value[:c.cursor]), " "
	if c.cursor < len(c.value) {
		at = string(c.value[c.cursor])
		right = string(c.value[c.cursor+1:])
	}
	return left, at, right
}

// masked is a copy of c with every character replaced by mask.
func (c *stringCursor) masked(mask rune) *stringCursor {
	value := make([]rune, len(c.value))
	for index := range value {
		value[index] = mask
	}
	return &stringCursor{value: value, cursor: c.cursor}
}

// wordJumps are the start of the text, the first character of each word, and
// the end of the text, in order.
func (c *stringCursor) wordJumps() []int {
	jumps := []int{0}
	inWord := false
	for index, r := range c.value {
		if unicode.IsSpace(r) {
			inWord = false
		} else if !inWord {
			jumps = append(jumps, index)
			inWord = true
		}
	}
	return append(jumps, len(c.value))
}
