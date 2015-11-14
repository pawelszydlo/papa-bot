// Functions helping in lexical analysis.
package lexical

import (
	"encoding/csv"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
)

var StopWords = map[string]bool{}
var wordRe = regexp.MustCompile(`(\pL+)`)
var quoteRe = regexp.MustCompile(`("[\pL| |\pP]{1,40}?"|'[\pL| |\pP]{1,40}?')`)

// LoadStopWords will load stop words for a given language.
func LoadStopWords(lang string) error {
	// Load stopwords from file.
	file, err := os.Open(fmt.Sprintf("stopwords/%s.csv", lang))
	if err != nil {
		return err
	}
	defer file.Close()
	words, err := csv.NewReader(file).ReadAll()
	if err != nil {
		return err
	}
	if len(words) == 0 {
		return errors.New("Empty stop word file.")
	}

	for _, word := range words {
		StopWords[strings.ToLower(word[0])] = true
	}

	return nil
}

// RemoveStopWords will remove stop words from a word list.
func RemoveStopWords(tokenized []string) []string {
	replaced := []string{}
	for _, word := range tokenized {
		if !StopWords[strings.ToLower(word)] {
			replaced = append(replaced, word)
		}
	}
	return replaced
}

// Tokenize will ignore non-alphanumeric characters and return only words.
func Tokenize(text string) []string {
	wordRe.Longest()
	wordsArray := wordRe.FindAllStringSubmatch(text, -1)
	words := []string{}
	for _, word := range wordsArray {
		words = append(words, word[0])
	}
	return words
}

// FindQuotes will return all texts in quotes.
func FindQuotes(text string) []string {
	namesArray := quoteRe.FindAllStringSubmatch(text, -1)
	names := []string{}
	for _, word := range namesArray {
		names = append(names, strings.Trim(word[1], `"' `))
	}
	return names
}
