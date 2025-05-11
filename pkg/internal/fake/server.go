// Package fake provides a fake server mocking the Loki Push API. It can be used with [httptest] to test the Loki logger
// client.
package fake

import (
	"io"
	"net/http"
	"net/http/httptest"
	"sync"

	"github.com/gogo/protobuf/proto"
	"github.com/grafana/loki/pkg/push"
	"github.com/klauspost/compress/snappy"
)

// PushPath is the same as in the client package but provided here to avoid circular dependencies.
const PushPath = "/loki/api/v1/push"

// Server is a [http.Handler] that mocks the Loki Push API. It stores all of the streams posted to it in memory. It can
// safely handle multiple concurrent requests.
type Server struct {
	streams []push.Stream
	lock    *sync.RWMutex
	// sendError is the count of errors to return from Push before succeeding. It is decremented each time Push is
	// called.
	sendError uint
}

// NewServer creates a new Server with the given sendError count. It is safe to call concurrently from multiple
// goroutines.
func NewServer(sendError uint) *Server {
	return &Server{
		streams:   []push.Stream{},
		lock:      &sync.RWMutex{},
		sendError: sendError,
	}
}

// Streams locks the server for reading and returns the streams that have been posted to it. It should be paird with a
// call to [Close] to unlock the server.
func (server *Server) Streams() []push.Stream {
	server.lock.RLock()

	return server.streams
}

// Close unlocks the server from reading.
func (server *Server) Close() {
	server.lock.RUnlock()
}

// Start starts the server and returns a [httptest.Server] that can be used to get the URL of the server. It should not
// be called multiple times.
func (server *Server) Start() *httptest.Server {
	testServer := httptest.NewServer(server)

	return testServer
}

var _ http.Handler = (*Server)(nil)

func (server *Server) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if request.URL.Path != PushPath {
		writer.WriteHeader(http.StatusNotFound)

		return
	}

	if request.Method != http.MethodPost {
		writer.WriteHeader(http.StatusMethodNotAllowed)
		writer.Header().Add("Allow", http.MethodPost)
		writer.Header().Add("Content-Type", "text/plain")
		_, _ = writer.Write([]byte("Method Not Allowed"))

		return
	}

	server.lock.Lock()
	defer server.lock.Unlock()

	if server.sendError > 0 {
		server.sendError--

		writeError(writer, "Internal Server Error")

		return
	}

	body, err := io.ReadAll(request.Body)
	if err != nil {
		writeError(writer, "Failed to read request body")

		return
	}

	decoded, err := snappy.Decode(nil, body)
	if err != nil {
		writeError(writer, "Failed to decode request body")

		return
	}

	pushRequest := push.PushRequest{}
	err = proto.Unmarshal(decoded, &pushRequest)

	if err != nil {
		writeError(writer, "Failed to unmarshal request body")

		return
	}

	server.streams = append(server.streams, pushRequest.Streams...)

	writer.WriteHeader(http.StatusNoContent)
}

func writeError(writer http.ResponseWriter, message string) {
	writer.Header().Add("Content-Type", "text/plain")
	writer.WriteHeader(http.StatusInternalServerError)
	_, _ = writer.Write([]byte(message))
}
