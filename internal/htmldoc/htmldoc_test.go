package htmldoc

import (
	"strings"
	"testing"
)

const testHTML = `
<!DOCTYPE html>
<html>
<head><title>Test Page</title></head>
<body>
<div class="header">
  <h1 class="title">   Solo Leveling   </h1>
  <span class="status">Ongoing</span>
</div>
<div class="chapters">
  <a class="chapter" href="/chapter/1">Chapter 1</a>
  <a class="chapter" href="/chapter/2">Chapter 2</a>
  <a class="chapter" href="/chapter/3">Chapter 3</a>
</div>
<img src="/img1.jpg" alt="Cover 1">
<img src="/img2.jpg" alt="Cover 2">
<img alt="No src attribute">
<table>
  <tr><td>Author</td><td>Chugong</td></tr>
</table>
<div class="tag">Fantasy</div>
<div class="tag">Action</div>
</body>
</html>
`

func mustParse(t *testing.T, markup string) *Document {
	t.Helper()
	doc, err := Parse(markup)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	return doc
}

func TestParse(t *testing.T) {
	doc := mustParse(t, testHTML)
	if doc.root == nil {
		t.Fatal("Document.root is nil")
	}
	if doc.doc == nil {
		t.Fatal("Document.doc is nil")
	}
}

// TestEnginesShareOneTree guards the reason the handle exists: a lookup must not
// re-parse the markup, so both engines have to query the same tree.
func TestEnginesShareOneTree(t *testing.T) {
	doc := mustParse(t, testHTML)
	fromCSS, err := doc.FindText("h1.title")
	if err != nil {
		t.Fatalf("FindText failed: %v", err)
	}
	fromXPath, err := doc.XPathText("//h1[@class='title']")
	if err != nil {
		t.Fatalf("XPathText failed: %v", err)
	}
	if fromCSS != fromXPath {
		t.Fatalf("engines disagree: CSS %q, XPath %q", fromCSS, fromXPath)
	}
}

func TestParseNilDocument(t *testing.T) {
	doc := &Document{}
	text, err := doc.FindText("div")
	if err != nil {
		t.Fatalf("FindText on nil doc returned error: %v", err)
	}
	if text != "" {
		t.Fatalf("Expected empty string, got %q", text)
	}
	texts, err := doc.XPathListText("//div")
	if err != nil {
		t.Fatalf("XPathListText on nil doc returned error: %v", err)
	}
	if len(texts) != 0 {
		t.Fatalf("Expected empty list, got %v", texts)
	}
}

func TestFindText(t *testing.T) {
	doc := mustParse(t, testHTML)

	text, err := doc.FindText("h1.title")
	if err != nil {
		t.Fatalf("FindText failed: %v", err)
	}
	if text != "Solo Leveling" {
		t.Fatalf("Expected trimmed 'Solo Leveling', got %q", text)
	}

	text, err = doc.FindText(".nonexistent")
	if err != nil {
		t.Fatalf("FindText on non-existent element returned error: %v", err)
	}
	if text != "" {
		t.Fatalf("Expected empty string, got %q", text)
	}
}

// TestFindTextFirstMatchWins pins "first match in document order", not "best".
func TestFindTextFirstMatchWins(t *testing.T) {
	doc := mustParse(t, `<div class="tag">Fantasy</div><div class="tag">Action</div>`)
	text, err := doc.FindText("div.tag")
	if err != nil {
		t.Fatalf("FindText failed: %v", err)
	}
	if text != "Fantasy" {
		t.Fatalf("Expected the first match 'Fantasy', got %q", text)
	}
}

func TestFindTextInvalidSelector(t *testing.T) {
	doc := mustParse(t, testHTML)

	_, err := doc.FindText("{invalid")
	if err == nil {
		t.Fatal("Expected error for invalid selector, got nil")
	}
	if !strings.Contains(err.Error(), "invalid CSS selector") {
		t.Fatalf("Error should name the failure, got: %v", err)
	}
	if !strings.Contains(err.Error(), "{invalid") {
		t.Fatalf("Error should mention the selector, got: %v", err)
	}
}

// TestValidSelectorsAccepted covers selectors a character blacklist would have
// refused even though the matching engine accepts them.
func TestValidSelectorsAccepted(t *testing.T) {
	doc := mustParse(t, testHTML)
	for _, selector := range []string{
		"div:not(.chapters)",
		"a[href='/chapter/1']",
		"div > p",
		"body div.header",
		"*",
	} {
		if _, err := doc.FindText(selector); err != nil {
			t.Fatalf("selector %q should be accepted, got %v", selector, err)
		}
	}

	positional := mustParse(t, `<ul><li>one</li><li>two</li><li>three</li></ul>`)
	text, err := positional.FindText("li:nth-child(2)")
	if err != nil {
		t.Fatalf("nth-child selector should be accepted, got %v", err)
	}
	if text != "two" {
		t.Fatalf("Expected 'two', got %q", text)
	}
}

// TestInvalidSelectorIsNotASilentMiss is the reason checkCSS exists: goquery.Find
// swallows a compile error and matches nothing, which would look identical to a
// legitimate empty result.
func TestInvalidSelectorIsNotASilentMiss(t *testing.T) {
	doc := mustParse(t, testHTML)
	for _, selector := range []string{"div[", "a[href", "div,,p", ""} {
		if _, err := doc.FindText(selector); err == nil {
			t.Fatalf("selector %q should be reported as invalid, not matched as empty", selector)
		}
	}
}

func TestFindAttr(t *testing.T) {
	doc := mustParse(t, testHTML)

	val, err := doc.FindAttr("a.chapter", "href")
	if err != nil {
		t.Fatalf("FindAttr failed: %v", err)
	}
	if val != "/chapter/1" {
		t.Fatalf("Expected '/chapter/1', got %q", val)
	}

	val, err = doc.FindAttr("h1.title", "nonexistent")
	if err != nil {
		t.Fatalf("FindAttr on missing attr returned error: %v", err)
	}
	if val != "" {
		t.Fatalf("Expected empty string, got %q", val)
	}

	// An attribute name is not a selector and never needs validating.
	if _, err := doc.FindAttr("a.chapter", "href["); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := doc.FindAttr("a.chapter[", "href"); err == nil {
		t.Fatal("Expected error for invalid selector in FindAttr")
	}
}

func TestFindListText(t *testing.T) {
	doc := mustParse(t, testHTML)

	texts, err := doc.FindListText("a.chapter")
	if err != nil {
		t.Fatalf("FindListText failed: %v", err)
	}
	if len(texts) != 3 {
		t.Fatalf("Expected 3 chapters, got %d", len(texts))
	}
	if texts[0] != "Chapter 1" || texts[1] != "Chapter 2" || texts[2] != "Chapter 3" {
		t.Fatalf("Unexpected chapter texts: %v", texts)
	}

	texts, err = doc.FindListText(".nonexistent")
	if err != nil {
		t.Fatalf("FindListText on non-existent returned error: %v", err)
	}
	if len(texts) != 0 {
		t.Fatalf("Expected empty list, got %v", texts)
	}
}

func TestFindListAttrSkipsMissing(t *testing.T) {
	doc := mustParse(t, testHTML)

	srcs, err := doc.FindListAttr("img", "src")
	if err != nil {
		t.Fatalf("FindListAttr failed: %v", err)
	}
	if len(srcs) != 2 {
		t.Fatalf("Expected 2 src values (3 img tags, one without src), got %d: %v", len(srcs), srcs)
	}
	if srcs[0] != "/img1.jpg" || srcs[1] != "/img2.jpg" {
		t.Fatalf("Unexpected srcs: %v", srcs)
	}

	// An attribute that is present but empty still counts as carried.
	empty := mustParse(t, `<p data-x="">a</p><p>b</p>`)
	vals, err := empty.FindListAttr("p", "data-x")
	if err != nil {
		t.Fatalf("FindListAttr failed: %v", err)
	}
	if len(vals) != 1 || vals[0] != "" {
		t.Fatalf("Expected one empty value, got %v", vals)
	}
}

func TestXPathText(t *testing.T) {
	doc := mustParse(t, testHTML)

	text, err := doc.XPathText("//h1[@class='title']")
	if err != nil {
		t.Fatalf("XPathText failed: %v", err)
	}
	if text != "Solo Leveling" {
		t.Fatalf("Expected trimmed 'Solo Leveling', got %q", text)
	}

	text, err = doc.XPathText("//div[@class='nonexistent']")
	if err != nil {
		t.Fatalf("XPathText on non-existent returned error: %v", err)
	}
	if text != "" {
		t.Fatalf("Expected empty string, got %q", text)
	}
}

// TestXPathByPositionOrText covers XPath reaching elements a CSS selector cannot
// name, which is why both engines are exposed.
func TestXPathByPositionOrText(t *testing.T) {
	doc := mustParse(t, testHTML)

	text, err := doc.XPathText("(//td)[2]")
	if err != nil {
		t.Fatalf("positional XPath failed: %v", err)
	}
	if text != "Chugong" {
		t.Fatalf("Expected 'Chugong', got %q", text)
	}

	texts, err := doc.XPathListText("//div[contains(text(),'A')]")
	if err != nil {
		t.Fatalf("text() XPath failed: %v", err)
	}
	if len(texts) != 1 || texts[0] != "Action" {
		t.Fatalf("Expected [Action], got %v", texts)
	}
}

func TestXPathTextInvalidExpression(t *testing.T) {
	doc := mustParse(t, testHTML)

	_, err := doc.XPathText("[invalid")
	if err == nil {
		t.Fatal("Expected error for invalid XPath, got nil")
	}
	if !strings.Contains(err.Error(), "invalid XPath expression") {
		t.Fatalf("Error should name the failure, got: %v", err)
	}
	if !strings.Contains(err.Error(), "[invalid") {
		t.Fatalf("Error should mention the expression, got: %v", err)
	}
}

func TestXPathAttr(t *testing.T) {
	doc := mustParse(t, testHTML)

	val, err := doc.XPathAttr("//img[1]", "src")
	if err != nil {
		t.Fatalf("XPathAttr failed: %v", err)
	}
	if val != "/img1.jpg" {
		t.Fatalf("Expected '/img1.jpg', got %q", val)
	}

	val, err = doc.XPathAttr("//h1", "nonexistent")
	if err != nil {
		t.Fatalf("XPathAttr on missing attr returned error: %v", err)
	}
	if val != "" {
		t.Fatalf("Expected empty string, got %q", val)
	}
}

// TestXPathAttrNameMatchingElementName guards the htmlquery quirk where an
// attribute lookup whose name equals a parentless node's own name answers with
// that node's text.
func TestXPathAttrNameMatchingElementName(t *testing.T) {
	doc := mustParse(t, testHTML)
	val, err := doc.XPathAttr("//html", "html")
	if err != nil {
		t.Fatalf("XPathAttr failed: %v", err)
	}
	if val != "" {
		t.Fatalf("Expected empty string, got %q", val)
	}
}

func TestXPathListText(t *testing.T) {
	doc := mustParse(t, testHTML)

	texts, err := doc.XPathListText("//a[@class='chapter']")
	if err != nil {
		t.Fatalf("XPathListText failed: %v", err)
	}
	if len(texts) != 3 {
		t.Fatalf("Expected 3 chapters, got %d", len(texts))
	}
	if texts[0] != "Chapter 1" || texts[1] != "Chapter 2" || texts[2] != "Chapter 3" {
		t.Fatalf("Unexpected chapter texts: %v", texts)
	}

	texts, err = doc.XPathListText("//div[@class='nonexistent']")
	if err != nil {
		t.Fatalf("XPathListText on non-existent returned error: %v", err)
	}
	if len(texts) != 0 {
		t.Fatalf("Expected empty list, got %v", texts)
	}
}

func TestXPathListAttrSkipsMissing(t *testing.T) {
	doc := mustParse(t, testHTML)

	srcs, err := doc.XPathListAttr("//img", "src")
	if err != nil {
		t.Fatalf("XPathListAttr failed: %v", err)
	}
	if len(srcs) != 2 {
		t.Fatalf("Expected 2 src attributes, got %d: %v", len(srcs), srcs)
	}
	if srcs[0] != "/img1.jpg" || srcs[1] != "/img2.jpg" {
		t.Fatalf("Unexpected srcs: %v", srcs)
	}
}

// TestXPathListAttrInvalidExpression covers the list entry point, which the
// single-value test does not reach.
func TestXPathListAttrInvalidExpression(t *testing.T) {
	doc := mustParse(t, testHTML)
	if _, err := doc.XPathListAttr("][", "src"); err == nil {
		t.Fatal("Expected error for invalid XPath, got nil")
	}
}

func TestMalformedMarkupStillParses(t *testing.T) {
	doc, err := Parse(`<div><p>Unclosed paragraph<div>Nested without closing`)
	if err != nil {
		t.Fatalf("Malformed markup should not error: %v", err)
	}
	if doc.root == nil {
		t.Fatal("Malformed markup produced no tree")
	}

	text, err := doc.FindText("p")
	if err != nil {
		t.Fatalf("Query on malformed doc failed: %v", err)
	}
	if !strings.Contains(text, "Unclosed paragraph") {
		t.Fatalf("Expected the recovered paragraph text, got %q", text)
	}
}

// TestParsePathAsMarkup pins that Parse treats its argument as markup and never
// reaches the filesystem: a path is just text that matches no element.
func TestParsePathAsMarkup(t *testing.T) {
	doc := mustParse(t, "/some/file/path")
	text, err := doc.FindText("h1")
	if err != nil {
		t.Fatalf("Query on path-as-markup failed: %v", err)
	}
	if text != "" {
		t.Fatalf("Path string should not match elements, got %q", text)
	}
}
