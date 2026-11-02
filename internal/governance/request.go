package governance

import (
	"bytes"
	"encoding/json"
	"io"
)

// DecodeRequest admits no agent-supplied grants, commands, resources or check results.
func DecodeRequest(data []byte) (Request, error) {
	if len(data) == 0 || len(data) > 16*1024 {
		return Request{}, ErrInvalid
	}
	fields, err := uniqueObject(data, []string{"id", "task_id", "session_id", "revision", "skill", "operation"})
	if err != nil {
		return Request{}, err
	}
	if _, err := uniqueObject(fields["skill"], []string{"id", "instructions_hash", "sop_hash", "policy_hash"}); err != nil {
		return Request{}, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var req Request
	if decoder.Decode(&req) != nil {
		return Request{}, ErrInvalid
	}
	if !keyPattern.MatchString(req.ID) || !keyPattern.MatchString(req.TaskID) || !keyPattern.MatchString(req.SessionID) || !keyPattern.MatchString(req.Revision) || !keyPattern.MatchString(req.Operation) || !req.Skill.valid() {
		return Request{}, ErrInvalid
	}
	return req, nil
}

// Reject duplicate and case-aliased keys before typed decoding, so reviewers and
// dispatch never interpret different fields from an ambiguous approval request.
func uniqueObject(data []byte, allowed []string) (map[string]json.RawMessage, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	start, err := decoder.Token()
	if err != nil || start != json.Delim('{') {
		return nil, ErrInvalid
	}
	fields := make(map[string]json.RawMessage, len(allowed))
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return nil, ErrInvalid
		}
		key, ok := token.(string)
		if !ok {
			return nil, ErrInvalid
		}
		known := false
		for _, name := range allowed {
			if key == name {
				known = true
				break
			}
		}
		if !known {
			return nil, ErrInvalid
		}
		if _, duplicate := fields[key]; duplicate {
			return nil, ErrInvalid
		}
		var value json.RawMessage
		if decoder.Decode(&value) != nil {
			return nil, ErrInvalid
		}
		fields[key] = value
	}
	if end, err := decoder.Token(); err != nil || end != json.Delim('}') {
		return nil, ErrInvalid
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		return nil, ErrInvalid
	}
	return fields, nil
}
