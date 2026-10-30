package compile

import (
	"testing"
)

// The `toml_edit` and `serde_json` semantics compile reproduces. End-to-end
// fidelity against the mda CLI is TestCompileMatchesTheMdaCLI; these pin the
// corners its fixtures do not reach.

func TestTomlDocumentsRoundTripUnchanged(t *testing.T) {
	for _, source := range []string{
		"",
		"# only a comment\n",
		"a = 1\n",
		"# leading comment\n\ntitle = \"x\"   # trailing\n\"quoted key\" = 'literal'\n" +
			"a.b . c = 1\nsite.\"google.com\" = true\n",
		"arr = [\n  1, # one\n  [2, 3],\n  { k = \"v\", n.m = 1 },\n]\nempty = [ ]\ninline = {}\n",
		"ml = \"\"\"\nline \\\n  continued\"\"\"\nlit = '''\nraw ''quoted'''''\nesc = \"tab\\t \\u00e9 \\\"\"\n",
		"date = 1979-05-27 07:32:00Z\nlocal = 1979-05-27\nfloat = +1.5e3\nhex = 0xDEAD_BEEF\ninf = -inf\n",
		"[table]   # header comment\nkey = true\n\n  [ table.sub ]\nx = 1\n\n[[aot]]\nn = 1\n\n[[aot]]\nn = 2\n\n# end\n",
		"[a.b]\nx = 1\n[a]\ny = 2\n[c]\n",
		"[x]\ny.z = 1\ny.w = 2\n",
		"\n\n[project]\nname = \"x\"\n\n\n",
	} {
		doc, err := parseTomlDocument(source)
		if err != nil {
			t.Fatalf("%q: %v", source, err)
		}
		if got := doc.String(); got != source {
			t.Fatalf("round trip of %q gave %q", source, got)
		}
	}
}

func TestTomlEndsEveryKeyValueWithANewline(t *testing.T) {
	doc, err := parseTomlDocument("a = 1")
	if err != nil {
		t.Fatal(err)
	}
	if got := doc.String(); got != "a = 1\n" {
		t.Fatalf("got %q", got)
	}
}

func TestTomlCarriageReturnsAreDroppedFromKeptWhitespace(t *testing.T) {
	doc, err := parseTomlDocument("a = 1\r\n\r\n# c\r\n[t]\r\nb = [\r\n  1,\r\n]\r\n")
	if err != nil {
		t.Fatal(err)
	}
	if got := doc.String(); got != "a = 1\n\n# c\n[t]\nb = [\n  1,\n]\n" {
		t.Fatalf("got %q", got)
	}
}

func TestTomlRejectsInvalidDocuments(t *testing.T) {
	for _, source := range []string{"[project\n", "a = 1\na = 2\n", "a = \n"} {
		if _, err := parseTomlDocument(source); err == nil {
			t.Fatalf("%q parsed", source)
		}
	}
}

func TestTomlGetMutInsertsPlaceholdersLikeIndexMut(t *testing.T) {
	doc, err := parseTomlDocument("[project]\nname = \"x\"\n\n[tool]\nother = 1\n")
	if err != nil {
		t.Fatal(err)
	}

	// `tool.get_mut("uv")` inserts a placeholder, which `get_mut("sources")`
	// turns into an inline table.
	if slot := tomlGetMutPath(doc.root, "tool", "uv", "sources"); slot == nil || !isTomlNone(*slot) {
		t.Fatalf("slot: %v", slot)
	}
	// A placeholder in a table renders as nothing.
	if slot := tomlGetMutPath(doc.root, "project", "dependencies"); slot == nil || !isTomlNone(*slot) {
		t.Fatalf("slot: %v", slot)
	}
	// The root lookup never inserts.
	if slot := tomlGetMutPath(doc.root, "missing", "x"); slot != nil {
		t.Fatalf("slot: %v", slot)
	}
	if got := doc.String(); got != "[project]\nname = \"x\"\n\n[tool]\nother = 1\nuv = {}\n" {
		t.Fatalf("got %q", got)
	}
}

func TestTomlInsertResetsAnExistingKeysFormatting(t *testing.T) {
	doc, err := parseTomlDocument("[sources]\n# The sibling.\n\"dep\"   =   { path = \"x\" }   # old\nother = 1\n")
	if err != nil {
		t.Fatal(err)
	}
	item, _ := doc.root.items.get("sources")
	table := item.(*tomlTable)
	source := &tomlInlineTable{}
	source.items.put(newTomlKey("path"), newTomlString("y"))
	source.items.put(newTomlKey("editable"), newTomlBool(true))

	table.insertItem("dep", source)
	table.insertItem("new key", newTomlString("it's"))

	if got := doc.String(); got != "[sources]\ndep = { path = \"y\", editable = true }\nother = 1\n\"new key\" = \"it's\"\n" {
		t.Fatalf("got %q", got)
	}
}

func TestTomlStringsUseTheDefaultRepresentation(t *testing.T) {
	for text, want := range map[string]string{
		"plain":            `"plain"`,
		`say "hi"`:         `'say "hi"'`,
		`back\slash`:       `'back\slash'`,
		"it's \"both\"":    `"""it's "both""""`,
		"line\nbreak":      "\"\"\"\nline\nbreak\"\"\"",
		"tab\there":        `"tab\there"`,
		"bell\x07":         `"bell\u0007"`,
		"it's \\ \"both\"": `'''it's \ "both"'''`,
	} {
		if got := defaultTomlStringRepr(text); got != want {
			t.Fatalf("%q: got %s, want %s", text, got, want)
		}
	}
	for name, want := range map[string]string{
		"py-modules":  "py-modules",
		"google.com":  `"google.com"`,
		`say "hi"`:    `'say "hi"'`,
		"":            `""`,
		"tab\tin key": `"tab\tin key"`,
	} {
		if got := defaultTomlKeyRepr(name); got != want {
			t.Fatalf("%q: got %s, want %s", name, got, want)
		}
	}
}

func TestJSONNumbersPrintAsSerdeJSONReadsThem(t *testing.T) {
	for literal, want := range map[string]string{
		"0":                    "0",
		"-12":                  "-12",
		"18446744073709551615": "18446744073709551615",
		"18446744073709551616": "1.8446744073709552e19",
		"-0":                   "-0.0",
		"1.0":                  "1.0",
		"1e2":                  "100.0",
		"1.5e-7":               "1.5e-7",
		"0.0001":               "0.0001",
		"123456789012345678.5": "1.2345678901234568e17",
		"1e16":                 "1e16",
		"2.5e15":               "2500000000000000.0",
		"-3.25":                "-3.25",
		"0.000001":             "1e-6",
	} {
		if got := normalizeJSONNumber(literal); string(got) != want {
			t.Fatalf("%s: got %s, want %s", literal, got, want)
		}
	}
}

func TestJSONKeepsKeyOrderAndEscapesLikeSerdeJSON(t *testing.T) {
	object, ok, err := parseJSONObject([]byte(`{"b": 1, "a": {"x": [], "y": {}}, "b": 2, "s": "\u0001/<é>\"\\\n"}`))
	if err != nil || !ok {
		t.Fatalf("got %v, %v", ok, err)
	}
	// A duplicate key keeps its first place and its last value.
	want := "{\n  \"b\": 2,\n  \"a\": {\n    \"x\": [],\n    \"y\": {}\n  },\n  \"s\": \"\\u0001/<é>\\\"\\\\\\n\"\n}"
	if got := prettyJSON(object); got != want {
		t.Fatalf("got %s", got)
	}

	// `remove` swaps the last key into the removed one's place.
	object.remove("b")
	object.insert("c", true)
	object.insert("a", nil)
	if got := prettyJSON(object); got != "{\n  \"s\": \"\\u0001/<é>\\\"\\\\\\n\",\n  \"a\": null,\n  \"c\": true\n}" {
		t.Fatalf("got %s", got)
	}

	for _, text := range []string{`{"a": 1} x`, `{"a": }`, ``} {
		if _, err := parseJSON([]byte(text)); err == nil {
			t.Fatalf("%q parsed", text)
		}
	}
}
