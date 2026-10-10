package htmldoc

import (
	"fmt"
	"strings"

	"github.com/antchfx/htmlquery"
	"golang.org/x/net/html"
)

// XPathText returns the trimmed text of the first node the expression selects.
func (d *Document) XPathText(expr string) (string, error) {
	node, err := d.xpathOne(expr)
	if err != nil || node == nil {
		return "", err
	}
	return strings.TrimSpace(htmlquery.InnerText(node)), nil
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
