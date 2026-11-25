package appagent

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"unicode"
	"unicode/utf8"

	textunicode "golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
)

const (
	localReadBytes = 50 << 10
	localReadLines = 2000
)

var binaryFile = errors.New("BinaryFile")

type textEncoding uint8

const (
	plainUTF8 textEncoding = iota
	bomUTF8
	littleUTF16
	bigUTF16
)

func openText(source io.Reader) (io.Reader, textEncoding, error) {
	buffer := bufio.NewReaderSize(source, 4096)
	head, _ := buffer.Peek(3)
	switch {
	case strings.HasPrefix(string(head), "\xff\xfe"):
		return transform.NewReader(buffer, textunicode.UTF16(textunicode.LittleEndian, textunicode.ExpectBOM).NewDecoder()), littleUTF16, nil
	case strings.HasPrefix(string(head), "\xfe\xff"):
		return transform.NewReader(buffer, textunicode.UTF16(textunicode.BigEndian, textunicode.ExpectBOM).NewDecoder()), bigUTF16, nil
	case strings.HasPrefix(string(head), "\xef\xbb\xbf"):
		_, _ = buffer.Discard(3)
		return buffer, bomUTF8, nil
	}
	sample, _ := buffer.Peek(512)
	if len(sample) > 0 && !strings.HasPrefix(http.DetectContentType(sample), "text/") {
		mime := http.DetectContentType(sample)
		if mime != "application/octet-stream" || strings.ContainsRune(string(sample), 0) {
			return nil, plainUTF8, binaryFile
		}
	}
	return buffer, plainUTF8, nil
}

func (format textEncoding) encode(text string) ([]byte, error) {
	switch format {
	case bomUTF8:
		return []byte("\xef\xbb\xbf" + text), nil
	case littleUTF16:
		return textunicode.UTF16(textunicode.LittleEndian, textunicode.UseBOM).NewEncoder().Bytes([]byte(text))
	case bigUTF16:
		return textunicode.UTF16(textunicode.BigEndian, textunicode.UseBOM).NewEncoder().Bytes([]byte(text))
	default:
		return []byte(text), nil
	}
}

func textRune(r rune, n int) bool {
	return !(r == utf8.RuneError && n == 1) && (!unicode.IsControl(r) || r == '\r' || r == '\n' || r == '\t' || r == '\f')
}

type localReadInput struct {
	Path           string `json:"path" jsonschema:"required"`
	Offset         int    `json:"offset,omitempty" jsonschema:"description=First line number starting at 1"`
	Limit          int    `json:"limit,omitempty" jsonschema:"description=Number of lines to return (maximum 2000)"`
	LineByteOffset int    `json:"lineByteOffset,omitempty" jsonschema:"description=UTF-8 offset returned when continuing a long line"`
}

type localReadPage struct {
	Text               string `json:"text,omitempty"`
	Status             string `json:"status"`
	NextOffset         int    `json:"nextOffset,omitempty"`
	NextLineByteOffset int    `json:"nextLineByteOffset,omitempty"`
	Truncated          bool   `json:"truncated,omitempty"`
}

func readLocalPage(ctx context.Context, source io.Reader, input localReadInput, budget int) (localReadPage, error) {
	if input.Offset < 0 || input.Limit < 0 || input.LineByteOffset < 0 {
		return localReadPage{}, errors.New("ValidationError")
	}
	text, _, err := openText(source)
	if errors.Is(err, binaryFile) {
		return localReadPage{Status: "binary"}, nil
	}
	if err != nil {
		return localReadPage{}, err
	}
	reader := bufio.NewReader(text)
	start := max(1, input.Offset)
	limit := localReadLines
	if input.Limit > 0 {
		limit = min(limit, input.Limit)
	}
	var body strings.Builder
	var byteInLine, used int
	line, lines := 1, 0
	prefixNeeded := true
	result := localReadPage{Status: "text"}
	for {
		if err := ctx.Err(); err != nil {
			return localReadPage{}, err
		}
		r, size, err := reader.ReadRune()
		if err == io.EOF {
			break
		}
		if err != nil {
			return localReadPage{}, err
		}
		if !textRune(r, size) {
			return localReadPage{Status: "binary"}, nil
		}
		take := line >= start && (line != start || byteInLine >= input.LineByteOffset)
		if line == start && byteInLine < input.LineByteOffset && (r == '\n' || byteInLine+size > input.LineByteOffset) {
			return localReadPage{}, errors.New("ValidationError")
		}
		if take {
			var prefix string
			if prefixNeeded {
				prefix = fmt.Sprintf("%d\t", line)
			}
			encoded, _ := json.Marshal(string(r))
			cost := len(prefix) + len(encoded) - 2
			if prefixNeeded {
				cost++ // JSON escapes the tab after the line number.
			}
			if (prefixNeeded && lines == limit) || used+cost > budget-256 {
				result.Truncated, result.NextOffset, result.NextLineByteOffset = true, line, byteInLine
				break
			}
			if prefixNeeded {
				body.WriteString(prefix)
				lines++
				prefixNeeded = false
			}
			used += cost
			body.WriteRune(r)
		}
		if r == '\n' {
			line++
			byteInLine = 0
			prefixNeeded = true
		} else {
			byteInLine += size
		}
	}
	result.Text = body.String()
	return result, nil
}
