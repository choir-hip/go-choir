package sources

import (
	"bytes"
	"encoding/xml"
	"errors"
	"strings"

	"golang.org/x/net/html/charset"
)

// ErrNotAFeed is a body that is not an RSS, Atom or RDF feed document.
var ErrNotAFeed = errors.New("the response is not an RSS or Atom feed")

// FeedProbe is what an owner is told about a feed they ask to add.
type FeedProbe struct {
	Title string
	Items int
}

type probeEnvelope struct {
	XMLName xml.Name
	Title   string `xml:"title"`
	Channel *struct {
		Title string     `xml:"title"`
		Items []struct{} `xml:"item"`
	} `xml:"channel"`
	Items   []struct{} `xml:"item"`
	Entries []struct{} `xml:"entry"`
}

// ProbeFeed reads a feed document's title and item count without keeping the
// items. A document whose root is not rss, feed or RDF is ErrNotAFeed, so an
// HTML page that parses as loose XML is refused as what it is.
func ProbeFeed(data []byte) (FeedProbe, error) {
	var env probeEnvelope
	decoder := xml.NewDecoder(bytes.NewReader(stripInvalidXMLControlBytes(data)))
	decoder.CharsetReader = charset.NewReaderLabel
	decoder.Strict = false
	if err := decoder.Decode(&env); err != nil {
		return FeedProbe{}, ErrNotAFeed
	}
	switch strings.ToLower(env.XMLName.Local) {
	case "rss", "feed", "rdf":
	default:
		return FeedProbe{}, ErrNotAFeed
	}
	probe := FeedProbe{Title: strings.TrimSpace(env.Title)}
	if env.Channel != nil {
		if t := strings.TrimSpace(env.Channel.Title); t != "" {
			probe.Title = t
		}
		probe.Items = len(env.Channel.Items)
	}
	probe.Items += len(env.Items) + len(env.Entries)
	return probe, nil
}
