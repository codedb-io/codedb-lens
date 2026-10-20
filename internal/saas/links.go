// Package saas is the single source of every link to the codeDB SaaS.
// UTM parameters carry no personal data; they only tell which surface and rule led to the visit.
package saas

import "net/url"

const (
	Landing  = "https://codedb.com.br/"
	Register = "https://app.codedb.com.br/auth/register"
)

// Link adds utm_source=lens, the surface (report, cli, gui, docs) and the campaign (usually a rule ID).
func Link(base, medium, campaign string) string {
	u, err := url.Parse(base)
	if err != nil {
		return base
	}
	q := u.Query()
	q.Set("utm_source", "lens")
	q.Set("utm_medium", medium)
	q.Set("utm_campaign", campaign)
	u.RawQuery = q.Encode()
	return u.String()
}
