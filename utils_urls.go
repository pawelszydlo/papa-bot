package papaBot

import (
	"golang.org/x/net/idna"
	"golang.org/x/text/encoding"
	"golang.org/x/text/transform"
	"html"
	"strings"
)

// Sanitize string.
func SanitizeString(str string, enc encoding.Encoding) string {
	str, _, _ = transform.String(enc.NewDecoder(), str)
	str = html.UnescapeString(str)
	str = strings.Replace(str, "\n", "", -1)
	str = strings.Replace(str, "\r", "", -1)
	return strings.Trim(str, " ")
}

// Standardizes the url.
func StandardizeURL(url string) string {
	link := url
	var schema, domain, path string

	// Try to get the schema
	slice := strings.SplitN(url, "://", 2)
	if len(slice) == 2 && len(slice[0]) < 10 { // schema exists
		schema = slice[0] + "://"
		link = slice[1]
	} else {
		schema = "http://"
	}

	// Get the domain
	slice = strings.SplitN(link, "/", 2)
	if len(slice) == 2 {
		domain = slice[0]
		path = "/" + slice[1]
	} else {
		domain = slice[0]
		path = "/"
	}

	domain, _ = idna.ToASCII(domain)
	link = schema + domain + path

	return link
}
