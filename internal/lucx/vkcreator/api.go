// Adapted from https://github.com/ildarmaga/wdtt panel/vk_call_api.go (GPL-3.0).
package vkcreator

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	vkCallAppID      = "6287487"
	vkCallAPIVersion = "5.280"
	vkCallWebHost    = "vk.ru"
	vkCallUserAgent  = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/148.0.0.0 Safari/537.36"
)

var vkCallHTTPClient = &http.Client{Timeout: 60 * time.Second}

type vkAPIError struct {
	Code    int    `json:"error_code"`
	Message string `json:"error_msg"`
}

var vkHTTPPostDo = func(endpoint string, form url.Values, headers map[string]string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", vkCallUserAgent)
	req.Header.Set("Origin", "https://"+vkCallWebHost)
	req.Header.Set("Referer", "https://"+vkCallWebHost+"/")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := vkCallHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func vkParseAPIError(body []byte) error {
	var wrap struct {
		Error vkAPIError `json:"error"`
	}
	if json.Unmarshal(body, &wrap) != nil || wrap.Error.Code == 0 {
		return nil
	}
	return fmt.Errorf("VK API %d: %s", wrap.Error.Code, wrap.Error.Message)
}

func truncateBody(b []byte, max int) string {
	s := strings.TrimSpace(string(b))
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

func WebToken(cookieHeader string) (string, error) {
	body, err := vkHTTPPostDo("https://login."+vkCallWebHost+"/?act=web_token",
		url.Values{"version": {"1"}, "app_id": {vkCallAppID}},
		map[string]string{"Cookie": cookieHeader})
	if err != nil {
		return "", fmt.Errorf("web_token: %w", err)
	}
	var tok struct {
		Data struct {
			AccessToken string `json:"access_token"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &tok); err != nil {
		return "", fmt.Errorf("web_token parse: %w", err)
	}
	if tok.Data.AccessToken == "" {
		return "", fmt.Errorf("empty VK token, response: %s", truncateBody(body, 300))
	}
	return tok.Data.AccessToken, nil
}

func currentUserID(token string) (string, error) {
	body, err := vkHTTPPostDo("https://api."+vkCallWebHost+"/method/users.get",
		url.Values{"v": {vkCallAPIVersion}},
		map[string]string{"Authorization": "Bearer " + token})
	if err != nil {
		return "", fmt.Errorf("users.get: %w", err)
	}
	if err := vkParseAPIError(body); err != nil {
		return "", err
	}
	var resp struct {
		Response []struct {
			ID int64 `json:"id"`
		} `json:"response"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("users.get parse: %w", err)
	}
	if len(resp.Response) == 0 || resp.Response[0].ID == 0 {
		return "", fmt.Errorf("users.get: empty id, response: %s", truncateBody(body, 300))
	}
	return fmt.Sprint(resp.Response[0].ID), nil
}

// CallCreateResult is the outcome of calls.start.
type CallCreateResult struct {
	CallID   string
	JoinLink string
}

// CreateCallLink creates a VK group call and returns join_link + call_id.
func CreateCallLink(cookieHeader string) (CallCreateResult, error) {
	var out CallCreateResult
	token, err := WebToken(cookieHeader)
	if err != nil {
		return out, err
	}
	peerID, err := currentUserID(token)
	if err != nil {
		return out, err
	}
	body, err := vkHTTPPostDo("https://api."+vkCallWebHost+"/method/calls.start",
		url.Values{"v": {vkCallAPIVersion}, "peer_id": {peerID}},
		map[string]string{"Authorization": "Bearer " + token})
	if err != nil {
		return out, fmt.Errorf("calls.start: %w", err)
	}
	if err := vkParseAPIError(body); err != nil {
		return out, err
	}
	var call struct {
		Response struct {
			CallID   string `json:"call_id"`
			JoinLink string `json:"join_link"`
		} `json:"response"`
	}
	if err := json.Unmarshal(body, &call); err != nil {
		return out, fmt.Errorf("calls.start parse: %w", err)
	}
	if call.Response.CallID == "" {
		return out, fmt.Errorf("calls.start: empty call_id, response: %s", truncateBody(body, 300))
	}
	if call.Response.JoinLink == "" {
		return out, fmt.Errorf("calls.start: empty join_link, response: %s", truncateBody(body, 300))
	}
	out.CallID = call.Response.CallID
	out.JoinLink = call.Response.JoinLink
	return out, nil
}

// ForceFinishCall ends a VK call by call_id.
func ForceFinishCall(cookieHeader, callID string) error {
	callID = strings.TrimSpace(callID)
	if callID == "" {
		return fmt.Errorf("call_id empty")
	}
	token, err := WebToken(cookieHeader)
	if err != nil {
		return err
	}
	body, err := vkHTTPPostDo("https://api."+vkCallWebHost+"/method/calls.forceFinish",
		url.Values{"v": {vkCallAPIVersion}, "call_id": {callID}},
		map[string]string{"Authorization": "Bearer " + token})
	if err != nil {
		return fmt.Errorf("calls.forceFinish: %w", err)
	}
	if err := vkParseAPIError(body); err != nil {
		return err
	}
	var resp struct {
		Response int `json:"response"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return fmt.Errorf("calls.forceFinish parse: %w", err)
	}
	if resp.Response != 1 {
		return fmt.Errorf("calls.forceFinish: unexpected response %s", truncateBody(body, 300))
	}
	return nil
}

func anonToken() (string, error) {
	body, err := vkHTTPPostDo("https://login."+vkCallWebHost+"/?act=get_anonym_token",
		url.Values{"client_id": {vkCallAppID}}, nil)
	if err != nil {
		return "", err
	}
	var tok struct {
		Data struct {
			AccessToken string `json:"access_token"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &tok); err != nil {
		return "", err
	}
	if tok.Data.AccessToken == "" {
		return "", fmt.Errorf("empty anonym token")
	}
	return tok.Data.AccessToken, nil
}

// CallAlive reports whether the join link still has an active call preview.
func CallAlive(joinLink string) bool {
	joinLink = strings.TrimSpace(joinLink)
	if joinLink == "" {
		return false
	}
	token, err := anonToken()
	if err != nil {
		return false
	}
	body, err := vkHTTPPostDo("https://api."+vkCallWebHost+"/method/calls.getCallPreview",
		url.Values{"v": {vkCallAPIVersion}, "vk_join_link": {joinLink}},
		map[string]string{"Authorization": "Bearer " + token})
	if err != nil {
		return false
	}
	if vkParseAPIError(body) != nil {
		return false
	}
	var resp struct {
		Response struct {
			OKJoinLink string `json:"ok_join_link"`
		} `json:"response"`
	}
	if json.Unmarshal(body, &resp) != nil {
		return false
	}
	return resp.Response.OKJoinLink != ""
}
