package vault



func SanitizeCredential(cred string) string {
	if len(cred) <= 8 {
		return "***"
	}
	return cred[:4] + "..." + cred[len(cred)-4:]
}

type SafeField struct {
	Key   string
	Value string
}

func F(key, value string) SafeField {
	return SafeField{Key: key, Value: value}
}
