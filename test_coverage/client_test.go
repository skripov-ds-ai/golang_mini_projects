package main

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/go-playground/assert/v2"
)

// код писать тут

func TestSearchClientTimeout(t *testing.T) {
	server := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				time.Sleep(client.Timeout + time.Second)
			},
		),
	)
	defer server.Close()

	req := SearchRequest{Limit: 0}
	client := &SearchClient{URL: server.URL}

	_, err := client.FindUsers(req)
	if err == nil {
		t.Errorf("Expected: error; received: none")
	}

	result := "timeout for limit=1&offset=0&order_by=0&order_field=&query="
	if errString := err.Error(); errString != result {
		t.Errorf("Expected: \"%s\"; received: \"%s\"", result, errString)
	}
}

func TestSearchClientError(t *testing.T) {
	req := SearchRequest{Limit: 0}
	client := &SearchClient{URL: ""}

	_, err := client.FindUsers(req)
	if err == nil {
		t.Errorf("Expected: error; received: none")
	}

	result := "unknown error Get \"?limit=1&offset=0&order_by=0&order_field=&query=\": unsupported protocol scheme \"\""
	if errString := err.Error(); errString != result {
		t.Errorf("Expected: \"%s\"; received: \"%s\"", result, errString)
	}
}

func TestSearchClientFindUsersLimitLessThan0(t *testing.T) {
	var serverCalled bool
	server := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				serverCalled = true
			},
		),
	)
	defer server.Close()

	req := SearchRequest{Limit: -1}
	client := &SearchClient{URL: server.URL}

	_, err := client.FindUsers(req)
	if err == nil {
		t.Errorf("Expected: error; received: none")
	}

	result := "limit must be > 0"
	if errString := err.Error(); errString != result {
		t.Errorf("Expected: \"%s\"; received: \"%s\"", result, errString)
	}

	if serverCalled {
		t.Errorf("Expected: the http request not to be made")
	}
}

func TestSearchClientFindUsersOffsetLessThan0(t *testing.T) {
	var serverCalled bool
	server := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				serverCalled = true
			},
		),
	)
	defer server.Close()

	req := SearchRequest{Offset: -1}
	client := &SearchClient{URL: server.URL}

	_, err := client.FindUsers(req)
	if err == nil {
		t.Errorf("Error was expected; none received")
	}

	result := "offset must be > 0"
	if errString := err.Error(); errString != result {
		t.Errorf("Expected: \"%s\"; received: \"%s\"", result, errString)
	}

	if serverCalled {
		t.Errorf("Expected: the http request not to be made")
	}
}

const (
	token       = "42"
	notToken    = "43"
	datasetPath = "./dataset.xml"
)

type XML struct {
	Root       xml.Name  `xml:"root"`
	Users      []userXML `xml:"row"`
	OrderField string
	Order      int64
}

type userXML struct {
	//Company       string `xml:"company"`
	//Email         string `xml:"email"`
	//Phone         string `xml:"phone"`
	//Address       string `xml:"address"`
	//Registered    string `xml:"registered"`
	//FavoriteFruit string `xml:"favoriteFruit"`
	//GUID          string `xml:"guid"`
	//IsActive      bool   `xml:"isActive"`
	//Balance       string `xml:"balance"`
	//Picture       string `xml:"picture"`
	//EyeColor      string `xml:"eyeColor"`
	ID        int    `xml:"id" json:"id"`
	Age       int    `xml:"age" json:"age"`
	About     string `xml:"about" json:"about"`
	Gender    string `xml:"gender" json:"gender"`
	FirstName string `xml:"first_name"`
	LastName  string `xml:"last_name"`
	Name      string `json:"name"`
}

func (us XML) Len() int {
	return len(us.Users)
}

func (us XML) Swap(i, j int) {
	us.Users[i], us.Users[j] = us.Users[j], us.Users[i]
}

func (us XML) Less(i, j int) bool {
	if us.OrderField == "id" {
		return us.Users[i].ID < us.Users[j].ID
	}
	if us.OrderField == "age" {
		return us.Users[i].Age < us.Users[j].Age
	}
	return us.Users[i].Name < us.Users[j].Name
}

func SearchServer(w http.ResponseWriter, r *http.Request) {
	accessToken := r.Header.Get("AccessToken")
	query := r.FormValue("query")
	orderField := r.FormValue("order_field")
	orderByString := r.FormValue("order_by")
	orderBy, err := strconv.ParseInt(orderByString, 10, 0)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	limitString := r.FormValue("limit")
	limit, err := strconv.ParseInt(limitString, 10, 0)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	offsetString := r.FormValue("offset")
	offset, err := strconv.ParseInt(offsetString, 10, 0)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	switch {
	case orderBy < -1:
		w.WriteHeader(http.StatusBadRequest)
		return
	case orderBy > 1:
		w.WriteHeader(http.StatusBadRequest)
		resp := SearchErrorResponse{Error: "unknown bad request error"}
		bs, err := json.Marshal(resp)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Write(bs)
		w.Header().Set("Content-Type", "application/json")
		return
	}
	switch orderField {
	case "Age", "Id", "Name", "":
		orderField = strings.ToLower(orderField)
	default:
		w.WriteHeader(http.StatusBadRequest)
		resp := SearchErrorResponse{Error: "ErrorBadOrderField"}
		bs, err := json.Marshal(resp)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(bs)
		return
	}

	if accessToken != token {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	file, err := os.Open(datasetPath)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer file.Close()

	var doc XML
	bs, err := ioutil.ReadAll(file)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	err = xml.Unmarshal(bs, &doc)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	doc.Order = orderBy
	doc.OrderField = orderField

	if query != "" {
		users := make([]userXML, 0)
		for _, user := range doc.Users {
			if strings.Contains(user.Name, query) || strings.Contains(user.About, query) {
				users = append(users, user)
			}
		}
		doc.Users = users
	}

	if doc.Order != 0 {
		sort.Sort(doc)
	}
	if len(doc.Users) != 0 {
		if int(offset) > len(doc.Users) {
			doc.Users = make([]userXML, 0)
		} else {
			doc.Users = doc.Users[offset : offset+limit]
		}
	}

	result, err := json.Marshal(doc.Users)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	w.Write(result)
}

func TestSearchClientFindUsersLimitIs24WrongToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(SearchServer))
	defer server.Close()

	req := SearchRequest{Offset: 0, Limit: 24, Query: ""}
	client := &SearchClient{URL: server.URL, AccessToken: notToken}

	_, err := client.FindUsers(req)
	if err == nil {
		t.Errorf("Error was expected; none received")
	}

	result := "Bad AccessToken"
	if errString := err.Error(); errString != result {
		t.Errorf("Expected: \"%s\"; received: \"%s\"", result, errString)
	}
}

func TestSearchClientFindUsersLimitIs26WrongToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(SearchServer))
	defer server.Close()

	req := SearchRequest{Offset: 0, Limit: 26, Query: ""}
	client := &SearchClient{URL: server.URL, AccessToken: notToken}

	_, err := client.FindUsers(req)
	if err == nil {
		t.Errorf("Error was expected; none received")
	}

	result := "Bad AccessToken"
	if errString := err.Error(); errString != result {
		t.Errorf("Expected: \"%s\"; received: \"%s\"", result, errString)
	}
}

func TestSearchClientFindUsersInternalServerError(t *testing.T) {
	server := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
		),
	)
	defer server.Close()

	req := SearchRequest{Offset: 0, Limit: 26, Query: ""}
	client := &SearchClient{URL: server.URL, AccessToken: notToken}

	_, err := client.FindUsers(req)
	if err == nil {
		t.Errorf("Error was expected; none received")
	}

	result := "SearchServer fatal error"
	if errString := err.Error(); errString != result {
		t.Errorf("Expected: \"%s\"; received: \"%s\"", result, errString)
	}
}

func TestSearchClientFindUsersUnknownBadRequestErrorOrderBy42(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(SearchServer))
	defer server.Close()

	req := SearchRequest{Offset: 0, Limit: 26, Query: "", OrderBy: 42}
	client := &SearchClient{URL: server.URL, AccessToken: token}

	_, err := client.FindUsers(req)
	if err == nil {
		t.Errorf("Error was expected; none received")
	}

	result := "unknown bad request error: unknown bad request error"
	if errString := err.Error(); errString != result {
		t.Errorf("Expected: \"%s\"; received: \"%s\"", result, errString)
	}
}

func TestSearchClientFindUsersUnknownBadRequestErrorOrderByMinus2(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(SearchServer))
	defer server.Close()

	req := SearchRequest{Offset: 0, Limit: 26, Query: "", OrderBy: -2}
	client := &SearchClient{URL: server.URL, AccessToken: token}

	_, err := client.FindUsers(req)
	if err == nil {
		t.Errorf("Error was expected; none received")
	}

	result := "cant unpack error json: unexpected end of JSON input"
	if errString := err.Error(); errString != result {
		t.Errorf("Expected: \"%s\"; received: \"%s\"", result, errString)
	}
}

func TestSearchClientFindUsersErrorBadOrderField(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(SearchServer))
	defer server.Close()

	req := SearchRequest{Offset: 0, Limit: 26, Query: "", OrderBy: 0, OrderField: "hahaha"}
	client := &SearchClient{URL: server.URL, AccessToken: token}

	_, err := client.FindUsers(req)
	if err == nil {
		t.Errorf("Error was expected; none received")
	}

	result := fmt.Sprintf("OrderField %s invalid", req.OrderField)
	if errString := err.Error(); errString != result {
		t.Errorf("Expected: \"%s\"; received: \"%s\"", result, errString)
	}
}

func TestSearchClientFindUsersZeroOffset(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(SearchServer))
	defer server.Close()

	req := SearchRequest{Offset: 0, Limit: 2, Query: "", OrderBy: 0, OrderField: ""}
	client := &SearchClient{URL: server.URL, AccessToken: token}

	resp, err := client.FindUsers(req)
	if err != nil {
		t.Errorf("None was expected; err received: %s", err.Error())
	}

	assert.Equal(t, resp.NextPage, true)
	assert.Equal(t, len(resp.Users), req.Limit)
}

func TestSearchClientFindUsersBigOffset(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(SearchServer))
	defer server.Close()

	req := SearchRequest{Offset: 10000000, Limit: 25, Query: "", OrderBy: 0, OrderField: ""}
	client := &SearchClient{URL: server.URL, AccessToken: token}

	resp, err := client.FindUsers(req)
	if err != nil {
		t.Errorf("None was expected; err received: %s", err.Error())
	}

	assert.Equal(t, resp.NextPage, false)
}

func TestSearchClientFindUsersCantUnpackResult(t *testing.T) {
	server := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("{"))
			},
		),
	)
	defer server.Close()

	req := SearchRequest{Offset: 10000000, Limit: 25, Query: "", OrderBy: 0, OrderField: ""}
	client := &SearchClient{URL: server.URL, AccessToken: token}

	_, err := client.FindUsers(req)
	if err == nil {
		t.Errorf("Error was expected; none received")
	}

	result := "cant unpack result json: unexpected end of JSON input"
	if errString := err.Error(); errString != result {
		t.Errorf("Expected: \"%s\"; received: \"%s\"", result, errString)
	}
}
