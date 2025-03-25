package niu

import (
	"fmt"
)

type wxAuthResp struct {
	AccessToken    string `json:"access_token"`
	ExpiresIn      int    `json:"expires_in"`
	RefreshToken   string `json:"refresh_token"`
	OpenId         string `json:"openid"`
	Scope          string `json:"scope"`
	IsSnapshotUser int    `json:"is_snapshotuser"`
	Unionid        string `json:"unionid"`
}

// 用于获取微信的OpenId
func GetWxOpenId(code, appId, appSecret string) (string, error) {
	var resp wxAuthResp
	link := fmt.Sprintf("https://api.weixin.qq.com/sns/oauth2/access_token?appid=%v&secret=%v&code=%v&grant_type=authorization_code", appId, appSecret, code)

	headers := make(map[string]string)
	err := HttpGetJson(link, NewHttpOptions(headers), &resp)
	if err != nil {
		return "", err
	}

	return resp.OpenId, nil
}
