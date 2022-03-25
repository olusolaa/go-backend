package middleware

import (
	"fmt"
	"github.com/cespare/xxhash/v2"
	"github.com/olusolaa/go-backend/pkg"
	"math"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

func Limit(requestLimit int, windowLength time.Duration, options ...Option) func(next http.Handler) http.Handler {
	return NewRateLimiter(requestLimit, windowLength, options...).Handler
}

type KeyFunc func(r *http.Request) (string, error)
type Option func(rl *rateLimiter)

func LimitAll(requestLimit int, windowLength time.Duration) func(next http.Handler) http.Handler {
	return Limit(requestLimit, windowLength)
}

func LimitByIP(requestLimit int, windowLength time.Duration) func(next http.Handler) http.Handler {
	return Limit(requestLimit, windowLength, WithKeyFuncs(KeyByIP))
}

func KeyByIP(r *http.Request) (string, error) {
	var ip string

	if tcip := r.Header.Get("True-Client-IP"); tcip != "" {
		ip = tcip
	} else if xrip := r.Header.Get("X-Real-IP"); xrip != "" {
		ip = xrip
	} else if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		i := strings.Index(xff, ", ")
		if i == -1 {
			i = len(xff)
		}
		ip = xff[:i]
	} else {
		var err error
		ip, _, err = net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			ip = r.RemoteAddr
		}
	}

	return canonicalizeIP(ip), nil
}

func KeyByEndpoint(r *http.Request) (string, error) {
	return r.URL.Path, nil
}

func KeyByFrom(r *http.Request) (string, error) {
	pkg.DecodePostRequest()
	return pkg.GetDecodedPostRequest().From, nil
}

func WithKeyFuncs(keyFuncs ...KeyFunc) Option {
	return func(rl *rateLimiter) {
		if len(keyFuncs) > 0 {
			rl.keyFn = composedKeyFunc(keyFuncs...)
		}
	}
}

func WithLimitHandler(h http.HandlerFunc) Option {
	return func(rl *rateLimiter) {
		rl.onRequestLimit = h
	}
}

func WithLimitCounter(c LimitCounter) Option {
	return func(rl *rateLimiter) {
		rl.limitCounter = c
	}
}

func composedKeyFunc(keyFuncs ...KeyFunc) KeyFunc {
	return func(r *http.Request) (string, error) {
		var key strings.Builder
		for i := 0; i < len(keyFuncs); i++ {
			k, err := keyFuncs[i](r)
			if err != nil {
				return "", err
			}
			key.WriteString(k)
		}
		return key.String(), nil
	}
}

// canonicalizeIP returns a form of ip suitable for comparison to other IPs.
// For IPv4 addresses, this is simply the whole string.
// For IPv6 addresses, this is the /64 prefix.
func canonicalizeIP(ip string) string {
	isIPv6 := false
	// This is how net.ParseIP decides if an address is IPv6
	// https://cs.opensource.google/go/go/+/refs/tags/go1.17.7:src/net/ip.go;l=704
	for i := 0; !isIPv6 && i < len(ip); i++ {
		switch ip[i] {
		case '.':
			// IPv4
			return ip
		case ':':
			// IPv6
			isIPv6 = true
			break
		}
	}
	if !isIPv6 {
		// Not an IP address at all
		return ip
	}

	ipv6 := net.ParseIP(ip)
	if ipv6 == nil {
		return ip
	}

	return ipv6.Mask(net.CIDRMask(64, 128)).String()
}

type LimitCounter interface {
	Increment(key string, currentWindow time.Time) error
	Get(key string, currentWindow, previousWindow time.Time) (int, int, error)
}

func NewRateLimiter(requestLimit int, windowLength time.Duration, options ...Option) *rateLimiter {
	return newRateLimiter(requestLimit, windowLength, options...)
}

func newRateLimiter(requestLimit int, windowLength time.Duration, options ...Option) *rateLimiter {
	rl := &rateLimiter{
		requestLimit: requestLimit,
		windowLength: windowLength,
	}

	for _, opt := range options {
		opt(rl)
	}

	if rl.keyFn == nil {
		rl.keyFn = func(r *http.Request) (string, error) {
			return "*", nil
		}
	}

	if rl.limitCounter == nil {
		rl.limitCounter = &localCounter{
			counters:     make(map[uint64]*count),
			windowLength: windowLength,
		}
	}

	if rl.onRequestLimit == nil {
		rl.onRequestLimit = func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, http.StatusText(http.StatusTooManyRequests), http.StatusTooManyRequests)
		}
	}

	return rl
}

func LimitCounterKey(key string, window time.Time) uint64 {
	h := xxhash.New()
	h.WriteString(key)
	h.WriteString(fmt.Sprintf("%d", window.Unix()))
	return h.Sum64()
}

type rateLimiter struct {
	requestLimit   int
	windowLength   time.Duration
	keyFn          KeyFunc
	limitCounter   LimitCounter
	onRequestLimit http.HandlerFunc
}

func (r *rateLimiter) Counter() LimitCounter {
	return r.limitCounter
}

func (r *rateLimiter) Status(key string) (bool, float64, error) {
	t := time.Now().UTC()
	currentWindow := t.Truncate(r.windowLength)
	previousWindow := currentWindow.Add(-r.windowLength)

	currCount, prevCount, err := r.limitCounter.Get(key, currentWindow, previousWindow)
	if err != nil {
		return false, 0, err
	}

	diff := t.Sub(currentWindow)
	rate := float64(prevCount)*(float64(r.windowLength)-float64(diff))/float64(r.windowLength) + float64(currCount)

	if rate > float64(r.requestLimit) {
		return false, rate, nil
	}
	return true, rate, nil
}

func (l *rateLimiter) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key, err := l.keyFn(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusPreconditionRequired)
			return
		}

		currentWindow := time.Now().UTC().Truncate(l.windowLength)

		w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", l.requestLimit))
		w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", 0))
		w.Header().Set("X-RateLimit-Reset", fmt.Sprintf("%d", currentWindow.Add(l.windowLength).Unix()))

		_, rate, err := l.Status(key)
		if err != nil {
			http.Error(w, err.Error(), http.StatusPreconditionRequired)
			return
		}
		nrate := int(math.Round(rate))

		if l.requestLimit > nrate {
			w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", l.requestLimit-nrate))
		}

		if nrate >= l.requestLimit {
			w.Header().Set("Retry-After", fmt.Sprintf("%d", int(l.windowLength.Seconds()))) // RFC 6585
			l.onRequestLimit(w, r)
			return
		}

		err = l.limitCounter.Increment(key, currentWindow)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		next.ServeHTTP(w, r)
	})
}

type localCounter struct {
	counters     map[uint64]*count
	windowLength time.Duration
	lastEvict    time.Time
	mu           sync.Mutex
}

var _ LimitCounter = &localCounter{}

type count struct {
	value     int
	updatedAt time.Time
}

func (c *localCounter) Increment(key string, currentWindow time.Time) error {
	c.evict()

	c.mu.Lock()
	defer c.mu.Unlock()

	hkey := LimitCounterKey(key, currentWindow)

	v, ok := c.counters[hkey]
	if !ok {
		v = &count{}
		c.counters[hkey] = v
	}
	v.value += 1
	v.updatedAt = time.Now()

	return nil
}

func (c *localCounter) Get(key string, currentWindow, previousWindow time.Time) (int, int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	curr, ok := c.counters[LimitCounterKey(key, currentWindow)]
	if !ok {
		curr = &count{value: 0, updatedAt: time.Now()}
	}
	prev, ok := c.counters[LimitCounterKey(key, previousWindow)]
	if !ok {
		prev = &count{value: 0, updatedAt: time.Now()}
	}

	return curr.value, prev.value, nil
}

func (c *localCounter) evict() {
	c.mu.Lock()
	defer c.mu.Unlock()

	d := c.windowLength * 3

	if time.Since(c.lastEvict) < d {
		return
	}
	c.lastEvict = time.Now()

	for k, v := range c.counters {
		if time.Since(v.updatedAt) >= d {
			delete(c.counters, k)
		}
	}
}
