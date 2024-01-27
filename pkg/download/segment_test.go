package download

import (
	"io"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewSegment(t *testing.T) {
	t.Run("NewSegment", func(t *testing.T) {
		fileWriter, err := NewFileWriter("/tmp/dl/segments", "segment1.txt")
		assert.NoError(t, err)

		defer t.Cleanup(func() {
			fileWriter.Close()
			os.Remove("/tmp/dl/segments/segment1.txt")
		})

		segment, err := NewSegment(SegmentParams{
			ID:             1,
			Start:          int64(0),
			End:            int64(10),
			MaxSegmentSize: int64(5),
			Writer:         fileWriter,
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
	})
	t.Run("Copy data into segment", func(t *testing.T) {
		fileWriter, err := NewFileWriter("/tmp/dl/segments", "segment2.txt")
		assert.NoError(t, err)

		defer t.Cleanup(func() {
			fileWriter.Close()
			os.Remove("/tmp/dl/segments/segment2.txt")
		})

		segment, err := NewSegment(SegmentParams{
			ID:             1,
			Start:          int64(0),
			End:            int64(10),
			MaxSegmentSize: int64(5),
			Writer:         fileWriter,
		})
		if assert.NoError(t, err) {
			_, err = io.Copy(segment, strings.NewReader("abcdef"))
			_, err = fileWriter.Seek(0, io.SeekStart)
			content, err := io.ReadAll(fileWriter)
			assert.NoError(t, err)
			assert.Equal(t, "abcdef", string(content))
		}
	})
	t.Run("segment.setDone", func(t *testing.T) {
		fileWriter, err := NewFileWriter("/tmp/dl/segments", "segment3.txt")
		assert.NoError(t, err)

		defer t.Cleanup(func() {
			fileWriter.Close()
			os.Remove("/tmp/dl/segments/segment3.txt")
		})

		segment, err := NewSegment(SegmentParams{
			ID:             1,
			Start:          int64(0),
			End:            int64(10),
			MaxSegmentSize: int64(5),
			Writer:         fileWriter,
		})
		if assert.NoError(t, err) {
			_, err := io.Copy(segment, strings.NewReader("abcdefgh"))
			assert.NoError(t, err)

			segment.setDone(true)

			_, err = fileWriter.Seek(0, io.SeekStart)
			content, err := io.ReadAll(fileWriter)
			assert.NoError(t, err)
			assert.Equal(t, "abcdefgh", string(content))
		}
	})
	t.Run("ReadFrom", func(t *testing.T) {
		fileWriter, err := NewFileWriter("/tmp/dl/segments", "segment4.txt")
		assert.NoError(t, err)

		defer t.Cleanup(func() {
			fileWriter.Close()
			os.Remove("/tmp/dl/segments/segment3.txt")
		})

		segment, err := NewSegment(SegmentParams{
			ID:             1,
			Start:          int64(0),
			End:            int64(10),
			MaxSegmentSize: int64(5),
			Writer:         fileWriter,
		})
		assert.NoError(t, err)
		assert.Equal(t, true, segment.resumable)

		src1 := strings.NewReader("one")
		n, err := segment.ReadFrom(src1)
		assert.NoError(t, err)
		assert.Equal(t, int64(3), n)
	})
}
