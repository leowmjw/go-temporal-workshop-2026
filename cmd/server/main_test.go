package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHomeAndHealth(t *testing.T) {
	t.Parallel()
	s := &server{namespace: "default", taskQueue: "workshop-main"}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.home)
	mux.HandleFunc("GET /health", s.health)

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	require.Equal(t, http.StatusOK, response.Code)
	require.Contains(t, response.Header().Get("Content-Type"), "text/html")

	request = httptest.NewRequest(http.MethodGet, "/health", nil)
	response = httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	require.Equal(t, http.StatusOK, response.Code)
	require.Contains(t, response.Body.String(), `"temporal":false`)
}

func TestScenarioNavigation(t *testing.T) {
	t.Parallel()
	s := &server{}
	request := httptest.NewRequest(http.MethodGet, "/scenario/update-with-start", nil)
	request.SetPathValue("name", "update-with-start")
	response := httptest.NewRecorder()
	s.scenario(response, request)
	require.Equal(t, http.StatusOK, response.Code)
	require.Contains(t, response.Body.String(), "Update-With-Start")
	require.Contains(t, response.Body.String(), "Apply +1 update")
}
