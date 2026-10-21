package patcher

import (
	"bytes"
)

// ByteMatcher проверяет соответствие байта
type ByteMatcher interface {
	Match(b byte) bool
}

type ExactByte byte

func (e ExactByte) Match(b byte) bool {
	return byte(e) == b
}

type AnyByte struct{}

func (AnyByte) Match(b byte) bool {
	return true
}

type ByteSet []byte

func (bs ByteSet) Match(b byte) bool {
	return bytes.IndexByte(bs, b) >= 0
}

type ByteRange struct {
	Min, Max byte
}

func (br ByteRange) Match(b byte) bool {
	return b >= br.Min && b <= br.Max
}

// Pattern представляет последовательность байтовых матчеров
type Pattern []ByteMatcher

// MatchAt проверяет, совпадает ли паттерн по смещению pos
func (p Pattern) MatchAt(data []byte, pos int) bool {
	if pos+len(p) > len(data) {
		return false
	}
	for i, m := range p {
		if !m.Match(data[pos+i]) {
			return false
		}
	}
	return true
}

// FindAll ищет все вхождения паттерна в диапазоне [start, end)
func (p Pattern) FindAll(data []byte, start, end int64) []int64 {
	var matches []int64
	if len(p) == 0 || start < 0 || end > int64(len(data)) || start >= end {
		return nil
	}

	first, hasFirst := p[0].(ExactByte)
	chunk := data[start:end]
	pos := 0

	for pos+len(p) <= len(chunk) {
		if hasFirst {
			idx := bytes.IndexByte(chunk[pos:], byte(first))
			if idx < 0 {
				break
			}
			pos += idx
			if pos+len(p) > len(chunk) {
				break
			}
		}

		if p.MatchAt(chunk, pos) {
			matches = append(matches, start+int64(pos))
		}
		pos++
	}
	return matches
}
