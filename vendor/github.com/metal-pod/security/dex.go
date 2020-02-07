package security

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/lestrrat-go/jwx/jwk"
)

const (
	refetchInterval = 10 * time.Minute
)

type updater struct {
	updated chan *jwk.Set
}

// A Dex ...
type Dex struct {
	baseURL         string
	keys            chan<- keyRQ
	update          chan updater
	refreshInterval time.Duration

	userExtractor UserExtractorFn
}

type keyRsp struct {
	keys *jwk.Set
	err  error
}
type keyRQ struct {
	rsp chan<- keyRsp
}

// NewDex returns a new Dex.
func NewDex(baseurl string) (*Dex, error) {
	dx := &Dex{
		baseURL:         baseurl,
		refreshInterval: refetchInterval,
		userExtractor:   defaultUserExtractor,
	}
	if err := dx.keyfetcher(); err != nil {
		return nil, err
	}
	return dx, nil
}

// Option configures Dex
type Option func(dex *Dex) *Dex

// With sets available Options
func (dx *Dex) With(opts ...Option) *Dex {
	for _, opt := range opts {
		opt(dx)
	}
	return dx
}

// UserExtractorFn extracts the User and Claims
type UserExtractorFn func(claims *Claims) (*User, error)

func UserExtractor(fn UserExtractorFn) Option {
	return func(dex *Dex) *Dex {
		dex.userExtractor = fn
		return dex
	}
}

// the keyfetcher fetches the keys from the remote dex at a regular interval.
// if the client needs the keys it returns the cached keys.
func (dx *Dex) keyfetcher() error {
	c := make(chan keyRQ)
	dx.keys = c
	dx.update = make(chan updater)
	t := time.Tick(dx.refreshInterval)
	keys, err := jwk.Fetch(dx.baseURL + "/keys")
	if err != nil {
		return fmt.Errorf("cannot fetch dex keys from %s/keys: %v", dx.baseURL, err)
	}
	go func() {
		defer close(c)
		for {
			select {
			case keyRQ := <-c:
				keyRQ.rsp <- keyRsp{keys, err}
			case <-t:
				keys, err = dx.updateKeys(keys, fmt.Sprintf("Timer: %s", dx.refreshInterval))
			case u := <-dx.update:
				keys, err = dx.updateKeys(keys, "forced update")
				u.updated <- keys
			}
		}
	}()
	return nil
}

// fetchKeys asks the current keyfetcher to give the current keyset
func (dx *Dex) fetchKeys() (*jwk.Set, error) {
	outchan := make(chan keyRsp)
	krq := keyRQ{rsp: outchan}
	defer close(krq.rsp)
	dx.keys <- krq
	rsp := <-outchan
	return rsp.keys, rsp.err
}

func (dx *Dex) forceUpdate() {
	u := updater{
		updated: make(chan *jwk.Set),
	}
	defer close(u.updated)
	dx.update <- u
	<-u.updated
}

func (dx *Dex) updateKeys(old *jwk.Set, reason string) (*jwk.Set, error) {
	k, e := jwk.Fetch(dx.baseURL + "/keys")
	if e != nil {
		return old, fmt.Errorf("cannot fetch dex keys from %s/keys: %v", dx.baseURL, e)
	}
	return k, e
}

// searchKey searches the given key in the set loaded from dex. If
// there is a key it will be returned otherwise an error is returned
func (dx *Dex) searchKey(kid string) (interface{}, error) {
	for i := 0; i < 2; i++ {
		keys, err := dx.fetchKeys()
		if err != nil {
			return nil, err
		}
		jwtkeys := keys.LookupKeyID(kid)
		if len(jwtkeys) == 0 {
			dx.forceUpdate()
			continue
		}
		return jwtkeys[0].Materialize()
	}
	return nil, fmt.Errorf("key %q not found", kid)
}

// User implements the UserGetter to get a user from the request.
func (dx *Dex) User(rq *http.Request) (*User, error) {
	auth := rq.Header.Get("Authorization")
	if auth == "" {
		return nil, errNoAuthFound
	}
	splitToken := strings.Split(auth, "Bearer")
	if len(splitToken) < 2 {
		// no Bearer token
		return nil, errNoAuthFound
	}
	bearerToken := strings.TrimSpace(splitToken[1])

	token, err := jwt.ParseWithClaims(bearerToken, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		kid := token.Header["kid"].(string)
		return dx.searchKey(kid)
	})
	if err != nil {
		return nil, err
	}
	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return dx.userExtractor(claims)
	}
	return nil, fmt.Errorf("invalid claims")
}

func defaultUserExtractor(claims *Claims) (*User, error) {
	var grps []RessourceAccess
	for _, g := range claims.Groups {
		grps = append(grps, RessourceAccess(g))
	}
	tenant := ""
	if claims.FederatedClaims != nil {
		cid := claims.FederatedClaims["connector_id"]
		if cid != "" {
			tenant = strings.Split(cid, "_")[0]
		}
	}
	usr := User{
		Name:   claims.Name,
		EMail:  claims.EMail,
		Groups: grps,
		Tenant: tenant,
	}
	return &usr, nil
}
