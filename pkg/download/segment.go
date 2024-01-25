package download

import (
	"bufio"
	"os"
)

// Segment represents a part of the file being downloaded.
// It contains the data and state for a specific portion (segment) of the file.
type Segment struct {
	// id uniquely identifies the segment.
	id int

	// start indicates the starting byte of the segment within the file.
	// It marks the beginning of the portion of the file this segment is responsible for downloading.
	start int64

	// end indicates the ending byte of the segment within the file.
	// This is the last byte in the range of this segment, inclusive.
	end int64

	// err captures any error encountered during the downloading of this segment.
	// A 'sticky' error remains associated with the segment to indicate issues like network failures or server errors.
	err error

	// maxSegmentSize specifies the maximum size in bytes that this segment can handle.
	// It's used to control the volume of data fetched in a single request and can be adjusted for optimization.
	maxSegmentSize int64

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

// NewSegment creates a new instance of a Segment struct.
// It initializes a segment of a file to be downloaded, with specified start and end byte positions.
// The caller is responsible for managing the temporary file, including its deletion after the segment is processed.
func NewSegment(id int, start, end, maxSegmentSize int64) (*Segment, error) {
	// Create a temporary file for the segment's data.
	file, err := os.CreateTemp("", "-")
	if err != nil {
		return nil, err
	}

	// The buffer size is set to half of maxSegmentSize, providing a balance between memory use and disk I/O.
	bufferSize := int(maxSegmentSize / 2)

	return &Segment{
		id:             id,
		start:          start,
		end:            end,
		maxSegmentSize: maxSegmentSize,
		buffer:         bufio.NewWriterSize(file, bufferSize),
	}, nil
}

func (seg *Segment) Write(data []byte) (int, error) {
	return seg.buffer.Write(data)
}

func (seg *Segment) Flush() error {
	return seg.buffer.Flush()
}

func (seg *Segment) Close() error {
	return seg.segmentFile.Close()
}

func (seg *Segment) setErr(err error) {
	if err != nil {
		seg.err = err
	}
}

func (seg *Segment) setDone(b bool) error {
	if b == false || seg.err != nil {
		return seg.err
	}

	seg.done = b
	return seg.Flush()
}
