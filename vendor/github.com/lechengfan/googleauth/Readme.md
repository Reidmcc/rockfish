##### Google OAuth HTTP handler

Inspired by <https://github.com/kr/githubauth>. This package provides a Handler to authenticate through Google OAuth.

###### Example

```go
h := &githubauth.Handler{
	PermittedEmails: map[string]bool{"tommy@interstellar.com":true},
	Keys:         keys(),
	ClientID:     os.Getenv("OAUTH_CLIENT_ID"),
	ClientSecret: os.Getenv("OAUTH_CLIENT_SECRET"),
}
http.ListenAndServe(":8080", h)
```
