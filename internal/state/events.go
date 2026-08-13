package state

import (
	"encoding/json"
	"fmt"
	"os"
)

func (s *Store) appendEventLocked(ev Event) error {
	path, err := s.eventsPath()
	if err != nil {
		return err
	}
	if ev.SchemaVersion == 0 {
		ev.SchemaVersion = SchemaVersion
	}
	if ev.Timestamp == "" {
		ev.Timestamp = s.timestamp()
	}
	line, err := json.Marshal(ev)
	if err != nil {
		return fmt.Errorf("encode event: %w", err)
	}
	line = append(line, '\n')
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open event log: %w", err)
	}
	_, werr := f.Write(line)
	serr := f.Sync()
	cerr := f.Close()
	if werr != nil {
		return fmt.Errorf("append event log: %w", werr)
	}
	if serr != nil {
		return fmt.Errorf("flush event log: %w", serr)
	}
	if cerr != nil {
		return fmt.Errorf("close event log: %w", cerr)
	}
	return nil
}

// Payload marshals v as a journal payload. Secrets must not be passed in.
func Payload(v any) json.RawMessage {
	return payloadJSON(v)
}

func payloadJSON(v any) json.RawMessage {
	if v == nil {
		return nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	return b
}
