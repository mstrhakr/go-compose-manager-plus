package runtime

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"testing/iotest"
	"time"
)

func logFrame(stream byte, message string) []byte {
	header := make([]byte, 8)
	header[0] = stream
	binary.BigEndian.PutUint32(header[4:], uint32(len(message)))
	return append(header, message...)
}

func logClient(t *testing.T, tty bool, body []byte) *Client {
	t.Helper()
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodGet {
			t.Fatalf("non-read-only request: %s", r.Method)
		}
		id := strings.Repeat("a", 64)
		switch r.URL.Path {
		case "/v1.40/containers/" + id + "/json":
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(
				fmt.Sprintf(`{"Id":%q,"Config":{"Tty":%t},"State":{"Status":"running"}}`, id, tty)))}, nil
		case "/v1.40/containers/" + id + "/logs":
			q := r.URL.Query()
			if q.Get("stdout") != "true" || q.Get("stderr") != "true" || q.Get("timestamps") != "true" ||
				q.Get("tail") != "all" || q.Get("since") == "" {
				t.Fatalf("unexpected log request: %s", r.URL)
			}
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(body))}, nil
		default:
			t.Fatalf("unexpected request: %s", r.URL)
			return nil, errors.New("unexpected request")
		}
	})}
	return &Client{http: client, stream: client}
}

func TestReadContainerLogsPreservesFramedOutput(t *testing.T) {
	id := strings.Repeat("a", 64)
	at := time.Date(2026, 11, 23, 22, 0, 0, 123456789, time.UTC)
	prefix := at.Format(time.RFC3339Nano) + " "
	first, second := " {\"level\":\"info\",\"message\":\"hello\"}\r\n  multiline\n", "traceback\n"
	body := logFrame(1, prefix+first)
	body = append(body, logFrame(2, prefix+second)...)
	body = append(body, logFrame(1, prefix+"  unfinished\x00\xff")...)
	body = append(body, logFrame(1, prefix+"next without newline")...)
	client := logClient(t, false, body)
	transport := client.stream.Transport
	client.stream = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		response, err := transport.RoundTrip(r)
		if err == nil {
			response.Body = io.NopCloser(iotest.OneByteReader(response.Body))
		}
		return response, err
	})}
	var got []ContainerLog
	err := client.ReadContainerLogs(context.Background(), id,
		LogOptions{Since: at, Until: at.Add(time.Second)}, func(log ContainerLog) error {
			if log.ReceivedAt.IsZero() || log.ReceivedAt.After(time.Now()) || log.Truncated {
				t.Fatalf("invalid receipt metadata: %#v", log)
			}
			log.ReceivedAt = time.Time{}
			got = append(got, log)
			return nil
		})
	want := []ContainerLog{
		{ID: id, Time: at, Stream: LogStdout, Message: first},
		{ID: id, Time: at, Stream: LogStderr, Message: second},
		{ID: id, Time: at, Stream: LogStdout, Message: "  unfinished\x00\xff"},
		{ID: id, Time: at, Stream: LogStdout, Message: "next without newline"},
	}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("decoded logs = %#v, error %v", got, err)
	}
}

func TestReadContainerLogsTTYAndTruncation(t *testing.T) {
	at := time.Now().UTC().Add(-time.Minute)
	prefix := at.Format(time.RFC3339Nano) + " "
	oversized := strings.Repeat("x", MaxLogMessageBytes+8192)
	for _, tty := range []bool{false, true} {
		t.Run(fmt.Sprintf("tty=%t", tty), func(t *testing.T) {
			text := prefix + oversized + "\n" + prefix + "{\"valid\":true}\n"
			body := []byte(text)
			if !tty {
				body = append(logFrame(1, prefix+oversized+"\n"), logFrame(1, prefix+"{\"valid\":true}\n")...)
			}
			var logs []ContainerLog
			err := logClient(t, tty, body).ReadContainerLogs(context.Background(), strings.Repeat("a", 64),
				LogOptions{Since: at, Until: at.Add(time.Second)}, func(log ContainerLog) error {
					logs = append(logs, log)
					return nil
				})
			stream := LogStdout
			if tty {
				stream = LogTTY
			}
			if err != nil || len(logs) != 2 || !logs[0].Truncated ||
				logs[0].Message != oversized[:MaxLogMessageBytes] || logs[0].Stream != stream ||
				logs[1].Truncated || logs[1].Message != "{\"valid\":true}\n" {
				t.Fatalf("truncated/next records differ: count %d, error %v", len(logs), err)
			}
		})
	}
}

func TestReadContainerLogsUsesExactRangeAfterSecondGranularRequest(t *testing.T) {
	at := time.Date(2026, 11, 23, 22, 0, 0, 123, time.UTC)
	end := at.Add(time.Second)
	var body []byte
	for _, stamp := range []time.Time{at.Add(-time.Nanosecond), at, end.Add(-time.Nanosecond), end} {
		body = append(body, logFrame(1, stamp.Format(time.RFC3339Nano)+" line\n")...)
	}
	client := logClient(t, false, body)
	transport := client.stream.Transport
	client.stream = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Query().Get("since") != fmt.Sprint(at.Unix()) ||
			r.URL.Query().Get("until") != fmt.Sprint(end.Unix()+1) || r.URL.Query().Get("follow") != "false" {
			t.Fatalf("unbounded or rounded-down request: %s", r.URL)
		}
		return transport.RoundTrip(r)
	})}
	var times []time.Time
	err := client.ReadContainerLogs(context.Background(), strings.Repeat("a", 64), LogOptions{Since: at, Until: end},
		func(log ContainerLog) error {
			times = append(times, log.Time)
			return nil
		})
	if err != nil || !reflect.DeepEqual(times, []time.Time{at, end.Add(-time.Nanosecond)}) {
		t.Fatalf("exact range = %v, error %v", times, err)
	}
}

func TestReadContainerLogFailures(t *testing.T) {
	at := time.Now().UTC().Add(-time.Minute)
	badReserved := logFrame(1, "x")
	badReserved[1] = 1
	oversizedHeader := make([]byte, 8)
	oversizedHeader[0] = 1
	binary.BigEndian.PutUint32(oversizedHeader[4:], maxLogFrameBytes+1)
	for _, test := range []struct {
		name string
		body []byte
		want string
	}{
		{"unsupported stream", logFrame(0, "x"), "unsupported stream"},
		{"reserved bytes", badReserved, "reserved bytes"},
		{"oversized frame", oversizedHeader, "size limit"},
		{"partial header", []byte{1}, "header"},
		{"partial payload", logFrame(1, "hello")[:10], "payload"},
		{"missing timestamp", logFrame(1, "invalid\n"), "no timestamp"},
		{"invalid timestamp", logFrame(1, "invalid message\n"), "timestamp is invalid"},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := logClient(t, false, test.body).ReadContainerLogs(context.Background(), strings.Repeat("a", 64),
				LogOptions{Since: at, Until: at.Add(time.Second)}, func(ContainerLog) error {
					t.Fatal("malformed record reached consumer")
					return nil
				})
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v; want %q", err, test.want)
			}
		})
	}
	client := logClient(t, false, nil)
	for _, options := range []LogOptions{
		{}, {Since: at}, {Since: at, Until: at}, {Since: at, Until: at.Add(time.Hour + time.Second)},
		{Since: at, Until: at.Add(time.Second), Follow: true},
		{Since: time.Now().Add(time.Hour), Follow: true},
	} {
		if err := client.ReadContainerLogs(context.Background(), strings.Repeat("a", 64), options,
			func(ContainerLog) error { return nil }); err == nil {
			t.Fatalf("accepted invalid options: %#v", options)
		}
	}
	client.http = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(fmt.Sprintf(
			`{"Id":%q,"Config":{},"State":{"Status":"running"}}`, strings.Repeat("a", 64))))}, nil
	})}
	if err := client.ReadContainerLogs(context.Background(), strings.Repeat("a", 64),
		LogOptions{Since: at, Until: at.Add(time.Second)}, func(ContainerLog) error { return nil }); err == nil ||
		!strings.Contains(err.Error(), "TTY setting") {
		t.Fatalf("missing framing metadata = %v", err)
	}
}

func TestReadContainerLogsCallbackAndFollowEOF(t *testing.T) {
	at := time.Now().UTC().Add(-time.Minute)
	body := logFrame(1, at.Format(time.RFC3339Nano)+" line\n")
	client := logClient(t, false, body)
	callbackError := errors.New("storage unavailable")
	err := client.ReadContainerLogs(context.Background(), strings.Repeat("a", 64), LogOptions{Since: at, Follow: true},
		func(ContainerLog) error { return callbackError })
	if !errors.Is(err, callbackError) {
		t.Fatalf("consumer error = %v", err)
	}
	err = client.ReadContainerLogs(context.Background(), strings.Repeat("a", 64), LogOptions{Since: at, Follow: true},
		func(ContainerLog) error { return nil })
	if err == nil || !strings.Contains(err.Error(), "continuity is unavailable") {
		t.Fatalf("following EOF = %v", err)
	}
	err = client.ReadContainerLogs(context.Background(), strings.Repeat("a", 64),
		LogOptions{Since: time.Now().Add(-time.Hour), Follow: true}, func(ContainerLog) error { return nil })
	if err == nil || !strings.Contains(err.Error(), "continuity is unavailable") {
		t.Fatalf("full-hour follow was rejected because the clock advanced: %v", err)
	}
	client.stream = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusNotImplemented, Body: io.NopCloser(strings.NewReader("private output"))}, nil
	})}
	err = client.ReadContainerLogs(context.Background(), strings.Repeat("a", 64), LogOptions{Since: at, Follow: true},
		func(ContainerLog) error { return nil })
	if err == nil || !strings.Contains(err.Error(), "HTTP 501") || strings.Contains(err.Error(), "private output") {
		t.Fatalf("unavailable logging driver = %v", err)
	}
}

func TestReadContainerLogsCancelsIdleFollow(t *testing.T) {
	ready := make(chan struct{})
	id := strings.Repeat("a", 64)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/json") {
			fmt.Fprintf(w, `{"Id":%q,"Config":{"Tty":false},"State":{"Status":"running"}}`, id)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		close(ready)
		<-r.Context().Done()
	}))
	defer server.Close()
	destination, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	transport := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		copy := r.Clone(r.Context())
		copy.URL.Scheme, copy.URL.Host = destination.Scheme, destination.Host
		return http.DefaultTransport.RoundTrip(copy)
	})}
	client := &Client{http: transport, stream: transport}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- client.ReadContainerLogs(ctx, id, LogOptions{Since: time.Now().Add(-time.Minute), Follow: true},
			func(ContainerLog) error { return nil })
	}()
	select {
	case <-ready:
	case <-time.After(5 * time.Second):
		t.Fatal("log stream did not start")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("idle cancellation = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("idle log stream did not stop")
	}
}
