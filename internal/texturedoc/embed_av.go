package texturedoc

import (
	"fmt"
	"net/url"
	"path"
	"regexp"
	"strings"
	"unicode/utf8"
)

// The video and audio embeds (station L8, last slice).

// MaxEmbedURLRunes caps an audio URL.
const MaxEmbedURLRunes = 2048

// VideoIDRE is a YouTube video id: 11 URL-safe base64 characters.
var VideoIDRE = regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`)

var audioExts = map[string]bool{".mp3": true, ".m4a": true, ".ogg": true, ".opus": true, ".wav": true}

// podcastEnclosureHosts serve podcast episodes by redirect or tracking path,
// so the URL may carry no audio extension.
var podcastEnclosureHosts = []string{
	"traffic.megaphone.fm", "dcs.megaphone.fm", "dts.podtrac.com", "play.podtrac.com", "chtbl.com",
	"media.transistor.fm", "stitcher.simplecastaudio.com", "injector.simplecastaudio.com",
	"audio.buzzsprout.com", "cdn.simplecast.com", "pdst.fm", "mgln.ai", "feeds.soundcloud.com",
	"api.spreaker.com", "pscrb.fm", "arttrk.com",
}

// ParseYouTubeURL returns the canonical video id of a youtube.com/watch?v=,
// youtu.be/ or youtube.com/shorts/ https URL; the id is all a video embed
// keeps.
func ParseYouTubeURL(raw string) (string, error) {
	u, err := parseHTTPS(raw)
	if err != nil {
		return "", err
	}
	host := strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
	host = strings.TrimPrefix(host, "m.")
	var id string
	switch host {
	case "youtube.com":
		switch {
		case u.Path == "/watch":
			id = u.Query().Get("v")
		case strings.HasPrefix(u.Path, "/shorts/"):
			id = strings.TrimPrefix(u.Path, "/shorts/")
		}
	case "youtu.be":
		id = strings.TrimPrefix(u.Path, "/")
	default:
		return "", fmt.Errorf("is not a YouTube URL (youtube.com/watch?v=, youtu.be/ or youtube.com/shorts/)")
	}
	if !VideoIDRE.MatchString(id) {
		return "", fmt.Errorf("has no valid YouTube video id (11 letters, digits, - or _)")
	}
	return id, nil
}

func parseHTTPS(raw string) (*url.URL, error) {
	switch {
	case raw == "":
		return nil, fmt.Errorf("is required (an https URL)")
	case utf8.RuneCountInString(raw) > MaxEmbedURLRunes:
		return nil, fmt.Errorf("is longer than %d characters", MaxEmbedURLRunes)
	}
	for _, r := range raw {
		if r <= 0x20 || r == 0x7f {
			return nil, fmt.Errorf("must not hold spaces or control characters")
		}
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" {
		return nil, fmt.Errorf("must be an https URL")
	}
	if u.User != nil {
		return nil, fmt.Errorf("must not carry credentials")
	}
	return u, nil
}

// ValidAudioURL says raw is an https URL to an audio file: an mp3, m4a, ogg,
// opus or wav path, or a known podcast-episode enclosure host.
func ValidAudioURL(raw string) error {
	u, err := parseHTTPS(raw)
	if err != nil {
		return err
	}
	if audioExts[strings.ToLower(path.Ext(u.Path))] {
		return nil
	}
	host := strings.ToLower(u.Hostname())
	for _, h := range podcastEnclosureHosts {
		if host == h || strings.HasSuffix(host, "."+h) {
			return nil
		}
	}
	return fmt.Errorf("is not an audio file URL (an https .mp3, .m4a, .ogg, .opus or .wav, or a known podcast enclosure host)")
}
