package adminconfig

import (
	"os"
	"strings"
)

// loadGoogle reads the Google sign-in settings. The client ID is public (it
// is sent to the browser), so there is no secret to configure.
func (c *Config) loadGoogle() {
	c.GoogleClientID = strings.TrimSpace(os.Getenv("ADMIN_GOOGLE_CLIENT_ID"))
	c.GoogleAllowedDomains = nil
	for _, d := range strings.Split(os.Getenv("ADMIN_GOOGLE_ALLOWED_DOMAINS"), ",") {
		if d = strings.ToLower(strings.TrimSpace(d)); d != "" {
			c.GoogleAllowedDomains = append(c.GoogleAllowedDomains, d)
		}
	}
}

func (c Config) googleProblems() []string {
	if c.GoogleClientID == "" && len(c.GoogleAllowedDomains) > 0 {
		return []string{"ADMIN_GOOGLE_ALLOWED_DOMAINS 需要同时设置 ADMIN_GOOGLE_CLIENT_ID"}
	}
	return nil
}
