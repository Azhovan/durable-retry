package download

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewSegment(t *testing.T) {
	writer := &bytes.Buffer{}

	id := 1
	maxSegmentSize := int64(5)
	start := int64(0)
	end := int64(10)
	segment, err := NewSegment(id, start, end, maxSegmentSize, writer)
	if assert.NoError(t, err) {
		assert.NotNil(t, segment)
		assert.Equal(t, 1, segment.id)
		assert.Equal(t, int64(0), segment.start)
		assert.Equal(t, int64(10), segment.end)
		assert.Equal(t, int64(5), segment.maxSegmentSize)
		assert.False(t, segment.done)
		assert.Nil(t, segment.err)
	}

	_, err = io.Copy(segment, strings.NewReader("abcde"))
	if assert.NoError(t, err) {
		segment.setDone(true)
		assert.Equal(t, "abcde", writer.String())
	}

}
