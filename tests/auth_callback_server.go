package tests

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"
)

type callbackResult struct {
	Code  string
	State string
	Error string
}

type callbackServer struct {
	Server      *http.Server
	Listener    net.Listener
	RedirectURL string
	Results     <-chan callbackResult
}

func startCallbackServer() (*callbackServer, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("start callback listener: %w", err)
	}

	results := make(chan callbackResult, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(
		writer http.ResponseWriter,
		request *http.Request,
	) {
		query := request.URL.Query()

		result := callbackResult{
			Code:  query.Get("code"),
			State: query.Get("state"),
			Error: query.Get("error"),
		}

		select {
		case results <- result:
		default:
		}

		writer.Header().Set("Content-Type", "text/plain")
		_, _ = writer.Write([]byte(
			"Authentication completed. This window can be closed.",
		))
	})

	server := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		err := server.Serve(listener)

		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			select {
			case results <- callbackResult{
				Error: err.Error(),
			}:
			default:
			}
		}
	}()

	return &callbackServer{
		Server:      server,
		Listener:    listener,
		RedirectURL: "http://" + listener.Addr().String() + "/callback",
		Results:     results,
	}, nil
}

func (server *callbackServer) Close() error {
	ctx, cancel := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)
	defer cancel()

	return server.Server.Shutdown(ctx)
}
