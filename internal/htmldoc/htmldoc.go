// Package htmldoc provides HTML parsing and querying for plugins.
package htmldoc

import (
	"fmt"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/andybalholm/cascadia"
	"github.com/antchfx/htmlquery"
	"golang.org/x/net/html"
)

// Document is an opaque handle to a parsed HTML document. The tree is parsed
// once and shared by both query engines, so no lookup re-parses it.
type Document struct {
	root *html.Node
	doc  *goquery.Document
}

// Parse parses the HTML markup and returns a document handle. Malformed markup
// still yields a handle: x/net/html always recovers some tree.
func Parse(markup string) (*Document, error) {
	root, err := html.Parse(strings.NewReader(markup))
	if err != nil {
		return nil, err
	}
	return &Document{root: root, doc: goquery.NewDocumentFromNode(root)}, nil
}

// checkCSS compiles a selector with cascadia, the engine goquery matches with.
// goquery.Find swallows a compile error and matches nothing instead, so an
// unparseable selector would be indistinguishable from one that found no
// element; compiling here keeps the two apart.
func checkCSS(selector string) error {
	if _, err := cascadia.Compile(selector); err != nil {
		return fmt.Errorf("invalid CSS selector %q: %w", selector, err)
	}
	return nil
}

// FindText returns the trimmed text of the first element matching the selector.
func (d *Document) FindText(selector string) (string, error) {
	if d.doc == nil {
		return "", nil
	}
	if err := checkCSS(selector); err != nil {
		return "", err
	}
	return strings.TrimSpace(d.doc.Find(selector).First().Text()), nil
}

// FindAttr returns the named attribute of the first element matching the
// selector, or the empty string when the element does not carry it.
func (d *Document) FindAttr(selector, attr string) (string, error) {
	if d.doc == nil {
		return "", nil
	}
	if err := checkCSS(selector); err != nil {
		return "", err
	}
	val, _ := d.doc.Find(selector).First().Attr(attr)
	return val, nil
}

// FindListText returns the trimmed text of every element matching the
// selector, in document order.
func (d *Document) FindListText(selector string) ([]string, error) {
	if d.doc == nil {
		return nil, nil
	}
	if err := checkCSS(selector); err != nil {
		return nil, err
	}
	var out []string
	d.doc.Find(selector).Each(func(_ int, sel *goquery.Selection) {
		out = append(out, strings.TrimSpace(sel.Text()))
	})
	return out, nil
}

// FindListAttr returns the named attribute of every element matching the
// selector, in document order. Elements that do not carry the attribute are
// skipped, so the list stays index-aligned with the values that exist.
func (d *Document) FindListAttr(selector, attr string) ([]string, error) {
	if d.doc == nil {
		return nil, nil
	}
	if err := checkCSS(selector); err != nil {
		return nil, err
	}
	var out []string
	d.doc.Find(selector).Each(func(_ int, sel *goquery.Selection) {
		if val, ok := sel.Attr(attr); ok {
			out = append(out, val)
		}
	})
	return out, nil
}

// XPathText returns the trimmed text of the first node the expression selects.
func (d *Document) XPathText(expr string) (string, error) {
	node, err := d.xpathOne(expr)
	if err != nil || node == nil {
		return "", err
	}
	return strings.TrimSpace(htmlquery.InnerText(node)), nil
}

// nodeAttr returns the named attribute of a node and whether the node carries
// it. htmlquery.SelectAttr cannot tell an absent attribute from an empty one,
// and on a parentless node it answers with the element text when the name
// matches the element, so the lookups check the attribute list directly.
func nodeAttr(n *html.Node, attr string) (string, bool) {
	for _, a := range n.Attr {
		if a.Key == attr {
			return a.Val, true
		}
	}
	return "", false
}

// XPathAttr returns the named attribute of the first node the expression
// selects, or the empty string when the node does not carry it.
func (d *Document) XPathAttr(expr, attr string) (string, error) {
	node, err := d.xpathOne(expr)
	if err != nil || node == nil {
		return "", err
	}
	val, _ := nodeAttr(node, attr)
	return val, nil
}

// XPathListText returns the trimmed text of every node the expression selects,
// in document order.
func (d *Document) XPathListText(expr string) ([]string, error) {
	nodes, err := d.xpathAll(expr)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, node := range nodes {
		out = append(out, strings.TrimSpace(htmlquery.InnerText(node)))
	}
	return out, nil
}

// XPathListAttr returns the named attribute of every node the expression
// selects, in document order. Nodes that do not carry the attribute are
// skipped, matching the CSS list lookup.
func (d *Document) XPathListAttr(expr, attr string) ([]string, error) {
	nodes, err := d.xpathAll(expr)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, node := range nodes {
		if val, ok := nodeAttr(node, attr); ok {
			out = append(out, val)
		}
	}
	return out, nil
}

// xpathOne returns the first node the expression selects, or nil when nothing
// matched. An unparseable expression is an error naming the expression.
func (d *Document) xpathOne(expr string) (*html.Node, error) {
	if d.root == nil {
		return nil, nil
	}
	node, err := htmlquery.Query(d.root, expr)
	if err != nil {
		return nil, fmt.Errorf("invalid XPath expression %q: %w", expr, err)
	}
	return node, nil
}

// xpathAll returns every node the expression selects, in document order.
func (d *Document) xpathAll(expr string) ([]*html.Node, error) {
	if d.root == nil {
		return nil, nil
	}
	nodes, err := htmlquery.QueryAll(d.root, expr)
	if err != nil {
		return nil, fmt.Errorf("invalid XPath expression %q: %w", expr, err)
	}
	return nodes, nil
}
