package compile

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// The Rust CLI edits `pyproject.toml` with `toml_edit` (0.25), which keeps the
// author's formatting and renders what it adds in its own default style. The
// edited manifests must match it byte for byte, so this file reproduces the
// part of `toml_edit` compile uses: its document model (keys with their decor,
// tables with their header positions, arrays and inline tables with their
// separators), the way its parser attaches whitespace and comments to that
// model, and the way `DocumentMut`'s `Display` renders it back.
//
// Documents are validated with go-toml first, so the parser below only has to
// find the structure of valid TOML.

// tomlDecor is the whitespace and comments around a key, value or header. A
// nil side renders the caller's default, as `toml_edit`'s unset decor does.
type tomlDecor struct {
	prefix, suffix *string
}

func explicitDecor(prefix, suffix string) tomlDecor {
	return tomlDecor{prefix: &prefix, suffix: &suffix}
}

func (d tomlDecor) writePrefix(out *strings.Builder, fallback string) {
	if d.prefix != nil {
		writeRaw(out, *d.prefix)
	} else {
		out.WriteString(fallback)
	}
}

func (d tomlDecor) writeSuffix(out *strings.Builder, fallback string) {
	if d.suffix != nil {
		writeRaw(out, *d.suffix)
	} else {
		out.WriteString(fallback)
	}
}

// writeRaw writes whitespace and comments kept from the source. Like
// `toml_edit`'s `RawString::encode`, it drops carriage returns, so CRLF line
// endings come out as LF.
func writeRaw(out *strings.Builder, raw string) {
	out.WriteString(strings.ReplaceAll(raw, "\r", ""))
}

// The default decors of `toml_edit`'s encoder.
var (
	defaultKeyDecor          = [2]string{"", " "}
	defaultTableDecor        = [2]string{"\n", ""}
	defaultKeyPathDecor      = [2]string{"", ""}
	defaultInlineKeyDecor    = [2]string{" ", " "}
	defaultValueDecor        = [2]string{" ", ""}
	defaultTrailingValue     = [2]string{" ", " "}
	defaultLeadingValueDecor = [2]string{"", ""}
)

// tomlKey is a key with its source spelling and decor: `leaf` around the whole
// (possibly dotted) key, `dotted` around this part of a dotted key.
type tomlKey struct {
	name   string
	repr   *string
	leaf   tomlDecor
	dotted tomlDecor
}

func newTomlKey(name string) *tomlKey {
	return &tomlKey{name: name}
}

func (k *tomlKey) display() string {
	if k.repr != nil {
		return *k.repr
	}
	return defaultTomlKeyRepr(k.name)
}

// clone copies k, so the copy's decor can change independently.
func (k *tomlKey) clone() *tomlKey {
	copied := *k
	return &copied
}

// tomlValue is a scalar, an array or an inline table.
type tomlValue interface {
	valueDecor() *tomlDecor
}

// tomlScalar is a string, number, boolean or date-time, kept as written.
type tomlScalar struct {
	decor tomlDecor
	repr  string
	// text is the decoded string of a string scalar.
	text     string
	isString bool
}

func (s *tomlScalar) valueDecor() *tomlDecor { return &s.decor }

// newTomlString is `toml_edit::Value::from(string)`.
func newTomlString(text string) *tomlScalar {
	return &tomlScalar{repr: defaultTomlStringRepr(text), text: text, isString: true}
}

type tomlArray struct {
	decor         tomlDecor
	values        []tomlValue
	trailingComma bool
	trailing      string
}

func (a *tomlArray) valueDecor() *tomlDecor { return &a.decor }

// strings returns the string elements, skipping the others.
func (a *tomlArray) strings() []string {
	var out []string
	for _, value := range a.values {
		if text, ok := tomlString(value); ok {
			out = append(out, text)
		}
	}
	return out
}

func (a *tomlArray) remove(index int) {
	a.values = slices.Delete(a.values, index, index+1)
}

type tomlInlineTable struct {
	decor         tomlDecor
	items         tomlItems
	implicit      bool
	dotted        bool
	trailingComma bool
	trailing      string
}

func (t *tomlInlineTable) valueDecor() *tomlDecor { return &t.decor }

// tomlTable is a standard table: a header (unless implicit or dotted) and the
// key/values under it.
type tomlTable struct {
	decor    tomlDecor
	implicit bool
	dotted   bool
	// position is the header's position in the source document; nil for a
	// table compile created.
	position *int
	items    tomlItems
}

type tomlArrayOfTables struct {
	tables []*tomlTable
}

// tomlItem is a tomlValue, *tomlTable, *tomlArrayOfTables or tomlNone.
type tomlItem any

// tomlNone is `Item::None`: the placeholder `toml_edit`'s `IndexMut` inserts
// for a key it is asked for. Lookups skip it and it renders as nothing, but it
// keeps its key's place.
type tomlNone struct{}

func isTomlNone(item tomlItem) bool {
	_, ok := item.(tomlNone)
	return ok
}

// tomlItems is an insertion-ordered map of keys to items (`toml_edit`'s
// `IndexMap<Key, Item>`), compared by decoded key.
type tomlItems struct {
	entries []tomlEntry
}

type tomlEntry struct {
	key  *tomlKey
	item tomlItem
}

func (m *tomlItems) index(name string) int {
	return slices.IndexFunc(m.entries, func(entry tomlEntry) bool { return entry.key.name == name })
}

// get is `Table::get`: the item of name, unless it is a placeholder.
func (m *tomlItems) get(name string) (tomlItem, bool) {
	if i := m.index(name); i >= 0 && !isTomlNone(m.entries[i].item) {
		return m.entries[i].item, true
	}
	return nil, false
}

// slot is `table.entry(name).or_insert(Item::None)`: the place of name's item,
// inserting a placeholder when absent.
func (m *tomlItems) slot(name string) *tomlItem {
	i := m.index(name)
	if i < 0 {
		m.entries = append(m.entries, tomlEntry{key: newTomlKey(name), item: tomlNone{}})
		i = len(m.entries) - 1
	}
	return &m.entries[i].item
}

// put inserts item under key, replacing the item of an existing key in place
// (keeping that key) and appending otherwise.
func (m *tomlItems) put(key *tomlKey, item tomlItem) {
	if i := m.index(key.name); i >= 0 {
		m.entries[i].item = item
		return
	}
	m.entries = append(m.entries, tomlEntry{key: key, item: item})
}

// remove is `shift_remove`.
func (m *tomlItems) remove(name string) (*tomlKey, tomlItem, bool) {
	i := m.index(name)
	if i < 0 {
		return nil, nil, false
	}
	entry := m.entries[i]
	m.entries = slices.Delete(m.entries, i, i+1)
	return entry.key, entry.item, true
}

// len counts the items that are not placeholders.
func (m *tomlItems) len() int {
	return len(m.names())
}

// tomlTableLike is a standard or inline table (`toml_edit::TableLike`).
type tomlTableLike interface {
	keyNames() []string
	getItem(name string) (tomlItem, bool)
	removeItem(name string) bool
	// insertItem is `TableLike::insert`: an existing key loses its formatting.
	insertItem(name string, item tomlItem)
}

func (t *tomlTable) keyNames() []string { return t.items.names() }
func (t *tomlTable) getItem(name string) (tomlItem, bool) {
	return t.items.get(name)
}
func (t *tomlTable) removeItem(name string) bool {
	_, _, ok := t.items.remove(name)
	return ok
}
func (t *tomlTable) insertItem(name string, item tomlItem) {
	t.items.insertFormatted(name, item)
}

func (t *tomlInlineTable) keyNames() []string { return t.items.names() }
func (t *tomlInlineTable) getItem(name string) (tomlItem, bool) {
	return t.items.get(name)
}
func (t *tomlInlineTable) removeItem(name string) bool {
	_, _, ok := t.items.remove(name)
	return ok
}
func (t *tomlInlineTable) insertItem(name string, item tomlItem) {
	t.items.insertFormatted(name, item)
}

// names are the keys of the items that are not placeholders, in order.
func (m *tomlItems) names() []string {
	var names []string
	for _, entry := range m.entries {
		if !isTomlNone(entry.item) {
			names = append(names, entry.key.name)
		}
	}
	return names
}

// insertFormatted is `Table::insert`: the key of an existing entry is reset to
// its default formatting.
func (m *tomlItems) insertFormatted(name string, item tomlItem) {
	if i := m.index(name); i >= 0 {
		m.entries[i].key.repr = nil
		m.entries[i].key.leaf = tomlDecor{}
		m.entries[i].key.dotted = tomlDecor{}
		m.entries[i].item = item
		return
	}
	m.entries = append(m.entries, tomlEntry{key: newTomlKey(name), item: item})
}

// entry returns the item of name, appending make() under a new key when absent
// (`table.entry(name).or_insert_with(make)`). A placeholder counts as present.
func (m *tomlItems) entry(name string, make func() tomlItem) tomlItem {
	if i := m.index(name); i >= 0 {
		return m.entries[i].item
	}
	item := make()
	m.entries = append(m.entries, tomlEntry{key: newTomlKey(name), item: item})
	return item
}

// asTableLike returns item as a standard or inline table.
func asTableLike(item tomlItem) (tomlTableLike, bool) {
	switch item := item.(type) {
	case *tomlTable:
		return item, true
	case *tomlInlineTable:
		return item, true
	}
	return nil, false
}

// tomlGet is `Item::get`: the item under name of a table-like item.
func tomlGet(item tomlItem, name string) (tomlItem, bool) {
	table, ok := asTableLike(item)
	if !ok {
		return nil, false
	}
	return table.getItem(name)
}

// tomlPath follows names through table-like items.
func tomlPath(item tomlItem, names ...string) (tomlItem, bool) {
	for _, name := range names {
		next, ok := tomlGet(item, name)
		if !ok {
			return nil, false
		}
		item = next
	}
	return item, true
}

// tomlGetMutPath is `root.get_mut(first).and_then(|item| item.get_mut(next))…`.
//
// The root lookup only reads, but each `Item::get_mut` after it inserts the
// key it is asked for as a placeholder, and turns a placeholder it is asked
// into into an inline table, as `toml_edit`'s `IndexMut` does. Those inserts
// show in the rendered document (`uv = {}`), so they are reproduced.
func tomlGetMutPath(root *tomlTable, first string, rest ...string) *tomlItem {
	i := root.items.index(first)
	if i < 0 || isTomlNone(root.items.entries[i].item) {
		return nil
	}
	slot := &root.items.entries[i].item
	for _, name := range rest {
		if isTomlNone(*slot) {
			inline := &tomlInlineTable{}
			inline.items.entries = []tomlEntry{{key: newTomlKey(name), item: tomlNone{}}}
			*slot = inline
		}
		switch item := (*slot).(type) {
		case *tomlTable:
			slot = item.items.slot(name)
		case *tomlInlineTable:
			slot = item.items.slot(name)
		default:
			return nil
		}
	}
	return slot
}

// tomlString returns item's decoded text when it is a string value.
func tomlString(item tomlItem) (string, bool) {
	scalar, ok := item.(*tomlScalar)
	if !ok || !scalar.isString {
		return "", false
	}
	return scalar.text, true
}

// tomlDocument is a parsed `pyproject.toml` (`toml_edit::DocumentMut`).
type tomlDocument struct {
	root     *tomlTable
	trailing string
}

// parseTomlDocument parses text the way `str::parse::<DocumentMut>` does.
func parseTomlDocument(text string) (*tomlDocument, error) {
	var check map[string]any
	if err := toml.Unmarshal([]byte(text), &check); err != nil {
		return nil, err
	}
	p := &tomlParser{src: text, trailingStart: -1}
	return p.document(), nil
}

// String renders the document like `DocumentMut`'s `Display`.
func (d *tomlDocument) String() string {
	type visited struct {
		position int
		table    *tomlTable
		path     []*tomlKey
		isArray  bool
	}
	var tables []visited
	lastPosition := 0
	var visit func(table *tomlTable, path []*tomlKey, isArray bool)
	visit = func(table *tomlTable, path []*tomlKey, isArray bool) {
		if !table.dotted {
			if table.position != nil {
				lastPosition = *table.position
			}
			tables = append(tables, visited{lastPosition, table, slices.Clone(path), isArray})
		}
		for _, entry := range table.items.entries {
			switch item := entry.item.(type) {
			case *tomlTable:
				visit(item, append(path, entry.key), false)
			case *tomlArrayOfTables:
				for _, child := range item.tables {
					visit(child, append(path, entry.key), true)
				}
			}
		}
	}
	visit(d.root, nil, false)
	slices.SortStableFunc(tables, func(a, b visited) int {
		aNested, bNested := len(a.path) > 0, len(b.path) > 0
		if aNested != bNested {
			if aNested {
				return 1
			}
			return -1
		}
		return a.position - b.position
	})

	var out strings.Builder
	d.root.decor.writePrefix(&out, "")
	firstTable := true
	for _, table := range tables {
		writeTomlTable(&out, table.table, table.path, table.isArray, &firstTable)
	}
	d.root.decor.writeSuffix(&out, "")
	writeRaw(&out, d.trailing)
	return out.String()
}

func writeTomlTable(out *strings.Builder, table *tomlTable, path []*tomlKey, isArray bool, firstTable *bool) {
	children := tableValues(&table.items, nil)
	visible := !(table.implicit && len(children) == 0)
	header := func(open, close string) {
		fallback := defaultTableDecor
		if *firstTable {
			*firstTable = false
			fallback = [2]string{"", defaultTableDecor[1]}
		}
		leaf := path[len(path)-1].leaf
		keyDecor := leaf
		if prefix := leaf.prefix; prefix != nil && strings.Contains(*prefix, "\n") {
			leaf.writePrefix(out, defaultKeyPathDecor[0])
			keyDecor = tomlDecor{}
		}
		table.decor.writePrefix(out, fallback[0])
		out.WriteString(open)
		writeTomlKeyPath(out, path, defaultKeyPathDecor, keyDecor)
		out.WriteString(close)
		table.decor.writeSuffix(out, fallback[1])
		out.WriteString("\n")
	}
	switch {
	case len(path) == 0:
		if len(children) > 0 {
			*firstTable = false
		}
	case isArray:
		header("[[", "]]")
	case visible:
		header("[", "]")
	}
	for _, child := range children {
		writeTomlKeyPath(out, child.path, defaultKeyDecor, child.path[len(child.path)-1].leaf)
		out.WriteString("=")
		writeTomlValue(out, child.value, defaultValueDecor)
		out.WriteString("\n")
	}
}

type tomlKeyValue struct {
	path  []*tomlKey
	value tomlValue
}

// tableValues lists the key/values that are visually children of items:
// dotted tables and dotted inline tables are flattened into dotted keys.
func tableValues(items *tomlItems, path []*tomlKey) []tomlKeyValue {
	var values []tomlKeyValue
	for _, entry := range items.entries {
		keys := append(slices.Clone(path), entry.key)
		switch item := entry.item.(type) {
		case *tomlTable:
			if item.dotted {
				values = append(values, tableValues(&item.items, keys)...)
			}
		case *tomlInlineTable:
			if item.dotted {
				values = append(values, tableValues(&item.items, keys)...)
			} else {
				values = append(values, tomlKeyValue{keys, item})
			}
		case tomlValue:
			values = append(values, tomlKeyValue{keys, item})
		}
	}
	return values
}

func writeTomlKeyPath(out *strings.Builder, keys []*tomlKey, fallback [2]string, leaf tomlDecor) {
	for i, key := range keys {
		first, last := i == 0, i == len(keys)-1
		if first {
			leaf.writePrefix(out, fallback[0])
		} else {
			out.WriteString(".")
			key.dotted.writePrefix(out, defaultKeyPathDecor[0])
		}
		out.WriteString(key.display())
		if last {
			leaf.writeSuffix(out, fallback[1])
		} else {
			key.dotted.writeSuffix(out, defaultKeyPathDecor[1])
		}
	}
}

func writeTomlValue(out *strings.Builder, value tomlValue, fallback [2]string) {
	switch value := value.(type) {
	case *tomlScalar:
		value.decor.writePrefix(out, fallback[0])
		out.WriteString(value.repr)
		value.decor.writeSuffix(out, fallback[1])
	case *tomlArray:
		value.decor.writePrefix(out, fallback[0])
		out.WriteString("[")
		for i, element := range value.values {
			inner := defaultLeadingValueDecor
			if i > 0 {
				inner = defaultValueDecor
				out.WriteString(",")
			}
			writeTomlValue(out, element, inner)
		}
		if value.trailingComma && len(value.values) > 0 {
			out.WriteString(",")
		}
		writeRaw(out, value.trailing)
		out.WriteString("]")
		value.decor.writeSuffix(out, fallback[1])
	case *tomlInlineTable:
		value.decor.writePrefix(out, fallback[0])
		out.WriteString("{")
		children := tableValues(&value.items, nil)
		for i, child := range children {
			if i > 0 {
				out.WriteString(",")
			}
			inner := defaultValueDecor
			if i == len(children)-1 {
				inner = defaultTrailingValue
			}
			writeTomlKeyPath(out, child.path, defaultInlineKeyDecor, child.path[len(child.path)-1].leaf)
			out.WriteString("=")
			writeTomlValue(out, child.value, inner)
		}
		if value.trailingComma && value.items.len() > 0 {
			out.WriteString(",")
		}
		writeRaw(out, value.trailing)
		out.WriteString("}")
		value.decor.writeSuffix(out, fallback[1])
	}
}

// tomlParser builds the document model from valid TOML, attaching whitespace
// and comments where `toml_edit`'s parser does.
type tomlParser struct {
	src string
	pos int

	// trailingStart..trailingEnd is the whitespace, comments and newlines
	// seen since the last expression; -1 when there are none.
	trailingStart, trailingEnd int

	root         *tomlTable
	current      *tomlTable
	header       *tomlHeader
	nextPosition int
}

type tomlHeader struct {
	path    []*tomlKey
	key     *tomlKey
	isArray bool
}

func (p *tomlParser) document() *tomlDocument {
	p.root = &tomlTable{}
	p.current = &tomlTable{}
	for p.pos < len(p.src) {
		switch c := p.src[p.pos]; {
		case c == ' ' || c == '\t':
			p.captureTrailing(p.whitespace())
		case c == '#':
			p.captureTrailing(p.comment())
		case c == '\n' || c == '\r':
			p.captureTrailing(p.newline())
		case c == '[':
			p.finishTable()
			prefix := p.takeTrailing()
			header := p.tableHeader()
			suffix := p.wsCommentNewline()
			p.startTable(header, explicitDecor(prefix, suffix))
		default:
			p.keyValue()
		}
	}
	p.finishTable()
	return &tomlDocument{root: p.root, trailing: p.takeTrailing()}
}

func (p *tomlParser) captureTrailing(start, end int) {
	if p.trailingStart < 0 {
		p.trailingStart = start
	}
	p.trailingEnd = end
}

func (p *tomlParser) takeTrailing() string {
	if p.trailingStart < 0 {
		return ""
	}
	text := p.src[p.trailingStart:p.trailingEnd]
	p.trailingStart = -1
	return text
}

func (p *tomlParser) peek() byte {
	if p.pos < len(p.src) {
		return p.src[p.pos]
	}
	return 0
}

// whitespace consumes a run of spaces and tabs.
func (p *tomlParser) whitespace() (int, int) {
	start := p.pos
	for p.pos < len(p.src) && (p.src[p.pos] == ' ' || p.src[p.pos] == '\t') {
		p.pos++
	}
	return start, p.pos
}

// optionalWhitespace consumes a run of whitespace, returning its text.
func (p *tomlParser) optionalWhitespace() (string, bool) {
	start, end := p.whitespace()
	return p.src[start:end], end > start
}

func (p *tomlParser) comment() (int, int) {
	start := p.pos
	for p.pos < len(p.src) && p.src[p.pos] != '\n' && !strings.HasPrefix(p.src[p.pos:], "\r\n") {
		p.pos++
	}
	return start, p.pos
}

func (p *tomlParser) newline() (int, int) {
	start := p.pos
	if strings.HasPrefix(p.src[p.pos:], "\r\n") {
		p.pos += 2
	} else {
		p.pos++
	}
	return start, p.pos
}

// wsCommentNewline consumes whitespace and a comment through the end of the
// line, returning them without the newline.
func (p *tomlParser) wsCommentNewline() string {
	start := p.pos
	end := p.pos
	for p.pos < len(p.src) {
		switch c := p.src[p.pos]; {
		case c == ' ' || c == '\t':
			_, end = p.whitespace()
		case c == '#':
			_, end = p.comment()
		case c == '\n' || c == '\r':
			p.newline()
			return p.src[start:end]
		default:
			return p.src[start:end]
		}
	}
	return p.src[start:end]
}

// tableHeader parses `[a.b]` or `[[a.b]]`.
func (p *tomlParser) tableHeader() tomlHeader {
	isArray := strings.HasPrefix(p.src[p.pos:], "[[")
	if isArray {
		p.pos += 2
	} else {
		p.pos++
	}
	prefix, _ := p.optionalWhitespace()
	path, key := p.keyPath()
	suffix, _ := p.optionalWhitespace()
	key.leaf = explicitDecor(prefix, suffix)
	if isArray {
		p.pos += 2
	} else {
		p.pos++
	}
	return tomlHeader{path: path, key: key, isArray: isArray}
}

// keyPath parses a simple or dotted key: the parent keys and the last key,
// each with the whitespace around it in its dotted decor.
func (p *tomlParser) keyPath() ([]*tomlKey, *tomlKey) {
	var path []*tomlKey
	prefix := ""
	for {
		key := p.simpleKey()
		suffix := ""
		// More keys follow when the next token, after optional
		// whitespace, is a dot.
		save := p.pos
		ws, _ := p.optionalWhitespace()
		if p.peek() != '.' {
			p.pos = save
			key.dotted = explicitDecor(prefix, suffix)
			return path, key
		}
		suffix = ws
		key.dotted = explicitDecor(prefix, suffix)
		path = append(path, key)
		p.pos++ // `.`
		prefix, _ = p.optionalWhitespace()
	}
}

func (p *tomlParser) simpleKey() *tomlKey {
	start := p.pos
	var name string
	switch p.peek() {
	case '"', '\'':
		name = p.stringLiteral()
	default:
		for p.pos < len(p.src) && isBareKeyByte(p.src[p.pos]) {
			p.pos++
		}
		name = p.src[start:p.pos]
	}
	repr := p.src[start:p.pos]
	return &tomlKey{name: name, repr: &repr}
}

func isBareKeyByte(c byte) bool {
	return 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' || '0' <= c && c <= '9' || c == '-' || c == '_'
}

func (p *tomlParser) keyValue() {
	keyPrefix := p.takeTrailing()
	path, key := p.keyPath()
	keySuffix, _ := p.optionalWhitespace()
	key.leaf = explicitDecor(keyPrefix, keySuffix)
	p.pos++ // `=`
	valuePrefix, _ := p.optionalWhitespace()
	value := p.value()
	valueSuffix := p.wsCommentNewline()
	*value.valueDecor() = explicitDecor(valuePrefix, valueSuffix)

	parent := descendTablePath(p.current, path, len(path) > 0)
	parent.items.put(key, value)
}

func (p *tomlParser) finishTable() {
	table := p.current
	p.current = &tomlTable{}
	if p.header == nil {
		p.root = table
		return
	}
	header := p.header
	p.header = nil
	parent := descendTablePath(p.root, header.path, false)
	if header.isArray {
		item, ok := parent.items.get(header.key.name)
		if !ok {
			item = &tomlArrayOfTables{}
			parent.items.put(header.key, item)
		}
		array := item.(*tomlArrayOfTables)
		array.tables = append(array.tables, table)
		return
	}
	parent.items.put(header.key, table)
}

func (p *tomlParser) startTable(header tomlHeader, decor tomlDecor) {
	if !header.isArray {
		// A table an earlier header implied keeps its children.
		parent := descendTablePath(p.root, header.path, false)
		if _, item, ok := parent.items.remove(header.key.name); ok {
			if table, ok := item.(*tomlTable); ok {
				p.current = table
			}
		}
	}
	p.nextPosition++
	position := p.nextPosition
	p.current.decor = decor
	p.current.implicit = false
	p.current.dotted = false
	p.current.position = &position
	p.header = &header
}

// descendTablePath returns the table at path under table, creating implicit
// tables (dotted ones for dotted keys) on the way.
func descendTablePath(table *tomlTable, path []*tomlKey, dotted bool) *tomlTable {
	for _, key := range path {
		item, ok := table.items.get(key.name)
		if !ok {
			child := &tomlTable{implicit: true, dotted: dotted}
			table.items.put(key.clone(), child)
			table = child
			continue
		}
		switch item := item.(type) {
		case *tomlArrayOfTables:
			table = item.tables[len(item.tables)-1]
		case *tomlTable:
			if dotted && item.implicit {
				item.dotted = true
			}
			table = item
		}
	}
	return table
}

func (p *tomlParser) value() tomlValue {
	switch p.peek() {
	case '[':
		return p.array()
	case '{':
		return p.inlineTable()
	case '"', '\'':
		start := p.pos
		text := p.stringLiteral()
		return &tomlScalar{repr: p.src[start:p.pos], text: text, isString: true}
	default:
		start := p.pos
		p.atom()
		return &tomlScalar{repr: p.src[start:p.pos]}
	}
}

// localDate matches the date part of a date-time that may continue after a
// space (`1979-05-27 07:32:00`).
var localDate = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

// atom consumes a number, boolean or date-time.
func (p *tomlParser) atom() {
	start := p.pos
	for p.pos < len(p.src) && !strings.ContainsRune(" \t,]}#\r\n", rune(p.src[p.pos])) {
		p.pos++
	}
	rest := p.src[p.pos:]
	if localDate.MatchString(p.src[start:p.pos]) && len(rest) >= 3 && rest[0] == ' ' &&
		isDigit(rest[1]) && isDigit(rest[2]) {
		p.pos++
		p.atom()
	}
}

func isDigit(c byte) bool {
	return '0' <= c && c <= '9'
}

// separatorDecor gathers whitespace, comments and newlines inside an array or
// inline table into one span.
type separatorDecor struct {
	start, end int
	set        bool
}

func (d *separatorDecor) add(start, end int) {
	if !d.set {
		d.start, d.set = start, true
	}
	d.end = end
}

// text returns the span, or the empty span at fallback when nothing was seen.
func (d *separatorDecor) text(src string, fallback int) string {
	if !d.set {
		return src[fallback:fallback]
	}
	return src[d.start:d.end]
}

// separator consumes one whitespace, comment or newline token, reporting
// false when the next token is something else.
func (p *tomlParser) separator() (int, int, bool) {
	switch c := p.peek(); {
	case c == ' ' || c == '\t':
		start, end := p.whitespace()
		return start, end, true
	case c == '#':
		start, end := p.comment()
		return start, end, true
	case c == '\n' || c == '\r':
		start, end := p.newline()
		return start, end, true
	}
	return 0, 0, false
}

func (p *tomlParser) array() *tomlArray {
	result := &tomlArray{}
	p.pos++ // `[`
	trailingStart := p.pos
	var prefix, suffix separatorDecor
	var current tomlValue
	finish := func(at int) {
		if current == nil {
			return
		}
		*current.valueDecor() = explicitDecor(prefix.text(p.src, at), suffix.text(p.src, at))
		result.values = append(result.values, current)
		current, prefix, suffix = nil, separatorDecor{}, separatorDecor{}
	}
	for p.pos < len(p.src) {
		if start, end, ok := p.separator(); ok {
			if current == nil {
				prefix.add(start, end)
			} else {
				suffix.add(start, end)
			}
			continue
		}
		switch p.peek() {
		case ',':
			finish(p.pos)
			p.pos++
			trailingStart = p.pos
		case ']':
			closeAt := p.pos
			finish(closeAt)
			result.trailingComma = trailingStart >= 0 && len(result.values) > 0
			if trailingStart < 0 {
				trailingStart = closeAt
			}
			result.trailing = p.src[trailingStart:closeAt]
			p.pos++
			return result
		default:
			trailingStart = -1
			if !prefix.set {
				prefix.add(p.pos, p.pos)
			}
			current = p.value()
		}
	}
	return result
}

func (p *tomlParser) inlineTable() *tomlInlineTable {
	result := &tomlInlineTable{}
	p.pos++ // `{`
	trailingStart := p.pos
	var prefix, suffix separatorDecor
	var path []*tomlKey
	var key *tomlKey
	var current tomlValue
	seenKeyValSep := false
	isPrefix := func() bool {
		if seenKeyValSep {
			return current == nil
		}
		return key == nil
	}
	finishValue := func(at int) {
		seenKeyValSep = false
		if key == nil || current == nil {
			return
		}
		*current.valueDecor() = explicitDecor(prefix.text(p.src, at), suffix.text(p.src, at))
		table := descendInlinePath(result, path)
		table.items.put(key, current)
		path, key, current = nil, nil, nil
		prefix, suffix = separatorDecor{}, separatorDecor{}
	}
	for p.pos < len(p.src) {
		if start, end, ok := p.separator(); ok {
			if isPrefix() {
				prefix.add(start, end)
			} else {
				suffix.add(start, end)
			}
			continue
		}
		switch c := p.peek(); {
		case c == '=':
			seenKeyValSep = true
			if key != nil {
				key.leaf = explicitDecor(prefix.text(p.src, p.pos), suffix.text(p.src, p.pos))
			}
			prefix, suffix = separatorDecor{}, separatorDecor{}
			p.pos++
		case c == ',':
			finishValue(p.pos)
			p.pos++
			trailingStart = p.pos
		case c == '}':
			closeAt := p.pos
			finishValue(closeAt)
			result.trailingComma = trailingStart >= 0 && result.items.len() > 0
			if trailingStart < 0 {
				trailingStart = closeAt
			}
			result.trailing = p.src[trailingStart:closeAt]
			p.pos++
			return result
		case !seenKeyValSep:
			trailingStart = -1
			if !prefix.set {
				prefix.add(p.pos, p.pos)
			}
			path, key = p.keyPath()
		default:
			if !prefix.set {
				prefix.add(p.pos, p.pos)
			}
			current = p.value()
		}
	}
	return result
}

// descendInlinePath returns the inline table at path under table, creating
// implicit dotted inline tables on the way.
func descendInlinePath(table *tomlInlineTable, path []*tomlKey) *tomlInlineTable {
	for _, key := range path {
		item, ok := table.items.get(key.name)
		if !ok {
			child := &tomlInlineTable{implicit: true, dotted: true}
			table.items.put(key.clone(), child)
			table = child
			continue
		}
		if child, ok := item.(*tomlInlineTable); ok {
			table = child
		}
	}
	return table
}

// stringLiteral consumes a basic, literal or multi-line string and returns
// its decoded text.
func (p *tomlParser) stringLiteral() string {
	rest := p.src[p.pos:]
	quote := rest[0]
	delimiter := string(quote)
	multiline := strings.HasPrefix(rest, strings.Repeat(delimiter, 3))
	if multiline {
		delimiter = strings.Repeat(delimiter, 3)
	}
	p.pos += len(delimiter)
	start := p.pos
	for p.pos < len(p.src) {
		if quote == '"' && p.src[p.pos] == '\\' {
			p.pos += 2
			continue
		}
		if strings.HasPrefix(p.src[p.pos:], delimiter) {
			break
		}
		p.pos++
	}
	end := p.pos
	p.pos += len(delimiter)
	// Up to two quotes may close a multi-line string's content.
	for extra := 0; multiline && extra < 2 && p.peek() == quote; extra++ {
		end++
		p.pos++
	}
	body := p.src[start:end]
	if multiline {
		if trimmed, ok := strings.CutPrefix(body, "\r\n"); ok {
			body = trimmed
		} else {
			body = strings.TrimPrefix(body, "\n")
		}
	}
	if quote == '\'' {
		return body
	}
	return decodeBasicString(body, multiline)
}

// decodeBasicString resolves the escapes of a basic string body.
func decodeBasicString(body string, multiline bool) string {
	var out strings.Builder
	for i := 0; i < len(body); i++ {
		c := body[i]
		if c != '\\' || i+1 >= len(body) {
			out.WriteByte(c)
			continue
		}
		i++
		switch body[i] {
		case 'b':
			out.WriteByte('\b')
		case 't':
			out.WriteByte('\t')
		case 'n':
			out.WriteByte('\n')
		case 'f':
			out.WriteByte('\f')
		case 'r':
			out.WriteByte('\r')
		case 'e':
			out.WriteByte(0x1b)
		case '"':
			out.WriteByte('"')
		case '\\':
			out.WriteByte('\\')
		case 'x', 'u', 'U':
			width := map[byte]int{'x': 2, 'u': 4, 'U': 8}[body[i]]
			if i+1+width <= len(body) {
				if code, err := strconv.ParseUint(body[i+1:i+1+width], 16, 32); err == nil {
					out.WriteRune(rune(code))
					i += width
					continue
				}
			}
			out.WriteByte('\\')
			out.WriteByte(body[i])
		default:
			// A line-ending backslash in a multi-line string trims the
			// newline and the whitespace after it.
			if multiline {
				j := i
				for j < len(body) && strings.ContainsRune(" \t\r\n", rune(body[j])) {
					j++
				}
				i = j - 1
				continue
			}
			out.WriteByte('\\')
			out.WriteByte(body[i])
		}
	}
	return out.String()
}

// defaultTomlKeyRepr is `toml_writer::TomlKeyBuilder::as_default`.
func defaultTomlKeyRepr(name string) string {
	unquoted := name != ""
	var singleQuotes, doubleQuotes, escapeCodes, escape bool
	for i := 0; i < len(name); i++ {
		c := name[i]
		if !isBareKeyByte(c) {
			unquoted = false
		}
		switch {
		case c == '\'':
			singleQuotes = true
		case c == '"':
			doubleQuotes = true
		case c == '\\':
			escape = true
		case c == '\t':
		case c <= 0x1f || c == 0x7f:
			escapeCodes = true
		}
	}
	switch {
	case unquoted:
		return name
	case !escapeCodes && !escape && !doubleQuotes:
		return encodeTomlString(name, "\"", true, false)
	case !escapeCodes && !singleQuotes:
		return "'" + name + "'"
	default:
		return encodeTomlString(name, "\"", true, false)
	}
}

// defaultTomlStringRepr is `toml_writer::TomlStringBuilder::as_default`.
func defaultTomlStringRepr(text string) string {
	var maxSingle, maxDouble, single, double int
	var escapeCodes, escape, newline bool
	for i := 0; i < len(text); i++ {
		c := text[i]
		if c == '\'' {
			single++
			maxSingle = max(maxSingle, single)
		} else {
			single = 0
		}
		if c == '"' {
			double++
			maxDouble = max(maxDouble, double)
		} else {
			double = 0
		}
		switch {
		case c == '\\':
			escape = true
		case c == '\t':
		case c == '\n':
			newline = true
		case c <= 0x1f || c == 0x7f:
			escapeCodes = true
		}
	}
	switch {
	case !escapeCodes && !escape && maxDouble == 0 && !newline:
		return encodeTomlString(text, "\"", true, false)
	case !escapeCodes && maxSingle == 0 && !newline:
		return "'" + text + "'"
	case !escapeCodes && !escape && maxDouble <= 2:
		return encodeTomlString(text, `"""`, true, newline)
	case !escapeCodes && maxSingle <= 2:
		prefix := ""
		if newline {
			prefix = "\n"
		}
		return "'''" + prefix + text + "'''"
	case newline:
		return encodeTomlString(text, `"""`, true, true)
	default:
		return encodeTomlString(text, "\"", true, false)
	}
}

// encodeTomlString writes a basic (or multi-line basic) string, escaping
// like `toml_writer`.
func encodeTomlString(text, delimiter string, escaped, newlinePrefix bool) string {
	var out strings.Builder
	multiline := len(delimiter) == 3
	out.WriteString(delimiter)
	if newlinePrefix && multiline {
		out.WriteString("\n")
	}
	if !escaped {
		out.WriteString(text)
		out.WriteString(delimiter)
		return out.String()
	}
	maxDouble := 0
	if multiline {
		maxDouble = 2
	}
	stream := text
	for stream != "" {
		unescapedEnd := 0
		escape := ""
		double := 0
		for i := 0; i < len(stream); i++ {
			c := stream[i]
			if c == '"' {
				double++
				if maxDouble < double {
					escape = `\"`
					break
				}
			} else {
				double = 0
			}
			stop := false
			switch {
			case c == '\b':
				escape, stop = `\b`, true
			case c == '\t':
				escape, stop = `\t`, true
			case c == '\n':
				if !multiline {
					escape, stop = `\n`, true
				}
			case c == '\f':
				escape, stop = `\f`, true
			case c == '\r':
				escape, stop = `\r`, true
			case c == '"':
			case c == '\\':
				escape, stop = `\\`, true
			case c <= 0x1f || c == 0x7f:
				stop = true
			}
			if stop {
				break
			}
			unescapedEnd = i + 1
		}
		out.WriteString(stream[:unescapedEnd])
		out.WriteString(escape)
		end := unescapedEnd
		if escape != "" {
			end++
		}
		stream = stream[end:]
		if escape == "" && stream != "" {
			fmt.Fprintf(&out, `\u%04X`, stream[0])
			stream = stream[1:]
		}
	}
	out.WriteString(delimiter)
	return out.String()
}
