package googleauth

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/kr/session"
	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const callbackPath = "/_googleauth"

type Session struct {
	// Client is an HTTP client obtained from oauth2.Config.Client.
	// It adds necessary OAuth2 credentials to outgoing requests to
	// perform Google API calls.
	*http.Client
}

type GoogleUser struct {
	Sub string `json:"sub"`
	Name string `json:"name"`
	GivenName string `json:"given_name"`
	FamilyName string `json:"family_name"`
	Profile string `json:"profile"`
	Picture string `json:"picture"`
	Email string `json:"email"`
	EmailVerified bool `json:"email_verified"`
	Gender string `json:"gender"`
}

type contextKey int

const sessionKey contextKey = 0

// GetSession returns data about the logged-in user
// given the Context provided to a ContextHandler.
func GetSession(ctx context.Context) (*Session, bool) {
	s, ok := ctx.Value(sessionKey).(*Session)
	return s, ok
}

// A ContextHandler can be used as the HTTP handler
// in a Handler value in order to obtain information
// about the logged-in Google user through the provided
// Context. See GetSession.
type ContextHandler interface {
	ServeHTTPContext(context.Context, http.ResponseWriter, *http.Request)
}

// Handler is an HTTP handler that requires
// users to log in with Google OAuth and requires
// their emails to be in the list of permitted emails.
type Handler struct {
	// PermittedEmails is a map of all Google Account
	// emails that are allowed access.
	// If unset, any user will be permitted.
	PermittedEmails map[string]bool

	// Used to initialize corresponding fields of a session Config.
	// See github.com/kr/session.
	// If Name is empty, "googleauth" is used.
	Name   string
	Path   string
	Domain string
	MaxAge time.Duration
	Keys   []*[32]byte

	// Used to initialize corresponding fields of oauth2.Config.
	// Scopes can be nil, in which case "https://www.googleapis.com/auth/userinfo.email"
	// will be requested.
	ClientID     string
	ClientSecret string
	Scopes       []string

	// Handler is the HTTP handler called
	// once authentication is complete.
	// If nil, http.DefaultServeMux is used.
	// If the value implements ContextHandler,
	// its ServeHTTPContext method will be called
	// instead of ServeHTTP, and a *Session value
	// can be obtained from GetSession.
	Handler http.Handler
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.ServeHTTPContext(context.Background(), w, r)
}

func (h *Handler) ServeHTTPContext(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	handler := h.Handler
	if handler == nil {
		handler = http.DefaultServeMux
	}
	if ctx, ok := h.loginOk(ctx, w, r); ok {
		if h2, ok := handler.(ContextHandler); ok {
			h2.ServeHTTPContext(ctx, w, r)
		} else {
			handler.ServeHTTP(w, r)
		}
	}
}

// loginOk checks that the user is logged in and authorized.
// If not, it performs one step of the oauth process.
func (h *Handler) loginOk(ctx context.Context, w http.ResponseWriter, r *http.Request) (context.Context, bool) {
	var user sess
	err := session.Get(r, &user, h.sessionConfig())
	if err != nil && err != http.ErrNoCookie {
		h.deleteCookie(w)
		http.Error(w, "internal error", 500)
		return ctx, false
	}

	redirectURL := "https://" + r.Host + callbackPath
	conf := &oauth2.Config{
		ClientID:     h.ClientID,
		ClientSecret: h.ClientSecret,
		RedirectURL:  redirectURL,
		Scopes:       h.Scopes,
		Endpoint:     google.Endpoint,
	}
	if conf.Scopes == nil {
		conf.Scopes = []string{"https://www.googleapis.com/auth/userinfo.email"}
	}
	if user.OAuthToken != nil {
		session.Set(w, user, h.sessionConfig()) // refresh the cookie
		ctx = context.WithValue(ctx, sessionKey, &Session{
			Client: conf.Client(ctx, user.OAuthToken),
		})
		return ctx, true
	}
	if r.URL.Path == callbackPath {
		if r.FormValue("state") != user.State {
			h.deleteCookie(w)
			http.Error(w, "access forbidden", 401)
			return ctx, false
		}
		tok, err := conf.Exchange(ctx, r.FormValue("code"))
		if err != nil {
			h.deleteCookie(w)
			http.Error(w, "access forbidden", 401)
			return ctx, false
		}
		client := conf.Client(ctx, tok)
		if len(h.PermittedEmails) > 0 {
			resp, err := client.Get("https://www.googleapis.com/oauth2/v3/userinfo")
			if err != nil || resp.StatusCode != 200 {
				h.deleteCookie(w)
				http.Error(w, "access forbidden", 401)
				return ctx, false
			}
			var v GoogleUser
			if err := json.NewDecoder(resp.Body).Decode(&v); err != nil {
				log.Println("Decode:", err)
				h.deleteCookie(w)
				http.Error(w, "Internal Server Error - Invalid JSON from Google", 500)
				return ctx, false
			}
			if !v.EmailVerified || !h.PermittedEmails[v.Email] {
				log.Println("User is not allowed access:", v.Email)
				h.deleteCookie(w)
				http.Error(w, "Access Forbidden - Your account is not allowed access.", 401)
				return ctx, false
			}
		}

		session.Set(w, sess{OAuthToken: tok}, h.sessionConfig())
		http.Redirect(w, r, user.NextURL, http.StatusTemporaryRedirect)
		return ctx, false
	}

	u := *r.URL
	u.Scheme = "https"
	u.Host = r.Host
	state := newState()
	session.Set(w, sess{NextURL: u.String(), State: state}, h.sessionConfig())
	http.Redirect(w, r, conf.AuthCodeURL(state), http.StatusTemporaryRedirect)
	return ctx, false
}

func (h *Handler) sessionConfig() *session.Config {
	c := &session.Config{
		Name:   h.Name,
		Path:   h.Path,
		Domain: h.Domain,
		MaxAge: h.MaxAge,
		Keys:   h.Keys,
	}
	if c.Name == "" {
		c.Name = "googleauth"
	}
	return c
}

func (h *Handler) deleteCookie(w http.ResponseWriter) error {
	conf := h.sessionConfig()
	conf.MaxAge = -1 * time.Second
	return session.Set(w, sess{}, conf)
}

type sess struct {
	OAuthToken *oauth2.Token `json:",omitempty"`
	NextURL    string        `json:",omitempty"`
	State      string        `json:",omitempty"`
}

func newState() string {
	b := make([]byte, 10)
	rand.Read(b)
	return hex.EncodeToString(b)
}
