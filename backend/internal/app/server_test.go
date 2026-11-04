package app

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/abysstract/abysstract-admin/backend/adminapp"
	"github.com/abysstract/abysstract-admin/backend/adminconfig"
)

func TestApplicationMiddlewareCoversCoreAndCustomRoutes(t *testing.T) {
	var order []string
	routes := http.NewServeMux()
	routes.HandleFunc("GET /core", func(w http.ResponseWriter, r *http.Request) {
		order = append(order, "core")
		w.WriteHeader(http.StatusNoContent)
	})
	routes.HandleFunc("GET /custom", func(w http.ResponseWriter, _ *http.Request) {
		order = append(order, "custom")
		w.WriteHeader(http.StatusNoContent)
	})
	server := NewServer(adminconfig.Config{}, routes)
	inner := server.Handler
	server.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		order = append(order, "before")
		inner.ServeHTTP(w, r)
		order = append(order, "after")
	})
	for _, route := range []string{"core", "custom"} {
		order = nil
		response := httptest.NewRecorder()
		server.Handler.ServeHTTP(response, httptest.NewRequest("GET", "/"+route, nil))
		if response.Code != http.StatusNoContent || !reflect.DeepEqual(order, []string{"before", route, "after"}) {
			t.Fatalf("%s: %d %v", route, response.Code, order)
		}
	}
}

func TestServeDrainsRequestsOnCancellation(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	server := NewServer(adminconfig.Config{}, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		close(entered)
		<-release
		w.WriteHeader(http.StatusNoContent)
	}))
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- serve(ctx, server, listener, time.Second) }()
	responseDone := make(chan error, 1)
	go func() {
		response, err := http.Get("http://" + listener.Addr().String())
		if err == nil {
			response.Body.Close()
			if response.StatusCode != http.StatusNoContent {
				err = errors.New("request interrupted")
			}
		}
		responseDone <- err
	}()
	<-entered
	cancel()
	select {
	case err := <-done:
		t.Fatalf("closed with request active: %v", err)
	default:
	}
	close(release)
	if err := <-responseDone; err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestServeTimeoutClosesConnections(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	defer close(release)
	server := NewServer(adminconfig.Config{}, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { close(entered); <-release }))
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- serve(ctx, server, listener, 10*time.Millisecond) }()
	requestDone := make(chan error, 1)
	go func() {
		response, err := http.Get("http://" + listener.Addr().String())
		if err == nil {
			response.Body.Close()
		}
		requestDone <- err
	}()
	<-entered
	cancel()
	if err := <-done; !errors.Is(err, adminapp.ErrShutdownTimeout) {
		t.Fatalf("serve: %v", err)
	}
	if err := <-requestDone; err == nil {
		t.Fatal("active connection stayed open")
	}
}
