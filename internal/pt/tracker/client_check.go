package tracker

import (
	"strings"
)

// AllowedClientPrefixes defines valid BitTorrent client PeerID prefixes aligned with NexusPHP.
var AllowedClientPrefixes = []string{
	"-qB", // qBittorrent
	"-TR", // Transmission
	"-DE", // Deluge
	"-lt", // libtorrent / rTorrent
	"-UT", // uTorrent
	"-XL", // Aria2/others if allowed or custom
}

// BlockedAgents defines User-Agents that are prohibited (e.g. web browsers or cheat tools).
var BlockedAgents = []string{
	"Mozilla",
	"Chrome",
	"Safari",
	"Opera",
	"curl",
	"Wget",
	"Baidu",
	"Thunder",
}

// IsBrowserAccess returns true if the user agent looks like a standard web browser.
func IsBrowserAccess(agent string) bool {
	lower := strings.ToLower(agent)
	return strings.Contains(lower, "mozilla") ||
		strings.Contains(lower, "chrome") ||
		strings.Contains(lower, "safari") ||
		strings.Contains(lower, "webkit")
}

// CheckClient validates the peer ID and user agent against PT security rules.
// Returns an error message if rejected, or empty string if allowed.
func CheckClient(peerID []byte, userAgent string) string {
	if len(peerID) != 20 {
		return "Invalid peer ID length"
	}

	if IsBrowserAccess(userAgent) {
		return "Browser access is prohibited. Please use an authorized BitTorrent client."
	}

	peerIDStr := string(peerID)
	// Check allowed prefixes
	isAllowed := false
	for _, p := range AllowedClientPrefixes {
		if strings.HasPrefix(peerIDStr, p) {
			isAllowed = true
			break
		}
	}

	// For flexibility, if userAgent explicitly identifies an allowed client, also permit it
	lowerAgent := strings.ToLower(userAgent)
	if strings.Contains(lowerAgent, "qbittorrent") ||
		strings.Contains(lowerAgent, "transmission") ||
		strings.Contains(lowerAgent, "deluge") ||
		strings.Contains(lowerAgent, "rtorrent") ||
		strings.Contains(lowerAgent, "utorrent") {
		isAllowed = true
	}

	if !isAllowed {
		return "Client is not whitelisted. Recommended clients: qBittorrent, Transmission, Deluge."
	}

	return ""
}
