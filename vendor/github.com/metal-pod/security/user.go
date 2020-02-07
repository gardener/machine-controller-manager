package security

import (
	"fmt"
	"net/http"

	"github.com/dgrijalva/jwt-go"
	"github.com/go-openapi/runtime"
	"golang.org/x/net/context"
)

// A User is the current user who is executing a rest function.
type User struct {
	EMail  string
	Name   string
	Groups []RessourceAccess
	Tenant string
}

var (
	guest = User{
		EMail:  "anonymous@fi-ts.io",
		Name:   "anonymous",
		Groups: []RessourceAccess{},
	}
	errNoAuthFound = fmt.Errorf("no auth found")
)

// RessourceAccess is the type for our groups
type RessourceAccess string

type accessGroup []RessourceAccess
type ressourceSet map[RessourceAccess]bool

func (ra accessGroup) asSet() ressourceSet {
	groupset := make(ressourceSet)
	for _, g := range ra {
		groupset[g] = true
	}
	return groupset
}

// We overwrite the Audience because in the current version of the jwt library this
// is not an array.
type Claims struct {
	jwt.StandardClaims
	Audience        interface{}       `json:"aud,omitempty"`
	Groups          []string          `json:"groups"`
	EMail           string            `json:"email"`
	Name            string            `json:"name"`
	FederatedClaims map[string]string `json:"federated_claims"`
}

// HasGroup returns true if the user has at least one of the given groups.
func (u *User) HasGroup(grps ...RessourceAccess) bool {
	acc := accessGroup(u.Groups).asSet()
	for _, grp := range grps {
		if ok := acc[grp]; ok {
			return true
		}
	}
	return false
}

// A UserGetter returns the authenticated user from the request.
type UserGetter interface {
	User(rq *http.Request) (*User, error)
}

// UserCreds stores different methods for user extraction from a request.
type UserCreds struct {
	dex       *Dex
	macauther []HMACAuth
}

// CredsOpt is a option setter for UserCreds
type CredsOpt func(*UserCreds)

// NewCreds returns a credention checker which tries to pull out the current user
// of a request. You can set many different HMAC auth'ers but only one for bearer tokens.
func NewCreds(opts ...CredsOpt) *UserCreds {
	res := &UserCreds{}

	for _, o := range opts {
		o(res)
	}

	return res
}

// WithDex sets the dex auther.
func WithDex(d *Dex) CredsOpt {
	return func(uc *UserCreds) {
		uc.dex = d
	}
}

// WithHMAC appends the given HMACAuth to the list of allowed authers.
func WithHMAC(hma HMACAuth) CredsOpt {
	return func(uc *UserCreds) {
		uc.macauther = append(uc.macauther, hma)
	}
}

// User pulls out a user from the request. It uses all authers
// which where specified when creating this usercred. the first
// auther which returns a user wins.
// if no auther returns a user, a guest with no rights will be returned.
func (uc *UserCreds) User(rq *http.Request) (*User, error) {
	authers := make([]UserGetter, 0, len(uc.macauther)+1)
	for i := range uc.macauther {
		authers = append(authers, &uc.macauther[i])
	}
	if uc.dex != nil {
		authers = append(authers, uc.dex)
	}
	for _, auth := range authers {
		u, err := auth.User(rq)
		if err == nil {
			return u, nil
		}
		if err != errNoAuthFound && err != errIllegalAuthFound && err != errUnknownAuthFound {
			return nil, err
		}
	}
	return &guest, nil
}

// AddUserToken adds the given token as a bearer token to the request.
func AddUserToken(rq *http.Request, token string) {
	rq.Header.Add("Authorization", "Bearer "+token)
}

// AddUserTokenToClientRequest to support openapi
func AddUserTokenToClientRequest(rq runtime.ClientRequest, token string) {
	rq.SetHeaderParam("Authorization", "Bearer "+token)
}

// use a private type and value for the key inside the context
type key int

var (
	userkey = key(0)
)

// GetUserFromContext returns the current user from the context. If no user is set
// it returns a guest with no rights.
func GetUserFromContext(ctx context.Context) *User {
	u, ok := ctx.Value(userkey).(*User)
	if ok {
		return u
	}
	return &guest
}

// PutUserInContext puts the given user as a value in the context.
func PutUserInContext(ctx context.Context, u *User) context.Context {
	return context.WithValue(ctx, userkey, u)
}

// GetUser reads the current user from the request.
func GetUser(rq *http.Request) *User {
	return GetUserFromContext(rq.Context())
}
