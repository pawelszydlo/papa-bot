package papaBot

import (
	"encoding/json"
	"golang.org/x/net/idna"
	"golang.org/x/text/encoding"
	"golang.org/x/text/transform"
	"html"
	"io/ioutil"
	"net/http"
	"strings"
	"time"
)

var httpClient = http.Client{Timeout: 5 * time.Second}

const (
	UserAgent       = "Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)"
	preloadBodySize = 50 * 1024
)

// GetHttpResponse gets and returns a HTTP response object.
func GetHttpResponse(url string) (resp *http.Response, err error) {
	// Build the request
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", UserAgent)

	// Get response
	return httpClient.Do(req)
}

// GetJsonResponse tries to get the body of the response and decode it from JSON.
func GetJsonResponse(url string) (map[string]interface{}, error) {

	// Get HTTP response
	resp, err := GetHttpResponse(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Load body
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Convert from JSON
	var raw_data interface{}
	if err := json.Unmarshal(body, &raw_data); err != nil {
		return nil, err
	}
	return raw_data.(map[string]interface{}), nil
}

// SanitizeString cleans a string.
func SanitizeString(str string, enc encoding.Encoding) string {
	str, _, _ = transform.String(enc.NewDecoder(), str)
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
