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
	segment, err := NewSegment(SegmentParams{
		ID:             1,
		Start:          int64(0),
		End:            int64(10),
		MaxSegmentSize: int64(5),
		Writer:         writer,
	})
	if assert.NoError(t, err) {
		assert.NotNil(t, segment)
		assert.Equal(t, 1, segment.ID)
		assert.Equal(t, int64(0), segment.Start)
		assert.Equal(t, int64(10), segment.End)
		assert.Equal(t, int64(5), segment.MaxSegmentSize)
		assert.False(t, segment.done)
		assert.Nil(t, segment.Err)
	}

	_, err = io.Copy(segment, strings.NewReader("abcde"))
	if assert.NoError(t, err) {
		segment.setDone(true)
		assert.Equal(t, "abcde", writer.String())
	}

}
