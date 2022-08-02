package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

type finalResponse struct {
	Error    string      `json:"error"`
	Response interface{} `json:"response,omitempty"`
}

func checkAuth(r *http.Request) error {
	if r.Header.Get("X-Auth") != "100500" {
		return ApiError{http.StatusForbidden, fmt.Errorf("unauthorized")}
	}
	return nil
}

func handleError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	if apiError, ok := err.(ApiError); ok {
		status = apiError.HTTPStatus
	}
	bs, e := json.Marshal(
		finalResponse{
			Error:    err.Error(),
			Response: nil,
		},
	)
	if e != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(status)
	w.Write(bs)
}

func extractValueFromRequest(r *http.Request, name string) (string, error) {
	var val string
	if r.Method == "GET" {
		val = r.URL.Query().Get(name)
	} else if r.Method == "POST" {
		val = r.FormValue(name)
	} else {
		return val, fmt.Errorf("Unsupported http method")
	}
	return val, nil
}

func (a *MyApi) handlerProfile(w http.ResponseWriter, r *http.Request) {
	var err error
	params, err := unpackProfileParams(r)
	if err != nil {
		handleError(w, err)
		return
	}
	resp, err := a.Profile(r.Context(), params)
	if err != nil {
		handleError(w, err)
		return
	}
	bs, err := json.Marshal(
		finalResponse{
			Response: resp,
		},
	)
	if err != nil {
		handleError(w, err)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write(bs)
}

func (a *MyApi) handlerCreate(w http.ResponseWriter, r *http.Request) {
	var err error
	// checking http method correctness
	if r.Method != "POST" {
		handleError(w, ApiError{http.StatusNotAcceptable, fmt.Errorf("bad method")})
		return
	}
	// checking authentication
	err = checkAuth(r)
	if err != nil {
		handleError(w, err)
		return
	}
	params, err := unpackCreateParams(r)
	if err != nil {
		handleError(w, err)
		return
	}
	resp, err := a.Create(r.Context(), params)
	if err != nil {
		handleError(w, err)
		return
	}
	bs, err := json.Marshal(
		finalResponse{
			Response: resp,
		},
	)
	if err != nil {
		handleError(w, err)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write(bs)
}

func (a *OtherApi) handlerCreate(w http.ResponseWriter, r *http.Request) {
	var err error
	// checking http method correctness
	if r.Method != "POST" {
		handleError(w, ApiError{http.StatusNotAcceptable, fmt.Errorf("bad method")})
		return
	}
	// checking authentication
	err = checkAuth(r)
	if err != nil {
		handleError(w, err)
		return
	}
	params, err := unpackOtherCreateParams(r)
	if err != nil {
		handleError(w, err)
		return
	}
	resp, err := a.Create(r.Context(), params)
	if err != nil {
		handleError(w, err)
		return
	}
	bs, err := json.Marshal(
		finalResponse{
			Response: resp,
		},
	)
	if err != nil {
		handleError(w, err)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write(bs)
}

func unpackProfileParams(r *http.Request) (m ProfileParams, err error) {
	var val string
	val, err = extractValueFromRequest(r, "login")
	if err != nil {
		return m, ApiError{http.StatusBadRequest, err}
	}
	if val == "" {
		return m, ApiError{http.StatusBadRequest, fmt.Errorf("login must me not empty")}
	}
	m.Login = val
	return m, nil
}

func unpackCreateParams(r *http.Request) (m CreateParams, err error) {
	var val string
	val, err = extractValueFromRequest(r, "login")
	if err != nil {
		return m, ApiError{http.StatusBadRequest, err}
	}
	if val == "" {
		return m, ApiError{http.StatusBadRequest, fmt.Errorf("login must me not empty")}
	}
	m.Login = val
	if len(m.Login) < 10 {
		return m, ApiError{http.StatusBadRequest, fmt.Errorf("login len must be >= 10")}
	}
	val, err = extractValueFromRequest(r, "full_name")
	if err != nil {
		return m, ApiError{http.StatusBadRequest, err}
	}
	m.Name = val
	val, err = extractValueFromRequest(r, "status")
	if err != nil {
		return m, ApiError{http.StatusBadRequest, err}
	}
	if val == "" {
		val = "user"
	}
	switch val {
	case "user", "moderator", "admin":
	default:
		return m, ApiError{http.StatusBadRequest, fmt.Errorf("status must be one of [user, moderator, admin]")}
	}
	m.Status = val
	val, err = extractValueFromRequest(r, "age")
	if err != nil {
		return m, ApiError{http.StatusBadRequest, err}
	}
	var conversionErr error
	m.Age, conversionErr = strconv.Atoi(val)
	if conversionErr != nil {
		return m, ApiError{http.StatusBadRequest, fmt.Errorf("age must be int")}
	}
	if m.Age < 0 {
		return m, ApiError{http.StatusBadRequest, fmt.Errorf("age must be >= 0")}
	}
	if m.Age > 128 {
		return m, ApiError{http.StatusBadRequest, fmt.Errorf("age must be <= 128")}
	}
	return m, nil
}

func unpackOtherCreateParams(r *http.Request) (m OtherCreateParams, err error) {
	var val string
	val, err = extractValueFromRequest(r, "username")
	if err != nil {
		return m, ApiError{http.StatusBadRequest, err}
	}
	if val == "" {
		return m, ApiError{http.StatusBadRequest, fmt.Errorf("username must me not empty")}
	}
	m.Username = val
	if len(m.Username) < 3 {
		return m, ApiError{http.StatusBadRequest, fmt.Errorf("username len must be >= 3")}
	}
	val, err = extractValueFromRequest(r, "account_name")
	if err != nil {
		return m, ApiError{http.StatusBadRequest, err}
	}
	m.Name = val
	val, err = extractValueFromRequest(r, "class")
	if err != nil {
		return m, ApiError{http.StatusBadRequest, err}
	}
	if val == "" {
		val = "warrior"
	}
	switch val {
	case "warrior", "sorcerer", "rouge":
	default:
		return m, ApiError{http.StatusBadRequest, fmt.Errorf("class must be one of [warrior, sorcerer, rouge]")}
	}
	m.Class = val
	val, err = extractValueFromRequest(r, "level")
	if err != nil {
		return m, ApiError{http.StatusBadRequest, err}
	}
	var conversionErr error
	m.Level, conversionErr = strconv.Atoi(val)
	if conversionErr != nil {
		return m, ApiError{http.StatusBadRequest, fmt.Errorf("level must be int")}
	}
	if m.Level < 1 {
		return m, ApiError{http.StatusBadRequest, fmt.Errorf("level must be >= 1")}
	}
	if m.Level > 50 {
		return m, ApiError{http.StatusBadRequest, fmt.Errorf("level must be <= 50")}
	}
	return m, nil
}

func (h *OtherApi) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/user/create":
		h.handlerCreate(w, r)
	default:
		handleError(w, ApiError{http.StatusNotFound, fmt.Errorf("unknown method")})
	}
}

func (h *MyApi) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/user/profile":
		h.handlerProfile(w, r)
	case "/user/create":
		h.handlerCreate(w, r)
	default:
		handleError(w, ApiError{http.StatusNotFound, fmt.Errorf("unknown method")})
	}
}
