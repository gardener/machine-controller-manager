package metalgo

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-openapi/runtime"
	httptransport "github.com/go-openapi/runtime/client"
	"github.com/go-openapi/strfmt"
	"github.com/metal-pod/metal-go/api/client/firewall"
	"github.com/metal-pod/metal-go/api/client/image"
	"github.com/metal-pod/metal-go/api/client/ip"
	"github.com/metal-pod/metal-go/api/client/machine"
	"github.com/metal-pod/metal-go/api/client/network"
	"github.com/metal-pod/metal-go/api/client/partition"
	"github.com/metal-pod/metal-go/api/client/project"
	"github.com/metal-pod/metal-go/api/client/size"
	sw "github.com/metal-pod/metal-go/api/client/switch_operations"
	"github.com/metal-pod/metal-go/api/models"
	"github.com/metal-pod/security"
	"github.com/pkg/errors"
)

// Driver holds the client connection to the metal api
type Driver struct {
	image     *image.Client
	project   *project.Client
	machine   *machine.Client
	firewall  *firewall.Client
	partition *partition.Client
	size      *size.Client
	sw        *sw.Client
	network   *network.Client
	ip        *ip.Client
	auth      runtime.ClientAuthInfoWriter
	bearer    string
	hmac      *security.HMACAuth
}

// NewDriver Create a new Driver for Metal to given url
func NewDriver(rawurl, bearer, hmac string) (*Driver, error) {
	roundTripper := &roundTripper{}

	parsedurl, err := url.Parse(rawurl)
	if err != nil {
		return nil, err
	}
	if parsedurl.Host == "" {
		return nil, fmt.Errorf("invalid url:%s, must be in the form scheme://host[:port]/basepath", rawurl)
	}

	transport := httptransport.New(parsedurl.Host, parsedurl.Path, []string{parsedurl.Scheme})
	transport.Transport = roundTripper

	driver := &Driver{
		machine:   machine.New(transport, strfmt.Default),
		firewall:  firewall.New(transport, strfmt.Default),
		size:      size.New(transport, strfmt.Default),
		image:     image.New(transport, strfmt.Default),
		partition: partition.New(transport, strfmt.Default),
		sw:        sw.New(transport, strfmt.Default),
		network:   network.New(transport, strfmt.Default),
		ip:        ip.New(transport, strfmt.Default),
		project:   project.New(transport, strfmt.Default),
		bearer:    bearer,
	}
	if hmac != "" {
		auth := security.NewHMACAuth("Metal-Admin", []byte(hmac))
		driver.hmac = &auth
	}
	driver.auth = runtime.ClientAuthInfoWriterFunc(driver.auther)

	return driver, nil
}

func (d *Driver) auther(rq runtime.ClientRequest, rg strfmt.Registry) error {
	if d.hmac != nil {
		d.hmac.AddAuthToClientRequest(rq, time.Now())
	} else if d.bearer != "" {
		security.AddUserTokenToClientRequest(rq, d.bearer)
	}
	return nil
}

// roundTripper is used to intercept api responses.
type roundTripper struct{}

// retryConfig is used to manage request retry behaviour.
type retryConfig struct {
	retriesLeft     uint
	delay           time.Duration
	retryConditions []func(statusCode int) bool
}

type retryKey string

const retryConfigKey = retryKey("retryConfig")

// newRetryContext creates and returns a new retry context.
// If assigned to a Swagger request that request will be retried up to maxAttempts with given delay
// in between as long as the responses indicate an error (i.e. status code >= 400) and none of the
// given retry conditions fails.
func newRetryContext(maxAttempts uint, delay time.Duration, retryConditions ...func(statusCode int) bool) context.Context {
	if maxAttempts == 0 {
		return nil
	}

	if delay <= 0 {
		delay = 5 * time.Second
	}

	rc := &retryConfig{
		retriesLeft:     maxAttempts,
		delay:           delay,
		retryConditions: retryConditions,
	}

	return context.WithValue(context.Background(), retryConfigKey, rc)
}

func (r *retryConfig) shouldRetryOn(statusCode int) bool {
	if r == nil || statusCode < 400 || r.retriesLeft == 0 {
		return false
	}

	for _, condition := range r.retryConditions {
		if !condition(statusCode) {
			return false
		}
	}

	return true
}

func (r *roundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	var reqBody []byte
	if req.Body != nil {
		var err error
		reqBody, err = ioutil.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		req.Body.Close()
		req.Body = ioutil.NopCloser(strings.NewReader(string(reqBody)))
	}

	resp, err := http.DefaultTransport.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	statusCode := resp.StatusCode

	rc, ok := req.Context().Value(retryConfigKey).(*retryConfig)
	if ok && rc.shouldRetryOn(statusCode) {
		time.Sleep(rc.delay)
		rc.retriesLeft--
		if reqBody != nil {
			req.Body = ioutil.NopCloser(strings.NewReader(string(reqBody)))
		}
		return r.RoundTrip(req)
	}

	switch statusCode {
	case http.StatusMovedPermanently, http.StatusTemporaryRedirect, http.StatusPermanentRedirect:
		return nil, errors.New("got redirect which is impossible")
	case http.StatusInternalServerError:
		return nil, errors.New("internal server error, please check server logs for details")
	default:
		if statusCode >= 400 {
			// domain errors must be in json to be able to distinguish from technical issues.
			respBody, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				return nil, errors.Wrap(err, "error reading error response")
			}
			resp.Body.Close()

			errorResponse := &models.HttperrorsHTTPErrorResponse{}
			err = json.Unmarshal(respBody, errorResponse)
			if err != nil {
				return nil, errors.Errorf("endpoint did not follow our error conventions:%s", string(respBody))
			}
			return nil, errors.New(*errorResponse.Message)
		}

		return resp, nil
	}
}
