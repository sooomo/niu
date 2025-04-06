package niu

import (
	"net"
	"strings"

	"github.com/gin-gonic/gin"
)

// 获取客户端的IP地址
func GetClientIp(ctx *gin.Context) string {
	ip := ctx.Request.Header.Get("X-Real-IP")
	netIp := net.ParseIP(ip)
	if netIp != nil {
		return ip
	}

	ips := ctx.Request.Header.Get("X-Forwarded-For")
	splitIps := strings.SplitSeq(ips, ",")
	for ip := range splitIps {
		netIp = net.ParseIP(ip)
		if netIp != nil {
			return ip
		}
	}

	ip, _, err := net.SplitHostPort(ctx.Request.RemoteAddr)
	if err != nil {
		return ""
	}

	netIp = net.ParseIP(ip)
	if netIp != nil {
		return ip
	}

	return ""
}
