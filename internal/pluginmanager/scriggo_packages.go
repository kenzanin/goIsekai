package pluginmanager

import (
	"encoding/json"
	"fmt"
	"goisekai/internal/hostnet"
	"goisekai/pkg/types"
	"strings"

	"github.com/open2b/scriggo/native"
)

// buildScriggoShim generates the main.go shim that dispatches ABI calls.
// The shim does NOT import "fmt" — it uses native err.Error() and string
// concatenation to avoid requiring any stdlib registration for the shim itself.
func buildScriggoShim(modPath string, hasAltTitles, hasInit bool) string {
	var b strings.Builder
	b.WriteString("package main\n\nimport (\n")
	fmt.Fprintf(&b, "\tp %q\n", modPath+"/plugin")
	b.WriteString("\t\"hostapi\"\n")
	b.WriteString(")\n\n")
	b.WriteString("func main() {\n")
	b.WriteString("\tfn := hostapi.Fn()\n")
	b.WriteString("\targ := hostapi.Arg()\n")
	b.WriteString("\tvar out string\n")
	b.WriteString("\tvar err error\n")
	b.WriteString("\tswitch fn {\n")
	for _, name := range scriggoRequired {
		fmt.Fprintf(&b, "\tcase %q:\n\t\tout, err = p.%s(arg)\n", name, name)
	}
	if hasAltTitles {
		fmt.Fprintf(&b, "\tcase %q:\n\t\tout, err = p.%s(arg)\n", types.GetAltTitlesFunc, types.GetAltTitlesFunc)
	}
	if hasInit {
		fmt.Fprintf(&b, "\tcase %q:\n\t\tout = p.%s()\n", types.InitFunc, types.InitFunc)
	}
	b.WriteString("\tdefault:\n\t\thostapi.Report(out, \"unknown ABI function: \"+fn)\n\t\treturn\n")
	b.WriteString("\t}\n")
	b.WriteString("\tif err != nil {\n")
	b.WriteString("\t\thostapi.Report(out, err.Error())\n")
	b.WriteString("\t} else {\n")
	b.WriteString("\t\thostapi.Report(out, \"\")\n")
	b.WriteString("\t}\n")
	b.WriteString("}\n")
	return b.String()
}

// hostapiPackage returns a native.Package providing the host API bridge.
func hostapiPackage(host *scriggoHost) native.Package {
	return native.Package{
		Name: "hostapi",
		Declarations: native.Declarations{
			"Fn":     func() string { return host.fn },
			"Arg":    func() string { return host.arg },
			"Report": func(out, errMsg string) { host.out, host.errMsg = out, errMsg },
		},
	}
}

// hostnetPackage returns a native.Package providing network access through the
// hostnet proxy.
func hostnetPackage(pluginID string, proxy *hostnet.Proxy) native.Package {
	get := func(url string) (string, error) {
		reqJSON, err := json.Marshal(types.HTTPRequest{Method: "GET", URL: url})
		if err != nil {
			return "", fmt.Errorf("hostnet.Get: marshal request: %w", err)
		}
		respJSON, err := proxy.HandleRequest(pluginID, string(reqJSON))
		if err != nil {
			return "", fmt.Errorf("hostnet.Get: %w", err)
		}
		var resp types.HTTPResponse
		if err := json.Unmarshal([]byte(respJSON), &resp); err != nil {
			return "", fmt.Errorf("hostnet.Get: unmarshal response: %w", err)
		}
		if resp.Status == 0 {
			return "", fmt.Errorf("hostnet.Get: request to %s failed (status 0)", url)
		}
		if resp.Status >= 400 {
			return "", fmt.Errorf("hostnet.Get: %s returned status %d", url, resp.Status)
		}
		return resp.Body, nil
	}

	post := func(url, body string) (string, error) {
		reqJSON, err := json.Marshal(types.HTTPRequest{Method: "POST", URL: url, Body: body})
		if err != nil {
			return "", fmt.Errorf("hostnet.Post: marshal request: %w", err)
		}
		respJSON, err := proxy.HandleRequest(pluginID, string(reqJSON))
		if err != nil {
			return "", fmt.Errorf("hostnet.Post: %w", err)
		}
		var resp types.HTTPResponse
		if err := json.Unmarshal([]byte(respJSON), &resp); err != nil {
			return "", fmt.Errorf("hostnet.Post: unmarshal response: %w", err)
		}
		if resp.Status == 0 {
			return "", fmt.Errorf("hostnet.Post: request to %s failed (status 0)", url)
		}
		if resp.Status >= 400 {
			return "", fmt.Errorf("hostnet.Post: %s returned status %d", url, resp.Status)
		}
		return resp.Body, nil
	}

	return native.Package{
		Name: "hostnet",
		Declarations: native.Declarations{
			"Get":  get,
			"Post": post,
		},
	}
}

// scriggoFmtPackage returns a native.Package wrapping a useful subset of real
// fmt functions. Plugins may import "fmt" to use these.
func scriggoFmtPackage() native.Package {
	return native.Package{
		Name: "fmt",
		Declarations: native.Declarations{
			"Println": fmt.Println,
			"Printf":  fmt.Printf,
			"Sprintf": fmt.Sprintf,
			"Sprint":  fmt.Sprint,
			"Errorf":  fmt.Errorf,
		},
	}
}
