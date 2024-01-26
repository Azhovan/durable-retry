package download

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Segment represents a part of the file being downloaded.
// It contains the data and state for a specific portion (segment) of the file.
type Segment struct {
	SegmentParams
	// done indicates whether the download of this segment is complete.
	// It is set to true once the segment is successfully downloaded or if an irrecoverable error occurs.
	done bool

	// buffer is used to temporarily store data for this segment before writing to the file.
	// It helps in efficient writing by reducing the number of write operations.
	buffer *bufio.Writer

	// segmentFile is a temporary file used for storing the data of this segment.
	// It acts as a physical storage for the data being buffered.
	segmentFile *os.File
}

// SegmentParams represents the parameters for a specific segment of a file being downloaded.
// It contains information such as the segment ID, start and end byte offsets, errors encountered,
// maximum segment size, and the writer to which the segment data will be written.
type SegmentParams struct {
	// id uniquely identifies the segment.
	ID int

	// start indicates the starting byte of the segment within the file.
	// It marks the beginning of the portion of the file this segment is responsible for downloading.
	Start int64

	// end indicates the ending byte of the segment within the file.
	// This is the last byte in the range of this segment, inclusive.
	End int64

	// err captures any error encountered during the downloading of this segment.
	// A 'sticky' error remains associated with the segment to indicate issues like network failures or server errors.
	Err error

	// maxSegmentSize specifies the maximum size in bytes that this segment can handle.
	// It's used to control the volume of data fetched in a single request and can be adjusted for optimization.
	MaxSegmentSize int64

	// Writer is an io.Writer where the data for each segment is written and persisted.
	// This field allows the Segment to abstract the details of where and how the data is stored.
	// It could be a file, a buffer in memory, or any other type that implements the io.Writer interface.
	Writer io.Writer
}

// NewSegment creates a new instance of a Segment struct.
// It initializes a segment of a file to be downloaded, with specified start and end byte positions.
// The caller is responsible for managing the temporary file, including its deletion after the segment is processed.
func NewSegment(params SegmentParams) (*Segment, error) {
	return &Segment{
		SegmentParams: SegmentParams{
			ID:             params.ID,
			Start:          params.Start,
			End:            params.End,
			MaxSegmentSize: params.MaxSegmentSize,
		},
		done:        false,
		buffer:      bufio.NewWriterSize(params.Writer, int(params.MaxSegmentSize)),
		segmentFile: nil,
	}, nil
}

// NewFileWriter creates a new temporary file in the specified directory with the given name pattern.
// It returns a pointer to the created os.File and any error encountered during the file creation process.
func NewFileWriter(dir, name string) (*os.File, error) {
	err := os.MkdirAll(dir, 0o755)
	if err != nil {
		return nil, err
	}

	file, err := os.Create(fmt.Sprintf("%s/%s", strings.TrimSuffix(dir, string(filepath.Separator)), name))
	if err != nil {
		return nil, err
	}
	return file, nil
}

// Write writes the given data to the segment's buffer.
func (seg *Segment) Write(data []byte) (int, error) {
	return seg.buffer.Write(data)
}

// Flush flushes the segment's buffer, writing any buffered data to the underlying io.Writer.
func (seg *Segment) Flush() error {
	return seg.buffer.Flush()
}

// Close closes the segment, specifically its associated file.
func (seg *Segment) Close() error {
	return seg.segmentFile.Close()
}

// setErr sets the segment's error field if the provided error is non-nil.
func (seg *Segment) setErr(err error) {
	if err != nil {
		seg.Err = err
	}
}

// setDone marks the segment as done or not done based on the provided boolean value.
// If the boolean is false or if there is an existing error, it returns the error.
// Otherwise, it sets the segment as done and flushes its buffer.
// This function is used to finalize the segment's operations.
func (seg *Segment) setDone(b bool) error {
	if b == false || seg.Err != nil {
		return seg.Err
	}

	seg.done = b
	return seg.Flush()
}
