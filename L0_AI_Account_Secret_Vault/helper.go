package vault

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
)

func generateAccountID() string {
	b := make([]byte, 6)
	rand.Read(b)
	return "acc_" + hex.EncodeToString(b)
}

func generateMaskedDisplay(platform string, credentials string) string {
	switch platform {
	case "fanqie":
		return extractFanqieDisplay(credentials)
	case "xhs":
		return extractXhsDisplay(credentials)
	case "wechat":
		return extractWechatDisplay(credentials)
	case "douyin":
		return extractDouyinDisplay(credentials)
	case "bilibili":
		return extractBilibiliDisplay(credentials)
	case "zhulang":
		return extractCookieBasedDisplay(credentials, "zhulang")
	default:
		return platform + "_bound"
	}
}

func extractFanqieDisplay(credentials string) string {
	if idx := findCookieField(credentials, "sessionid"); idx >= 0 {
		val := extractCookieValue(credentials, idx)
		if len(val) >= 6 {
			return "fanqie_" + val[:4] + val[len(val)-2:]
		}
	}
	return "fanqie_bound"
}

func extractXhsDisplay(credentials string) string {
	if idx := findCookieField(credentials, "a1"); idx >= 0 {
		val := extractCookieValue(credentials, idx)
		if len(val) >= 6 {
			return "xhs_" + val[:4] + val[len(val)-2:]
		}
	}
	return "xhs_bound"
}

func extractWechatDisplay(credentials string) string {
	appID := extractJSONField(credentials, "app_id")
	if appID != "" && len(appID) >= 6 {
		return "wechat_" + appID[len(appID)-6:]
	}
	if appID != "" {
		return "wechat_" + appID
	}
	return "wechat_bound"
}

func extractDouyinDisplay(credentials string) string {
	if idx := findCookieField(credentials, "sessionid"); idx >= 0 {
		val := extractCookieValue(credentials, idx)
		if len(val) >= 6 {
			return "douyin_" + val[:4] + val[len(val)-2:]
		}
	}
	return "douyin_bound"
}

func extractBilibiliDisplay(credentials string) string {
	if idx := findCookieField(credentials, "DedeUserID"); idx >= 0 {
		val := extractCookieValue(credentials, idx)
		if val != "" {
			return "bilibili_" + val
		}
	}
	return "bilibili_bound"
}

func extractCookieBasedDisplay(credentials string, platform string) string {
	if idx := findCookieField(credentials, "sessionid"); idx >= 0 {
		val := extractCookieValue(credentials, idx)
		if len(val) >= 6 {
			return platform + "_" + val[:4] + val[len(val)-2:]
		}
	}
	return platform + "_bound"
}

func findCookieField(cookieStr string, fieldName string) int {
	return strings.Index(cookieStr, fieldName+"=")
}

func extractCookieValue(cookieStr string, startIdx int) string {
	startIdx = strings.Index(cookieStr[startIdx:], "=") + 1 + startIdx
	if startIdx <= 0 {
		return ""
	}
	endIdx := strings.IndexAny(cookieStr[startIdx:], "; \t\n\r")
	if endIdx < 0 {
		return cookieStr[startIdx:]
	}
	return cookieStr[startIdx : startIdx+endIdx]
}

func extractJSONField(jsonStr string, fieldName string) string {
	searchKey := fmt.Sprintf(`"%s"`, fieldName)
	idx := strings.Index(jsonStr, searchKey)
	if idx < 0 {
		return ""
	}
	startIdx := strings.Index(jsonStr[idx+len(searchKey):], `"`) + idx + len(searchKey) + 1
	if startIdx <= 0 {
		return ""
	}
	endIdx := strings.Index(jsonStr[startIdx:], `"`)
	if endIdx < 0 {
		return ""
	}
	return jsonStr[startIdx : startIdx+endIdx]
}
