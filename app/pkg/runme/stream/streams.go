package stream

import (
	"context"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/jlewi/cloud-assistant/app/pkg/iam"
	"github.com/jlewi/cloud-assistant/app/pkg/logs"
	"github.com/jlewi/cloud-assistant/protos/gen/cassie"
	"github.com/pkg/errors"
	"google.golang.org/genproto/googleapis/rpc/code"
)

// Streams manages multiple websocket connections for a Runme execution (aka "run"). Each connection represents either:
// - A single Console DOM element
// - A client reconnection (e.g. when the client is disconnected and reconnects/resumes)
// These connections are multiplexed bidirectionally together to handle the execution flow.
type Streams struct {
	auth *iam.AuthContext

	// The known ID is the cell/block ID contained in requests. Once we have a known ID, we can reject requests with mismatched IDs.
	knownID string

	mu    sync.RWMutex
	conns map[string]*Connection

	authedSocketRequests chan *cassie.SocketRequest
}

// NewStreams creates a instance of Streams that manages multiple websocket connections attached to a muliplexed Runme execution.
func NewStreams(ctx context.Context, auth *iam.AuthContext, socketRequests chan *cassie.SocketRequest) *Streams {
	return &Streams{
		auth:                 auth,
		knownID:              "",
		conns:                make(map[string]*Connection, 1),
		authedSocketRequests: socketRequests,
	}
}

func (s *Streams) createStream(streamID string, sc *Connection) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.conns[streamID]; ok {
		return errors.New("connection already exists")
	}

	s.conns[streamID] = sc

	return nil
}

func (s *Streams) removeStream(ctx context.Context, streamID string) {
	log := logs.FromContextWithTrace(ctx)
	log.Info("Removing stream", "streamID", streamID)

	s.mu.Lock()
	defer s.mu.Unlock()

	sc, ok := s.conns[streamID]
	if !ok {
		log.Info("Stream not found", "streamID", streamID)
		return
	}

	delete(s.conns, streamID)
	_ = sc.Close()
}

func (s *Streams) close(ctx context.Context) {
	log := logs.FromContextWithTrace(ctx)

	s.mu.Lock()
	defer s.mu.Unlock()

	for streamID, conn := range s.conns {
		delete(s.conns, streamID)
		if err := conn.Close(); err != nil {
			log.Error(err, "Could not close websocket", "streamID", streamID)
		}
	}
}

func (s *Streams) receive(ctx context.Context, streamID string, runID string, sc *Connection) error {
	log := logs.FromContextWithTrace(ctx)

	for {
		log.Info("Reading socket requests", "streamID", streamID)
		req, err := sc.ReadSocketRequest(ctx)
		if err != nil {
			log.Error(err, "Could not read socket request")
			return err
		}

		log.Info("Received socket request", "streamID", streamID, "runID", req.GetRunId())

		// Return error to reject the connection if the socket request is not authorized.
		if err := s.auth.AuthorizeRequest(ctx, req); err != nil {
			log.Error(err, "Could not authorize request", "streamID", streamID, "runID", req.GetRunId())
			sc.ErrorMessage(ctx, code.Code_PERMISSION_DENIED, "Unauthorized request")
			return err
		}

		// Skip if request is explicitly ping-only.
		if req.GetPing() == nil && req.GetPayload() != nil {
			// Check if context runID matches the authorized one in the request.
			if req.GetRunId() != runID {
				log.Error(err, "RunID mismatch", "streamID", streamID, "runID", req.GetRunId(), "expectedRunID", runID)
				sc.ErrorMessage(ctx, code.Code_PERMISSION_DENIED, "RunID mismatch")
				return err
			}

			// Set the known ID if it is not already set.
			if s.knownID == "" {
				s.knownID = req.GetKnownId()
			}

			// Check if the knownID matches the one in the request.
			if req.GetKnownId() != s.knownID {
				log.Error(err, "KnownID mismatch", "streamID", streamID, "knownID", req.GetKnownId(), "expectedKnownID", s.knownID)
				sc.ErrorMessage(ctx, code.Code_PERMISSION_DENIED, "KnownID mismatch")
				return err
			}
		}

		// Handle protocol-level ping
		if req.GetPing() != nil {
			pong := &cassie.Pong{Timestamp: req.GetPing().GetTimestamp()}
			resp := &cassie.SocketResponse{Pong: pong}
			err := sc.WriteSocketResponse(ctx, resp)
			if err != nil {
				log.Error(err, "Could not send pong response")
			}
			continue // Do not forward ping to multiplexer
		}

		// Only authorized requests are forwarded to the multiplexer.
		s.authedSocketRequests <- req
	}
}

func (s *Streams) broadcast(ctx context.Context, responseData []byte) error {
	log := logs.FromContextWithTrace(ctx)
	s.mu.RLock()
	defer s.mu.RUnlock()

	for streamID, sc := range s.conns {
		log.Info("Sending response to stream", "streamID", streamID)
		err := sc.WriteMessage(websocket.TextMessage, responseData)
		if err != nil {
			log.Error(err, "Could not send message")
			return err
		}
	}

	return nil
}
