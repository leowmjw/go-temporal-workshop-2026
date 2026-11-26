package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestStandalonePageRendersQuotePickerAndControls(t *testing.T) {
	t.Parallel()
	s := &server{}
	request := httptest.NewRequest(http.MethodGet, "/scenario/standalone", nil)
	request.SetPathValue("name", "standalone")
	response := httptest.NewRecorder()
	s.scenario(response, request)
	require.Equal(t, http.StatusOK, response.Code)

	body := response.Body.String()
	require.Contains(t, body, "Standalone Activities")
	require.Equal(t, 10, strings.Count(body, "<option value="), "every quote should be selectable")
	require.Contains(t, body, `data-bind="quoteId"`)
	for _, action := range []string{"pause", "unpause", "cancel", "terminate"} {
		require.Contains(t, body, "/api/forge/"+action)
	}
	require.Contains(t, body, "Talk is cheap. Show me the code.")
}

func TestForgeControlRejectsUnknownActionWithoutTemporal(t *testing.T) {
	t.Parallel()
	s := &server{}
	request := httptest.NewRequest(http.MethodPost, "/api/forge/bogus", strings.NewReader(`{"activityId":"x"}`))
	request.SetPathValue("action", "bogus")
	response := httptest.NewRecorder()
	s.forgeControl(response, request)
	require.Equal(t, http.StatusServiceUnavailable, response.Code)
}
