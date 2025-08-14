package owctrl

import (
	"encoding/json"
	"time"
)

type UnixTime time.Time

func (u *UnixTime) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err == nil && s != "" {
		t, err := time.Parse(time.RFC3339Nano, s)
		if err != nil {
			return err
		}
		*u = UnixTime(t)
		return nil
	}
	var ts int64
	if err := json.Unmarshal(b, &ts); err == nil && ts > 0 {
		*u = UnixTime(time.Unix(ts, 0).UTC())
		return nil
	}
	*u = UnixTime(time.Now().UTC())
	return nil
}

type AdoptRequest struct {
	UUID        string         `json:"uuid"`
	Fingerprint string         `json:"fingerprint"`
	Metadata    map[string]any `json:"metadata"`
}

type AdoptResponse struct {
	DeviceID string `json:"device_id"`
	Next     string `json:"next"`
}

type DeviceDTO struct {
	ID   string `json:"id"`
	UUID string `json:"uuid"`
	Name string `json:"name,omitempty"`
}

type ConfigResponse struct {
	NetJSON  json.RawMessage `json:"netjson"`
	Version  int             `json:"version"`
	Checksum string          `json:"checksum"`
}

type AckRequest struct {
	Version   int      `json:"version"`
	Checksum  string   `json:"checksum"`
	AppliedAt UnixTime `json:"applied_at"`
	Status    string   `json:"status"`
}
