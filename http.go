package niu

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"
)

const (
	HeaderAuthorization = "Authorization"
	HeaderContentType   = "Content-Type"

	ContentTypeJson      = "application/json;charset=utf-8"
	ContentTypeText      = "text/plain;charset=utf-8"
	ContentTypeEncrypted = "application/x-encrypted;charset=utf-8"
)

// 发送 Http Post 请求
func HttpPost(url string, data []byte, options *HttpOptions) ([]byte, error) {
	// 创建请求
	req, err := http.NewRequest("POST", url, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	// 设置请求头
	for k, v := range options.Headers {
		req.Header.Add(k, v)
	}
	client := &http.Client{}         // 创建客户端
	client.Timeout = options.Timeout // 设置超时时间
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() // 这步是必要的，防止以后的内存泄漏，切记
	if resp.StatusCode != 200 {
		if resp.ContentLength > 0 {
			data, _ := io.ReadAll(resp.Body)
			return data, errors.New("status code:" + strconv.Itoa(resp.StatusCode))
		}
		return nil, errors.New("status code:" + strconv.Itoa(resp.StatusCode))
	}
	return io.ReadAll(resp.Body)
}

type HttpOptions struct {
	Headers map[string]string
	Timeout time.Duration
}

func NewHttpOptions(headers map[string]string) *HttpOptions {
	return &HttpOptions{
		Headers: headers,
		Timeout: 30 * time.Second,
	}
}

func NewEmptyHttpOptions() *HttpOptions {
	return &HttpOptions{}
}

func NewHttpOptionsJson(authorization string) *HttpOptions {
	headers := make(map[string]string)
	headers[HeaderAuthorization] = authorization
	headers[HeaderContentType] = ContentTypeJson

	return &HttpOptions{
		Headers: headers,
		Timeout: 30 * time.Second,
	}
}

// resp 必须是一个指针
func HttpPostJson(url string, data any, options *HttpOptions, resp any) error {
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return err
	}

	respBytes, err := HttpPost(url, jsonBytes, options)
	if err != nil {
		return err
	}
	err = json.Unmarshal(respBytes, resp)
	if err != nil {
		return err
	}
	return nil
}

// 发送 Http Get 请求
func HttpGet(url string, options *HttpOptions) ([]byte, error) {
	// 创建请求
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	// 设置请求头
	for k, v := range options.Headers {
		req.Header.Add(k, v)
	}

	client := &http.Client{}         // 创建客户端
	client.Timeout = options.Timeout // 设置超时时间
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() // 这步是必要的，防止以后的内存泄漏，切记
	if resp.StatusCode != 200 {
		return nil, errors.New("status code:" + strconv.Itoa(resp.StatusCode))
	}
	return io.ReadAll(resp.Body)
}

// resp 必须是一个指针
func HttpGetJson(url string, options *HttpOptions, resp any) error {
	respBytes, err := HttpGet(url, options)
	if err != nil {
		return err
	}
	err = json.Unmarshal(respBytes, resp)
	if err != nil {
		return err
	}
	return nil
}
