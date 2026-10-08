package recorder

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// Load rebuilds a finished, read-only Recorder from a JSONL event log written
// by a previous session. It is the basis for replaying past sessions in the
// cockpit.
func Load(r io.Reader) (*Recorder, error) {
	rec := &Recorder{closed: true}
	br := bufio.NewReader(r)
	for lineNo := 1; ; lineNo++ {
		line, err := br.ReadBytes('\n')
		if len(line) > 1 || (len(line) == 1 && line[0] != '\n') {
			var e Event
			if jerr := json.Unmarshal(line, &e); jerr != nil {
				return nil, fmt.Errorf("line %d: %w", lineNo, jerr)
			}
			rec.events = append(rec.events, e)
			if e.Seq > rec.seq {
				rec.seq = e.Seq
			}
			rec.session.ID = e.SessionID
			switch e.Type {
			case EventSessionStarted:
				rec.session.StartedAt = e.Timestamp
			case EventSessionCompleted:
				rec.session.EndedAt = e.Timestamp
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}
	}
	if len(rec.events) == 0 {
		return nil, errors.New("empty session log")
	}
	return rec, nil
}
