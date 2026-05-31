package discord

// Activity is the SET_ACTIVITY payload. Empty fields are omitted so
// Discord falls back to its defaults.
type Activity struct {
	Type       int         `json:"type,omitempty"`
	Name       string      `json:"name,omitempty"`
	Details    string      `json:"details,omitempty"`
	State      string      `json:"state,omitempty"`
	URL        string      `json:"url,omitempty"`
	Timestamps *Timestamps `json:"timestamps,omitempty"`
	Assets     *Assets     `json:"assets,omitempty"`
	Party      *Party      `json:"party,omitempty"`
	Buttons    []Button    `json:"buttons,omitempty"`
}

// Activity.Type values recognized by Discord.
const (
	TypePlaying   = 0
	TypeStreaming = 1
	TypeListening = 2
	TypeWatching  = 3
	TypeCompeting = 5
)

// Timestamps are UNIX epoch seconds. Start drives "elapsed", End drives
// "remaining"; both may be set when the range is known.
type Timestamps struct {
	Start int64 `json:"start,omitempty"`
	End   int64 `json:"end,omitempty"`
}

// Assets are the images the user uploaded in the Discord Developer Portal.
type Assets struct {
	LargeImage string `json:"large_image,omitempty"`
	LargeText  string `json:"large_text,omitempty"`
	LargeURL   string `json:"large_url,omitempty"`
	SmallImage string `json:"small_image,omitempty"`
	SmallText  string `json:"small_text,omitempty"`
	SmallURL   string `json:"small_url,omitempty"`
}

// Party describes a multi-user group. Size is [current, max].
type Party struct {
	Size []int `json:"size,omitempty"`
}

// Button is a clickable link rendered under the activity. Discord allows two.
type Button struct {
	Label string `json:"label"`
	URL   string `json:"url"`
}
