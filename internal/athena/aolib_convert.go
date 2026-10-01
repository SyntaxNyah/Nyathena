package athena

import (
	"strings"

	aolib "github.com/AO-Underground/aolib/go/v2"
)

// leEvidenceItems converts Nyathena's "name&description&image" evidence strings
// into aolib's typed []LEEvidenceItem.
func leEvidenceItems(ev []string) []aolib.LEEvidenceItem {
	out := make([]aolib.LEEvidenceItem, len(ev))
	for i, s := range ev {
		parts := strings.SplitN(s, "&", 3)
		if len(parts) > 0 {
			out[i].Name = parts[0]
		}
		if len(parts) > 1 {
			out[i].Description = parts[1]
		}
		if len(parts) > 2 {
			out[i].Image = parts[2]
		}
	}
	return out
}

// scCharDataItems converts Nyathena's "name&desc&evidence" character strings
// into aolib's typed []SCCharDataItem.
func scCharDataItems(strs []string) []aolib.SCCharDataItem {
	out := make([]aolib.SCCharDataItem, len(strs))
	for i, s := range strs {
		parts := strings.SplitN(s, "&", 3)
		if len(parts) > 0 {
			out[i].Name = parts[0]
		}
		if len(parts) > 1 {
			out[i].Desc = parts[1]
		}
		if len(parts) > 2 {
			out[i].Evidence = parts[2]
		}
	}
	return out
}

// smMusicListItems converts music-name strings into aolib's typed
// []SMMusicListItem.
func smMusicListItems(strs []string) []aolib.SMMusicListItem {
	out := make([]aolib.SMMusicListItem, len(strs))
	for i, s := range strs {
		out[i].Name = s
	}
	return out
}

// charAvailabilities converts Nyathena's "0"/"-1" taken flags into aolib's
// typed []CharAvailability.
func charAvailabilities(taken []string) []aolib.CharAvailability {
	out := make([]aolib.CharAvailability, len(taken))
	for i, s := range taken {
		if s == "-1" {
			out[i] = aolib.CharAvailabilityTaken
		} else {
			out[i] = aolib.CharAvailabilityFree
		}
	}
	return out
}
