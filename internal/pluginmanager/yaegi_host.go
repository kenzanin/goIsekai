package pluginmanager

import (
	"context"

	"goisekai/internal/hostnet"
	"goisekai/internal/htmldoc"
	"goisekai/pkg/types"
)

// yaegiHostPkg is a Yaegi-compatible host bridge.
// It exposes functions that use stdlib-compatible types, because yaegi
// cannot parse bogdanfinn/fhttp source that hostnet uses internally.
type yaegiHostPkg struct {
	proxy *hostnet.Proxy
	id    string
	mgr   *Manager // for the in-flight invoke ctx; nil falls back to background
}

// ctx is the in-flight invoke context, so a host call made by interpreted code
// (an HTTP fetch, a CDP solve) is torn down at the invoke deadline instead of
// outliving it on context.Background(). Mirrors the JS and Go runtimes.
func (y yaegiHostPkg) ctx() context.Context {
	if y.mgr == nil {
		return context.Background()
	}
	return y.mgr.invokeCtx(y.id)
}

// Get is available as hostnet.Get in interpreted plugins.
func (y yaegiHostPkg) Get(url string) (string, error) {
	resp, err := y.proxy.RequestContext(y.ctx(), y.id, types.HTTPRequest{
		Method: "GET",
		URL:    url,
	})
	if err != nil {
		return "", err
	}
	return resp.Body, nil
}

// Post is available as hostnet.Post in interpreted plugins.
func (y yaegiHostPkg) Post(url, body string) (string, error) {
	resp, err := y.proxy.RequestContext(y.ctx(), y.id, types.HTTPRequest{
		Method: "POST",
		URL:    url,
		Body:   body,
	})
	if err != nil {
		return "", err
	}
	return resp.Body, nil
}

// Parse is available as hostnet.Parse in interpreted plugins.
func (y yaegiHostPkg) Parse(markup string) (*htmldoc.Document, error) {
	return htmldoc.Parse(markup)
}

// FindText is available as hostnet.FindText in interpreted plugins.
func (y yaegiHostPkg) FindText(doc *htmldoc.Document, selector string) (string, error) {
	return doc.FindText(selector)
}

// FindAttr is available as hostnet.FindAttr in interpreted plugins.
func (y yaegiHostPkg) FindAttr(doc *htmldoc.Document, selector, attr string) (string, error) {
	return doc.FindAttr(selector, attr)
}

// FindListText is available as hostnet.FindListText in interpreted plugins.
func (y yaegiHostPkg) FindListText(doc *htmldoc.Document, selector string) ([]string, error) {
	return doc.FindListText(selector)
}

// FindListAttr is available as hostnet.FindListAttr in interpreted plugins.
func (y yaegiHostPkg) FindListAttr(doc *htmldoc.Document, selector, attr string) ([]string, error) {
	return doc.FindListAttr(selector, attr)
}

// XPathText is available as hostnet.XPathText in interpreted plugins.
func (y yaegiHostPkg) XPathText(doc *htmldoc.Document, expr string) (string, error) {
	return doc.XPathText(expr)
}

// XPathAttr is available as hostnet.XPathAttr in interpreted plugins.
func (y yaegiHostPkg) XPathAttr(doc *htmldoc.Document, expr, attr string) (string, error) {
	return doc.XPathAttr(expr, attr)
}

// XPathListText is available as hostnet.XPathListText in interpreted plugins.
func (y yaegiHostPkg) XPathListText(doc *htmldoc.Document, expr string) ([]string, error) {
	return doc.XPathListText(expr)
}

// XPathListAttr is available as hostnet.XPathListAttr in interpreted plugins.
func (y yaegiHostPkg) XPathListAttr(doc *htmldoc.Document, expr, attr string) ([]string, error) {
	return doc.XPathListAttr(expr, attr)
}
