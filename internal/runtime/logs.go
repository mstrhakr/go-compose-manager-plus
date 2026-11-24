package runtime

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const MaxLogMessageBytes = 64 << 10
const maxLogFrameBytes = 1 << 20
const maxLogLineBytes = MaxLogMessageBytes + 64

type LogStream string

const (
	LogStdout LogStream = "stdout"
	LogStderr LogStream = "stderr"
	LogTTY    LogStream = "tty"
)

type ContainerLog struct {
	ID         string
	Time       time.Time
	ReceivedAt time.Time
	Stream     LogStream
	Message    string
	Truncated  bool
}

type LogOptions struct {
	Since  time.Time
	Until  time.Time
	Follow bool
}

func logTimeValid(at time.Time) bool {
	ns := at.UnixNano()
	return !at.IsZero() && ns > 0 && time.Unix(0, ns).Equal(at)
}

func (options LogOptions) validate(now time.Time) error {
	if !logTimeValid(options.Since) {
		return errors.New("Docker log start time is invalid")
	}
	if options.Follow && !options.Until.IsZero() {
		return errors.New("following Docker logs requires an open-ended range")
	}
	if options.Until.IsZero() {
		if !options.Follow || options.Since.After(now) {
			return errors.New("following Docker logs requires a past start time")
		}
	} else if !logTimeValid(options.Until) || !options.Until.After(options.Since) ||
		options.Until.Sub(options.Since) > time.Hour {
		return errors.New("Docker log range must be positive and at most one hour")
	}
	return nil
}

// ReadContainerLogs decodes read-only Docker output, not application log formats.
// EOF is successful only for a finite read. Reconnect/replay policy belongs to
// the caller, including a retained-window backfill start for live following;
// Docker supplies no stable log-record ID.
func (c *Client) ReadContainerLogs(ctx context.Context, id string, options LogOptions, consume func(ContainerLog) error) error {
	if consume == nil {
		return errors.New("container log callback is required")
	}
	if c.http == nil || c.stream == nil {
		return errors.New("Docker log clients are not configured")
	}
	if err := options.validate(time.Now()); err != nil {
		return err
	}
	inspection, err := c.InspectContainer(ctx, id)
	if err != nil {
		return fmt.Errorf("inspect Docker log stream: %w", err)
	}
	if inspection.TTY == nil {
		return errors.New("Docker inspection has no TTY setting for log decoding")
	}
	query := url.Values{
		"stdout": {"true"}, "stderr": {"true"}, "timestamps": {"true"},
		"follow": {strconv.FormatBool(options.Follow)}, "tail": {"all"},
		"since": {strconv.FormatInt(options.Since.Unix(), 10)},
	}
	if !options.Until.IsZero() {
		until := options.Until.Unix()
		if options.Until.Nanosecond() != 0 {
			until++
		}
		query.Set("until", strconv.FormatInt(until, 10))
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"http://docker/v1.40/containers/"+id+"/logs?"+query.Encode(), nil)
	if err != nil {
		return fmt.Errorf("create Docker log request: %w", err)
	}
	response, err := c.stream.Do(request)
	if err != nil {
		return fmt.Errorf("request Docker logs: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("Docker logs returned HTTP %d", response.StatusCode)
	}
	err = decodeContainerLogs(response.Body, *inspection.TTY, func(stream LogStream, line []byte, truncated bool) error {
		prefix, message, found := strings.Cut(string(line), " ")
		if !found {
			return errors.New("Docker log record has no timestamp")
		}
		at, err := time.Parse(time.RFC3339Nano, prefix)
		if err != nil || !logTimeValid(at) {
			return errors.New("Docker log timestamp is invalid")
		}
		if at.Before(options.Since) || (!options.Until.IsZero() && !at.Before(options.Until)) {
			return nil
		}
		if len(message) > MaxLogMessageBytes {
			message = message[:MaxLogMessageBytes]
			truncated = true
		}
		if err := consume(ContainerLog{
			ID: id, Time: at.UTC(), ReceivedAt: time.Now().UTC(), Stream: stream,
			Message: message, Truncated: truncated,
		}); err != nil {
			return fmt.Errorf("consume Docker log: %w", err)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("decode Docker logs: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("Docker log stream ended: %w", err)
	}
	if options.Follow {
		return errors.New("Docker log stream ended; continuity is unavailable")
	}
	return nil
}

type logLineBuffer struct {
	data      []byte
	truncated bool
}

func (line *logLineBuffer) feed(stream LogStream, data []byte, consume func(LogStream, []byte, bool) error) error {
	for len(data) > 0 {
		next := bytes.IndexByte(data, '\n')
		complete := next >= 0
		size := len(data)
		if complete {
			size = next + 1
		}
		keep := min(size, maxLogLineBytes-len(line.data))
		line.data = append(line.data, data[:keep]...)
		line.truncated = line.truncated || keep < size
		data = data[size:]
		if complete {
			if err := line.flush(stream, consume); err != nil {
				return err
			}
		}
	}
	return nil
}

func (line *logLineBuffer) flush(stream LogStream, consume func(LogStream, []byte, bool) error) error {
	if len(line.data) == 0 {
		return nil
	}
	if err := consume(stream, line.data, line.truncated); err != nil {
		return err
	}
	line.data, line.truncated = line.data[:0], false
	return nil
}

func decodeContainerLogs(reader io.Reader, tty bool, consume func(LogStream, []byte, bool) error) error {
	chunk := make([]byte, 4096)
	if tty {
		var line logLineBuffer
		for {
			n, err := reader.Read(chunk)
			if n > 0 {
				if feedErr := line.feed(LogTTY, chunk[:n], consume); feedErr != nil {
					return feedErr
				}
			}
			if errors.Is(err, io.EOF) {
				return line.flush(LogTTY, consume)
			}
			if err != nil {
				return fmt.Errorf("read TTY log stream: %w", err)
			}
		}
	}
	for {
		var header [8]byte
		_, err := io.ReadFull(reader, header[:])
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("read Docker log header: %w", err)
		}
		stream := LogStdout
		switch header[0] {
		case 1:
		case 2:
			stream = LogStderr
		default:
			return errors.New("Docker log frame has unsupported stream")
		}
		if header[1] != 0 || header[2] != 0 || header[3] != 0 {
			return errors.New("Docker log frame has invalid reserved bytes")
		}
		size := binary.BigEndian.Uint32(header[4:])
		if size > maxLogFrameBytes {
			return errors.New("Docker log frame exceeds size limit")
		}
		if size == 0 {
			continue
		}
		// Docker timestamps each framed record, even without a trailing newline.
		record := make([]byte, 0, min(int(size), maxLogLineBytes))
		truncated := false
		for size > 0 {
			count := min(int(size), len(chunk))
			if _, err := io.ReadFull(reader, chunk[:count]); err != nil {
				return fmt.Errorf("read Docker log payload: %w", err)
			}
			keep := min(count, maxLogLineBytes-len(record))
			record = append(record, chunk[:keep]...)
			truncated = truncated || keep < count
			size -= uint32(count)
		}
		if err := consume(stream, record, truncated); err != nil {
			return err
		}
	}
}
