package patcher

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"regexp"

	bin "github.com/tessych/agy-patcher-bytessych/pkg/binary"
)

// Ошибки состояний сигнатур
var (
	ErrNotFound  = errors.New("signature not found")
	ErrAmbiguous = errors.New("ambiguous signatures (duplicate or mixed matches)")
)

// GateStatus представляет текущий статус файла
type GateStatus string

const (
	StatusUnpatched GateStatus = "unpatched"
	StatusPatched   GateStatus = "patched"
	StatusUnknown   GateStatus = "unknown"
)

// ContextValidator валидирует байты вокруг совпадения (например, ARM64 context check)
type ContextValidator func(data []byte, matchStart, matchEnd int, rangeStart, rangeEnd int64) bool

// Gate представляет одну ассемблерную сигнатуру для проверки и замены
type Gate struct {
	Original Pattern
	Patched  Pattern
	Fix      []byte
	Offset   int
	Desc     string
	Arch     string // "x64", "arm64", "all"
	Accept   ContextValidator
}

// MultiGate объединяет несколько сигнатур для разных архитектур
type MultiGate struct {
	Gates []*Gate
	Desc  string
}

// MatchInfo содержит результат успешного поиска сигнатуры
type MatchInfo struct {
	State  GateStatus
	Offset int64
	Gate   *Gate
}

func findPatternMatches(pat Pattern, data []byte, ranges []bin.Range, accept ContextValidator) []int64 {
	var matches []int64
	for _, r := range ranges {
		locs := pat.FindAll(data, r.Start, r.End)
		for _, loc := range locs {
			absStart := int(loc)
			absEnd := absStart + len(pat)
			if accept != nil && !accept(data, absStart, absEnd, r.Start, r.End) {
				continue
			}
			matches = append(matches, loc)
		}
	}
	return matches
}

// Resolve находит единственный гейт для заданной архитектуры
func (g *Gate) Resolve(data []byte, ranges []bin.Range, arch string) (*MatchInfo, error) {
	if g.Arch != bin.ArchAll && arch != "" && g.Arch != arch {
		return nil, ErrNotFound
	}

	origMatches := findPatternMatches(g.Original, data, ranges, g.Accept)
	patchMatches := findPatternMatches(g.Patched, data, ranges, g.Accept)

	total := len(origMatches) + len(patchMatches)
	if total == 0 {
		return nil, ErrNotFound
	}
	if total > 1 {
		return nil, fmt.Errorf("%w: found %d matches for %s", ErrAmbiguous, total, g.Desc)
	}

	if len(origMatches) == 1 {
		return &MatchInfo{
			State:  StatusUnpatched,
			Offset: origMatches[0] + int64(g.Offset),
			Gate:   g,
		}, nil
	}

	return &MatchInfo{
		State:  StatusPatched,
		Offset: patchMatches[0] + int64(g.Offset),
		Gate:   g,
	}, nil
}

// Resolve ищет подходящий гейт среди списка в MultiGate
func (m *MultiGate) Resolve(data []byte, ranges []bin.Range, arch string) (*MatchInfo, error) {
	var results []*MatchInfo

	for _, g := range m.Gates {
		info, err := g.Resolve(data, ranges, arch)
		if err == nil {
			results = append(results, info)
		} else if !errors.Is(err, ErrNotFound) {
			return nil, err
		}
	}

	if len(results) == 0 {
		return nil, ErrNotFound
	}
	if len(results) > 1 {
		return nil, fmt.Errorf("%w: multiple architecture gates matched for %s", ErrAmbiguous, m.Desc)
	}

	return results[0], nil
}

// ARM64 Context Helper: проверяет cbnz x1 / cbz x0 за 8 байт до гейта
func CLIARM64Context(data []byte, matchStart, matchEnd int, rangeStart, rangeEnd int64) bool {
	if int64(matchStart-8) < rangeStart {
		return false
	}
	cbnzX1 := binary.LittleEndian.Uint32(data[matchStart-8 : matchStart-4])
	cbzX0 := binary.LittleEndian.Uint32(data[matchStart-4 : matchStart])

	return (cbnzX1&0xff00001f) == 0xb5000001 &&
		(cbzX0&0xff00001f) == 0xb4000000
}

// IDE Context: замена поля isGoogleInternal на true
var (
	IDERegex     = regexp.MustCompile(`(resetIsTierGCPTos\(\),)this\.[A-Za-z_$0-9]+\.isGoogleInternal`)
	IDEDoneRegex = regexp.MustCompile(`resetIsTierGCPTos\(\),true`)
	IDEFixBytes  = []byte("resetIsTierGCPTos(),true")
)

// ResolveIDE ищет гейт в файле main.js
func ResolveIDE(data []byte) (*MatchInfo, error) {
	origLocs := IDERegex.FindAllIndex(data, -1)
	patchLocs := IDEDoneRegex.FindAllIndex(data, -1)

	total := len(origLocs) + len(patchLocs)
	if total == 0 {
		return nil, ErrNotFound
	}
	if total > 1 {
		return nil, fmt.Errorf("%w: found %d matches in IDE main.js", ErrAmbiguous, total)
	}

	gate := &Gate{
		Fix:    IDEFixBytes,
		Offset: 0,
		Desc:   "IDE auth-gate",
		Arch:   bin.ArchAll,
	}

	if len(origLocs) == 1 {
		return &MatchInfo{
			State:  StatusUnpatched,
			Offset: int64(origLocs[0][0]),
			Gate:   gate,
		}, nil
	}

	return &MatchInfo{
		State:  StatusPatched,
		Offset: int64(patchLocs[0][0]),
		Gate:   gate,
	}, nil
}

func VerifyBytesEqual(a, b []byte) bool {
	return bytes.Equal(a, b)
}
