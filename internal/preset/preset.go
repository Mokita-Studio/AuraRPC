// Package preset defines the Preset model, its JSON persistence and the
// validation rules for the Discord Rich Presence payload.
package preset

import (
	"crypto/rand"
	"fmt"
	"time"
)

// ActivityType is the verb shown before the name ("Playing X",
// "Listening to X", ...).
type ActivityType string

const (
	TypePlaying   ActivityType = "playing"
	TypeStreaming ActivityType = "streaming"
	TypeListening ActivityType = "listening"
	TypeWatching  ActivityType = "watching"
	TypeCompeting ActivityType = "competing"
)

// DisplayField selects which preset field (Name / Details / State) acts
// as the main title of the activity.
type DisplayField string

const (
	DisplayName    DisplayField = "name"
	DisplayDetails DisplayField = "details"
	DisplayState   DisplayField = "state"
)

// TimeMode controls how the activity's start timestamp is computed.
type TimeMode string

const (
	TimeNone          TimeMode = "none"
	TimeSinceConnect  TimeMode = "sinceConnect"
	TimeSincePresence TimeMode = "sincePresence"
	TimeSinceStart    TimeMode = "sinceStart"
	TimeLocal         TimeMode = "local"
	TimeCustom        TimeMode = "custom"
)

// Preset is a user-saved Rich Presence configuration.
type Preset struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	ClientID string       `json:"client_id"`
	Type     ActivityType `json:"type,omitempty"`
	Display  DisplayField `json:"display,omitempty"`

	AppName    string `json:"app_name,omitempty"`
	Details    string `json:"details,omitempty"`
	DetailsURL string `json:"details_url,omitempty"`
	State      string `json:"state,omitempty"`
	StateURL   string `json:"state_url,omitempty"`

	PartySize int `json:"party_size,omitempty"`
	PartyMax  int `json:"party_max,omitempty"`

	TimeMode  TimeMode `json:"time_mode,omitempty"`
	TimeStart string   `json:"time_start,omitempty"`
	TimeEndOn bool     `json:"time_end_on,omitempty"`
	TimeEnd   string   `json:"time_end,omitempty"`

	LargeKey  string `json:"large_key,omitempty"`
	LargeText string `json:"large_text,omitempty"`
	LargeURL  string `json:"large_url,omitempty"`
	SmallKey  string `json:"small_key,omitempty"`
	SmallText string `json:"small_text,omitempty"`
	SmallURL  string `json:"small_url,omitempty"`

	Btn1Text string `json:"btn1_text,omitempty"`
	Btn1URL  string `json:"btn1_url,omitempty"`
	Btn2Text string `json:"btn2_text,omitempty"`
	Btn2URL  string `json:"btn2_url,omitempty"`
}

// newID returns a UUID v4 generated with crypto/rand.
func newID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
