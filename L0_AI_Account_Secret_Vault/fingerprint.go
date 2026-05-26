package vault

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
)

// computeCredentialFingerprint 对凭证明文做规范化后计算 SHA256，用于全局去重。
// 同平台下相同 Cookie/凭证内容会得到相同指纹。
func computeCredentialFingerprint(platform, credentialsPlaintext string) string {
	normalized := normalizeCredentialPlaintext(platform, credentialsPlaintext)
	sum := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(sum[:])
}

func normalizeCredentialPlaintext(platform, credentials string) string {
	credentials = strings.TrimSpace(credentials)
	if credentials == "" {
		return ""
	}

	// JSON 凭证（如微信）按原文 trim 后哈希，避免引入 JSON 解析差异。
	if strings.HasPrefix(credentials, "{") {
		return credentials
	}

	type cookiePair struct {
		key string
		val string
	}

	parts := strings.Split(credentials, ";")
	pairs := make([]cookiePair, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		eq := strings.Index(part, "=")
		if eq < 0 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(part[:eq]))
		val := strings.TrimSpace(part[eq+1:])
		if key == "" {
			continue
		}
		pairs = append(pairs, cookiePair{key: key, val: val})
	}

	if len(pairs) == 0 {
		return credentials
	}

	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].key == pairs[j].key {
			return pairs[i].val < pairs[j].val
		}
		return pairs[i].key < pairs[j].key
	})

	var b strings.Builder
	for i, p := range pairs {
		if i > 0 {
			b.WriteByte(';')
		}
		b.WriteString(p.key)
		b.WriteByte('=')
		b.WriteString(p.val)
	}
	return b.String()
}
