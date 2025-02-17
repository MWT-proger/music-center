package server

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/navidrome/navidrome/conf"
	"github.com/navidrome/navidrome/model"
	. "github.com/navidrome/navidrome/utils/gg"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/navidrome/navidrome/consts"
	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/model/request"
	"github.com/unrolled/secure"
)

func requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		scheme := "http"
		if r.TLS != nil {
			scheme = "https"
		}

		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)
		status := ww.Status()

		message := fmt.Sprintf("HTTP: %s %s://%s%s", r.Method, scheme, r.Host, r.RequestURI)
		logArgs := []interface{}{
			r.Context(),
			message,
			"remoteAddr", r.RemoteAddr,
			"elapsedTime", time.Since(start),
			"httpStatus", ww.Status(),
			"responseSize", ww.BytesWritten(),
		}
		if log.CurrentLevel() >= log.LevelDebug {
			logArgs = append(logArgs, "userAgent", r.UserAgent())
		}

		switch {
		case status >= 500:
			log.Error(logArgs...)
		case status >= 400:
			log.Warn(logArgs...)
		default:
			log.Debug(logArgs...)
		}
	})
}

func loggerInjector(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		ctx = log.NewContext(r.Context(), "requestId", middleware.GetReqID(ctx))
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func robotsTXT(fs fs.FS) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/robots.txt") {
				r.URL.Path = "/robots.txt"
				http.FileServer(http.FS(fs)).ServeHTTP(w, r)
			} else {
				next.ServeHTTP(w, r)
			}
		})
	}
}

type URL struct {
	XMLName    xml.Name `xml:"url"`
	LOC        string   `xml:"loc"`
	Lastmod    string   `xml:"lastmod,omitempty"`
	Changefreq string   `xml:"changefreq,omitempty"`
	Priority   string   `xml:"priority,omitempty"`
}

type URLSet struct {
	XMLName    xml.Name `xml:"urlset"`
	XMLNS      string   `xml:"xmlns,attr"`
	XMLNSXHTML string   `xml:"xmlns:xhtml,attr"`
	URL        []URL    `xml:"url"`
}

func sitemapXML(ds model.DataStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/sitemap.xml") {
				baseURL := fmt.Sprintf("https://%s/app/", conf.Server.DomenName)
				urls := []URL{}
				urls = append(urls,
					URL{LOC: baseURL + "album/all", Priority: "1.0"},
					URL{LOC: baseURL + "album/topRated", Priority: "1.0"},
					URL{LOC: baseURL + "album/recentlyAdded", Priority: "1.0"},
					URL{LOC: baseURL + "album/mostPlayed", Priority: "1.0"},
					URL{LOC: baseURL + "artist"},
					URL{LOC: baseURL + "song", Priority: "0.5"},
				)

				albums, err := ds.Album(context.TODO()).GetAll(model.QueryOptions{Max: 100})
				if err == nil {
					for _, album := range albums {
						albumURL := baseURL + "album/" + album.ID + "/show"
						urls = append(urls, URL{
							LOC:        albumURL,
							Lastmod:    album.UpdatedAt.Format("2006-01-02"),
							Changefreq: "always",
							Priority:   "0.8",
						})
					}
				}

				artists, err := ds.Artist(context.TODO()).GetAll(model.QueryOptions{Max: 100})
				if err == nil {
					for _, artist := range artists {
						artistURL := baseURL + "artist/" + artist.ID + "/show"
						urls = append(urls, URL{
							LOC:        artistURL,
							Changefreq: "always",
							Priority:   "0.8",
						})
					}
				}

				url_set := URLSet{
					XMLNS:      "http://www.sitemaps.org/schemas/sitemap/0.9",
					XMLNSXHTML: "http://www.w3.org/1999/xhtml",
					URL:        urls,
				}

				x, err := xml.MarshalIndent(url_set, "", "	")
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}

				w.Header().Set("Content-Type", "application/xml")
				w.Write(x)
			} else {
				next.ServeHTTP(w, r)
			}
		})
	}
}

func corsHandler() func(http.Handler) http.Handler {
	return cors.Handler(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{
			http.MethodHead,
			http.MethodGet,
			http.MethodPost,
			http.MethodPut,
			http.MethodPatch,
			http.MethodDelete,
		},
		AllowedHeaders:   []string{"*"},
		AllowCredentials: false,
		ExposedHeaders:   []string{"x-content-duration", "x-total-count", "x-nd-authorization"},
	})
}

func secureMiddleware() func(http.Handler) http.Handler {
	sec := secure.New(secure.Options{
		ContentTypeNosniff: true,
		FrameDeny:          true,
		ReferrerPolicy:     "same-origin",
		PermissionsPolicy:  "autoplay=(), camera=(), microphone=(), usb=()",
		//ContentSecurityPolicy: "script-src 'self' 'unsafe-inline'",
	})
	return sec.Handler
}

func compressMiddleware() func(http.Handler) http.Handler {
	return middleware.Compress(
		5,
		"application/xml",
		"application/json",
		"application/javascript",
		"text/html",
		"text/plain",
		"text/css",
		"text/javascript",
	)
}

// clientUniqueIDMiddleware is a middleware that sets a unique client ID as a cookie if it's provided in the request header.
// If the unique client ID is not in the header but present as a cookie, it adds the ID to the request context.
func clientUniqueIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		clientUniqueId := r.Header.Get(consts.UIClientUniqueIDHeader)

		// If clientUniqueId is found in the header, set it as a cookie
		if clientUniqueId != "" {
			c := &http.Cookie{
				Name:     consts.UIClientUniqueIDHeader,
				Value:    clientUniqueId,
				MaxAge:   consts.CookieExpiry,
				HttpOnly: true,
				Secure:   true,
				SameSite: http.SameSiteStrictMode,
				Path:     If(conf.Server.BasePath, "/"),
			}
			http.SetCookie(w, c)
		} else {
			// If clientUniqueId is not found in the header, check if it's present as a cookie
			c, err := r.Cookie(consts.UIClientUniqueIDHeader)
			if !errors.Is(err, http.ErrNoCookie) {
				clientUniqueId = c.Value
			}
		}

		// If a valid clientUniqueId is found, add it to the request context
		if clientUniqueId != "" {
			ctx = request.WithClientUniqueId(ctx, clientUniqueId)
			r = r.WithContext(ctx)
		}

		// Call the next middleware or handler in the chain
		next.ServeHTTP(w, r)
	})
}

// serverAddressMiddleware is a middleware function that modifies the request object
// to reflect the address of the server handling the request, as determined by the
// presence of X-Forwarded-* headers or the scheme and host of the request URL.
func serverAddressMiddleware(h http.Handler) http.Handler {
	// Define a new handler function that will be returned by this middleware function.
	fn := func(w http.ResponseWriter, r *http.Request) {
		// Call the serverAddress function to get the scheme and host of the server
		// handling the request. If a host is found, modify the request object to use
		// that host and scheme instead of the original ones.
		if rScheme, rHost := serverAddress(r); rHost != "" {
			r.Host = rHost
			r.URL.Scheme = rScheme
		}

		// Call the next handler in the chain with the modified request and response.
		h.ServeHTTP(w, r)
	}

	// Return the new handler function as an http.Handler object.
	return http.HandlerFunc(fn)
}

// Define constants for the X-Forwarded-* header keys.
var (
	xForwardedHost   = http.CanonicalHeaderKey("X-Forwarded-Host")
	xForwardedProto  = http.CanonicalHeaderKey("X-Forwarded-Proto")
	xForwardedScheme = http.CanonicalHeaderKey("X-Forwarded-Scheme")
)

// serverAddress is a helper function that returns the scheme and host of the server
// handling the given request, as determined by the presence of X-Forwarded-* headers
// or the scheme and host of the request URL.
func serverAddress(r *http.Request) (scheme, host string) {
	// Save the original request host for later comparison.
	origHost := r.Host

	// Determine the protocol of the request based on the presence of a TLS connection.
	protocol := "http"
	if r.TLS != nil {
		protocol = "https"
	}

	// Get the X-Forwarded-Host header and extract the first host name if there are
	// multiple hosts listed. If there is no X-Forwarded-Host header, use the original
	// request host as the default.
	xfh := r.Header.Get(xForwardedHost)
	if xfh != "" {
		i := strings.Index(xfh, ",")
		if i == -1 {
			i = len(xfh)
		}
		xfh = xfh[:i]
	}
	host = FirstOr(r.Host, xfh)

	// Determine the protocol and scheme of the request based on the presence of
	// X-Forwarded-* headers or the scheme of the request URL.
	scheme = FirstOr(
		protocol,
		r.Header.Get(xForwardedProto),
		r.Header.Get(xForwardedScheme),
		r.URL.Scheme,
	)

	// If the request host has changed due to the X-Forwarded-Host header, log a trace
	// message with the original and new host values, as well as the scheme and URL.
	if host != origHost {
		log.Trace(r.Context(), "Request host has changed", "origHost", origHost, "host", host, "scheme", scheme, "url", r.URL)
	}

	// Return the scheme and host of the server handling the request.
	return scheme, host
}

// URLParamsMiddleware is a middleware function that decodes the query string of
// the incoming HTTP request, adds the URL parameters from the routing context,
// and re-encodes the modified query string.
func URLParamsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Retrieve the routing context from the request context.
		ctx := chi.RouteContext(r.Context())

		// Parse the existing query string into a URL values map.
		params, _ := url.ParseQuery(r.URL.RawQuery)

		// Loop through each URL parameter in the routing context.
		for i, key := range ctx.URLParams.Keys {
			// Skip any wildcard URL parameter keys.
			if strings.Contains(key, "*") {
				continue
			}

			// Add the URL parameter key-value pair to the URL values map.
			params.Add(":"+key, ctx.URLParams.Values[i])
		}

		// Re-encode the URL values map as a query string and replace the
		// existing query string in the request.
		r.URL.RawQuery = params.Encode()

		// Call the next handler in the chain with the modified request and response.
		next.ServeHTTP(w, r)
	})
}
