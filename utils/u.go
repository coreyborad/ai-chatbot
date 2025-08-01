package utils

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"

	"github.com/oschwald/geoip2-golang"
)

// GeolocationInfo 結構體，用於模擬地理定位服務的回應
type GeolocationInfo struct {
	IP        string  `json:"ip"`
	Country   string  `json:"country"`
	City      string  `json:"city"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Provider  string  `json:"provider"`
}

var db *geoip2.Reader

// getClientIP 從 HTTP 請求中獲取客戶端真實 IP 地址
func GetClientIP(r *http.Request) string {
	// 優先檢查 X-Forwarded-For 頭，這通常用於代理或負載均衡器
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// X-Forwarded-For 可能包含多個 IP (client, proxy1, proxy2)
		// 我們通常取第一個 (最左邊的) 作為真實客戶端 IP
		ips := strings.Split(xff, ", ")
		return ips[0]
	}
	// 其次檢查 X-Real-IP 頭
	if xRealIP := r.Header.Get("X-Real-IP"); xRealIP != "" {
		return xRealIP
	}
	// 最後使用 r.RemoteAddr (形式為 IP:Port)
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr // fallback if SplitHostPort fails
	}
	return ip
}

// isPrivateIP checks if a given net.IP address belongs to a private network range (RFC 1918 for IPv4, ULA for IPv6).
func isPrivateIP(ipStr string) bool {
	ip := net.ParseIP(ipStr)

	// If ip is nil, it means the string parsing failed earlier, so it's not a valid IP.
	if ip == nil {
		return false // Or handle this as an error, depending on your needs.
	}

	// Handle IPv4 private ranges
	var privateIPv4Ranges = []net.IPNet{
		{IP: net.ParseIP("10.0.0.0").To4(), Mask: net.CIDRMask(8, 32)},     // 10.0.0.0/8
		{IP: net.ParseIP("172.16.0.0").To4(), Mask: net.CIDRMask(12, 32)},  // 172.16.0.0/12
		{IP: net.ParseIP("192.168.0.0").To4(), Mask: net.CIDRMask(16, 32)}, // 192.168.0.0/16
	}

	// Handle IPv6 Unique Local Address (ULA) range
	_, privateIPv6ULANet, err := net.ParseCIDR("fc00::/7")
	if err != nil {
		fmt.Printf("Error parsing IPv6 ULA CIDR: %v\n", err)
		return false // This should ideally not happen for a hardcoded valid CIDR
	}

	if ip.To4() != nil { // Check if it's an IPv4 address
		for _, r := range privateIPv4Ranges {
			if r.Contains(ip) {
				return true
			}
		}
		// Special case for IPv4 loopback (127.0.0.1/8)
		if ip.IsLoopback() {
			return true
		}
	} else if ip.To16() != nil { // Check if it's an IPv6 address
		// Check for IPv6 ULA
		if privateIPv6ULANet.Contains(ip) {
			return true
		}
		// Special cases for IPv6 loopback (::1/128) and link-local (fe80::/10)
		if ip.IsLoopback() || ip.IsLinkLocalUnicast() {
			return true
		}
	}
	return false
}

func GetLocationByIP(ip string) string {
	isPrivateIP := isPrivateIP(ip)
	if isPrivateIP {
		log.Println("Private IP detected:", ip)
		return "Taiwan Taipei"
	}

	if db == nil {
		var err error
		db, err = geoip2.Open("./GeoLite2-City.mmdb")
		if err != nil {
			log.Fatal(err)
		}
	}
	record, err := db.City(net.ParseIP(ip))
	if err != nil {
		log.Println("Error getting location for IP:", ip, "-", err)
		return "Unknown"
	}
	return record.Country.Names["en"] + " " + record.City.Names["en"]
}
