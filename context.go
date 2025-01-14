package fronted

import (
	"crypto/x509"
	"fmt"
	"net/http"
	"time"

	tls "github.com/refraction-networking/utls"

	"github.com/getlantern/eventual"
)

var (
	DefaultContext = NewFrontingContext("default")
)

// Configure sets the masquerades to use, the trusted root CAs, and the
// cache file for caching masquerades to set up direct domain fronting
// in the default context.
//
// defaultProviderID is used when a masquerade without a provider is
// encountered (eg in a cache file)
func Configure(pool *x509.CertPool, providers map[string]*Provider, defaultProviderID string, cacheFile string) {
	if err := DefaultContext.Configure(pool, providers, defaultProviderID, cacheFile); err != nil {
		log.Errorf("Error configuring fronting %s context: %s!!", DefaultContext.name, err)
	}
}

// NewFronted creates a new http.RoundTripper that does direct domain fronting
// using the default context. If the default context isn't configured within
// the given timeout, this method returns nil, false.
func NewFronted(timeout time.Duration) (http.RoundTripper, bool) {
	return DefaultContext.NewFronted(timeout)
}

// Close closes any existing cache file in the default context
func Close() {
	DefaultContext.Close()
}

func NewFrontingContext(name string) *FrontingContext {
	return &FrontingContext{
		name:     name,
		instance: eventual.NewValue(),
	}
}

type FrontingContext struct {
	name     string
	instance eventual.Value
}

// Configure sets the masquerades to use, the trusted root CAs, and the
// cache file for caching masquerades to set up direct domain fronting.
// defaultProviderID is used when a masquerade without a provider is
// encountered (eg in a cache file)
func (fctx *FrontingContext) Configure(pool *x509.CertPool, providers map[string]*Provider, defaultProviderID string, cacheFile string) error {
	return fctx.ConfigureWithHello(pool, providers, defaultProviderID, cacheFile, tls.ClientHelloID{})
}

func (fctx *FrontingContext) ConfigureWithHello(pool *x509.CertPool, providers map[string]*Provider, defaultProviderID string, cacheFile string, clientHelloID tls.ClientHelloID) error {
	log.Debugf("Configuring fronted %s context", fctx.name)

	if len(providers) == 0 {
		return fmt.Errorf("no fronted providers for %s context", fctx.name)
	}

	_existing, ok := fctx.instance.Get(0)
	if ok && _existing != nil {
		existing := _existing.(*fronted)
		log.Debugf("Closing cache from existing instance for %s context", fctx.name)
		existing.closeCache()
	}

	_, err := newFronted(pool, providers, defaultProviderID, cacheFile, clientHelloID, func(f *fronted) {
		fctx.instance.Set(f)
	})
	if err != nil {
		return err
	}
	return nil
}

// NewFronted creates a new http.RoundTripper that does direct domain fronting.
// If the context isn't configured within the given timeout, this method
// returns nil, false.
func (fctx *FrontingContext) NewFronted(timeout time.Duration) (http.RoundTripper, bool) {
	start := time.Now()
	instance, ok := fctx.instance.Get(timeout)
	if !ok {
		log.Errorf("No DirectHttpClient available within %v for context %s", timeout, fctx.name)
		return nil, false
	} else {
		log.Debugf("DirectHttpClient available for context %s after %v", fctx.name, time.Since(start))
	}
	return instance.(http.RoundTripper), true
}

// Close closes any existing cache file in the default contexxt.
func (fctx *FrontingContext) Close() {
	_existing, ok := fctx.instance.Get(0)
	if ok && _existing != nil {
		existing := _existing.(*fronted)
		log.Debugf("Closing cache from existing instance in %s context", fctx.name)
		existing.closeCache()
	}
}
