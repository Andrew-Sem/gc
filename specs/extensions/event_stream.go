package extensions

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing/synctest"
	"time"

	"github.com/Andrew-Sem/gc/framework/journal"
	"github.com/Andrew-Sem/gc/framework/spec"
	"github.com/Andrew-Sem/gc/internal/web"
	"github.com/Andrew-Sem/gc/specs/contract"
)

const StreamSignal = "data: changed\n\n"

type streamRecorder struct {
	mu      sync.Mutex
	header  http.Header
	status  int
	body    bytes.Buffer
	flushes int
	hold    chan struct{}
}

func (r *streamRecorder) Header() http.Header { return r.header }

func (r *streamRecorder) WriteHeader(status int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.status == 0 {
		r.status = status
	}
}

func (r *streamRecorder) Write(data []byte) (int, error) {
	r.WriteHeader(http.StatusOK)
	r.mu.Lock()
	defer r.mu.Unlock()
	n, err := r.body.Write(data)
	if r.hold != nil {
		hold := r.hold
		r.hold = nil
		r.mu.Unlock()
		<-hold
		r.mu.Lock()
	}
	return n, err
}

func (r *streamRecorder) Flush() {
	r.WriteHeader(http.StatusOK)
	r.mu.Lock()
	defer r.mu.Unlock()
	r.flushes++
}

type openStream struct {
	recorder      *streamRecorder
	stop          context.CancelFunc
	ended         chan struct{}
	flushedAtOpen bool
	endedAtClose  bool
}

func (s *Scenario) openStream(j *journal.Journal[contract.Fact]) *openStream {
	request, err := spec.Get("/runs/events").HTTPRequest("http://gc.example.test")
	if err != nil {
		panic(err)
	}
	ctx, stop := context.WithCancel(context.Background())
	stream := &openStream{recorder: &streamRecorder{header: http.Header{}}, stop: stop, ended: make(chan struct{})}
	handler := s.app(web.New)(j)
	go func() {
		defer close(stream.ended)
		handler.ServeHTTP(stream.recorder, request.WithContext(ctx))
	}()
	synctest.Wait()
	stream.recorder.mu.Lock()
	defer stream.recorder.mu.Unlock()
	stream.flushedAtOpen = stream.recorder.flushes > 0
	s.streams = append(s.streams, stream)
	return stream
}

func (s *Scenario) closeStreams() {
	for _, stream := range s.streams {
		stream.stop()
	}
	synctest.Wait()
	for _, stream := range s.streams {
		select {
		case <-stream.ended:
			stream.endedAtClose = true
		default:
		}
	}
}

func (s *Scenario) WhenEventStreamsOpen(count int, writes ...[]contract.Fact) *Scenario {
	s.WhenAction(func(j *journal.Journal[contract.Fact]) {
		for range count {
			s.openStream(j)
		}
		for _, facts := range writes {
			j.Append(facts...)
			synctest.Wait()
		}
		s.closeStreams()
	})
	return s
}

func (s *Scenario) WhenEventStreamBusyWhileWritten(first, second contract.Fact) *Scenario {
	s.WhenAction(func(j *journal.Journal[contract.Fact]) {
		stream := s.openStream(j)
		hold := make(chan struct{})
		stream.recorder.mu.Lock()
		stream.recorder.hold = hold
		stream.recorder.mu.Unlock()
		j.Append(first)
		synctest.Wait()
		j.Append(second)
		synctest.Wait()
		close(hold)
		synctest.Wait()
		s.closeStreams()
	})
	return s
}

func Signals(count int) string {
	return strings.Repeat(StreamSignal, count)
}

type streamProblem string

func (p streamProblem) String() string {
	return string(p)
}

func (s *Scenario) StreamsSent(streams int, text string) func() spec.Problem {
	return func() spec.Problem {
		if len(s.streams) != streams {
			return streamProblem(fmt.Sprintf("expected %d streams, got %d", streams, len(s.streams)))
		}
		for i, stream := range s.streams {
			if got := stream.recorder.body.String(); got != text {
				return streamProblem(fmt.Sprintf("stream %d sent %q, want %q", i, got, text))
			}
		}
		return nil
	}
}

func (s *Scenario) StreamOpenedAtOnce() func() spec.Problem {
	return func() spec.Problem {
		for i, stream := range s.streams {
			switch got := stream.recorder; {
			case got.status != http.StatusOK:
				return streamProblem(fmt.Sprintf("stream %d answered %d, want 200", i, got.status))
			case got.header.Get("Content-Type") != "text/event-stream":
				return streamProblem(fmt.Sprintf("stream %d has Content-Type %q", i, got.header.Get("Content-Type")))
			case got.header.Get("Cache-Control") != "no-store":
				return streamProblem(fmt.Sprintf("stream %d has Cache-Control %q", i, got.header.Get("Cache-Control")))
			case !stream.flushedAtOpen:
				return streamProblem(fmt.Sprintf("stream %d did not send its headers at once", i))
			}
		}
		return nil
	}
}

func (s *Scenario) StreamsEnded() func() spec.Problem {
	return func() spec.Problem {
		for i, stream := range s.streams {
			if !stream.endedAtClose {
				return streamProblem(fmt.Sprintf("stream %d did not end when the client disconnected", i))
			}
		}
		return nil
	}
}

type launchStream struct {
	expected string
	request  *spec.Request
	stop     context.CancelFunc
	mu       sync.Mutex
	body     bytes.Buffer
}

func (l *Launch) WhenEventStreamOpen() *Launch {
	l.stream = &launchStream{}
	l.whenCalls++
	return l
}

func (l *Launch) WhenEventStreamOpenDuring(r spec.Request) *Launch {
	l.stream = &launchStream{request: &r}
	l.whenCalls++
	return l
}

func (l *Launch) ExpectStreamSent(text string) *Launch {
	l.stream.expected = text
	return l
}

func (l *Launch) watchStream() []spec.Problem {
	request, err := spec.Get("/runs/events").HTTPRequest("http://" + l.addr)
	if err != nil {
		return []spec.Problem{launchProblem(fmt.Sprintf("cannot build the stream request: %v", err))}
	}
	ctx, stop := context.WithCancel(context.Background())
	l.stream.stop = stop
	opening := time.AfterFunc(startupLimit, stop)
	response, err := http.DefaultClient.Do(request.WithContext(ctx))
	opening.Stop()
	if err != nil {
		return []spec.Problem{launchProblem(fmt.Sprintf("cannot open the stream: %v", err))}
	}
	go func() {
		defer response.Body.Close()
		buffer := make([]byte, 256)
		for {
			n, err := response.Body.Read(buffer)
			l.stream.mu.Lock()
			l.stream.body.Write(buffer[:n])
			l.stream.mu.Unlock()
			if err != nil {
				return
			}
		}
	}()
	if l.stream.request != nil {
		if status, err := l.do(l.addr, *l.stream.request); err != nil || status != http.StatusOK {
			return []spec.Problem{launchProblem(fmt.Sprintf("the request during the stream answered %d (%v)", status, err))}
		}
	}
	return l.waitForStream()
}

func (l *Launch) waitForStream() []spec.Problem {
	if l.stream.expected == "" {
		return nil
	}
	deadline := time.Now().Add(startupLimit)
	for time.Now().Before(deadline) {
		if l.streamBody() == l.stream.expected {
			return nil
		}
		time.Sleep(10 * time.Millisecond)
	}
	return []spec.Problem{launchProblem(fmt.Sprintf("the stream sent %q, want %q", l.streamBody(), l.stream.expected))}
}

func (l *Launch) streamBody() string {
	l.stream.mu.Lock()
	defer l.stream.mu.Unlock()
	return l.stream.body.String()
}

func (l *Launch) closeStream() {
	if l.stream != nil && l.stream.stop != nil {
		l.stream.stop()
	}
}
