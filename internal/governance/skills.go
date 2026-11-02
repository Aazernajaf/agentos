package governance

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path"
	"regexp"
	"strings"
)

var keyPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.:-]{0,127}$`)
var skillPattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// Skill pins content, not a claim that a model read or understood it.
type Skill struct {
	ID               string `json:"id"`
	InstructionsHash string `json:"instructions_hash"`
	SOPHash          string `json:"sop_hash"`
	PolicyHash       string `json:"policy_hash"`
}

func (s Skill) valid() bool {
	if !skillPattern.MatchString(s.ID) || len(s.ID) > 64 {
		return false
	}
	for _, value := range []string{s.InstructionsHash, s.SOPHash, s.PolicyHash} {
		data, err := hex.DecodeString(value)
		if err != nil || len(data) != sha256.Size {
			return false
		}
	}
	return true
}

var ErrSkill = errors.New("skill_or_sop_mismatch")

// LoadSkill pins canonical instructions/SOP plus AGENTS.md and shared policy.
// os.Root confines these reads; handlers have their own execution trust boundary.
func LoadSkill(directory, id string) (Skill, error) {
	if !skillPattern.MatchString(id) || len(id) > 64 {
		return Skill{}, ErrSkill
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		return Skill{}, ErrSkill
	}
	defer root.Close()
	skill, err := readCanonical(root, path.Join(".agents/skills", id, "SKILL.md"))
	if err != nil {
		return Skill{}, err
	}
	sop, err := readCanonical(root, path.Join(".agents/skills", id, "SOP.md"))
	if err != nil {
		return Skill{}, err
	}
	var policyParts [][]byte
	for _, name := range []string{"AGENTS.md", "docs/agents/operating-policy.md"} {
		data, err := readCanonical(root, name)
		if err != nil {
			return Skill{}, err
		}
		policyParts = append(policyParts, data)
	}
	encoded, _ := json.Marshal(policyParts)
	digest := func(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }
	return Skill{ID: id, InstructionsHash: digest(skill), SOPHash: digest(sop), PolicyHash: digest(encoded)}, nil
}
func readCanonical(root *os.Root, name string) ([]byte, error) {
	// Canonical policy packages do not allow borrowed symlink files/directories.
	var current string
	parts := strings.Split(name, "/")
	for i, part := range parts {
		current = path.Join(current, part)
		info, err := root.Lstat(current)
		if err != nil || info.Mode()&os.ModeSymlink != 0 {
			return nil, ErrSkill
		}
		if i < len(parts)-1 && !info.IsDir() {
			return nil, ErrSkill
		}
		if i == len(parts)-1 && (!info.Mode().IsRegular() || info.Size() == 0 || info.Size() > 256*1024) {
			return nil, ErrSkill
		}
	}
	file, err := root.Open(name)
	if err != nil {
		return nil, ErrSkill
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return nil, ErrSkill
	}
	data, err := io.ReadAll(io.LimitReader(file, 256*1024+1))
	if err != nil || len(data) == 0 || len(data) > 256*1024 {
		return nil, ErrSkill
	}
	return data, nil
}
