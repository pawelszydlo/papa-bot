package papaBot

import (
	"html"
	"strings"

	"golang.org/x/net/idna"
)

// SanitizeString cleans a string.
func SanitizeString(str string) string {
	str = html.UnescapeString(str)
	str = strings.Replace(str, "\n", "", -1)
	str = strings.Replace(str, "\r", "", -1)
	return strings.Trim(str, " ")
}

// StandardizeURL standardizes the url by making sure it has a schema and converting IDNA domains into ASCII.
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
