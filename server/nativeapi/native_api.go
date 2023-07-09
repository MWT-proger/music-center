package nativeapi

import (
	"context"
	"net/http"

	"github.com/deluan/rest"
	"github.com/go-chi/chi/v5"
	"github.com/navidrome/navidrome/conf"
	"github.com/navidrome/navidrome/core"
	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/server"
)

type Router struct {
	http.Handler
	ds    model.DataStore
	share core.Share
}

func New(ds model.DataStore, share core.Share) *Router {
	r := &Router{ds: ds, share: share}
	r.Handler = r.routes()
	return r
}

func (n *Router) routes() http.Handler {
	r := chi.NewRouter()

	// Public
	n.RX(r, "/translation", newTranslationRepository, false, false)

	// Protected
	r.Group(func(r chi.Router) {
		r.Use(server.JWTRefresher)
		n.R(r, "/song", model.MediaFile{}, false, false)
		n.R(r, "/album", model.Album{}, false, false)
		n.R(r, "/artist", model.Artist{}, false, false)
		n.R(r, "/genre", model.Genre{}, false, false)
		n.R(r, "/player", model.Player{}, true, false)

		n.R(r, "/transcoding", model.Transcoding{}, conf.Server.EnableTranscodingConfig, false)
		n.R(r, "/radio", model.Radio{}, true, false)
		if conf.Server.EnableSharing {
			n.RX(r, "/share", n.share.NewRepository, true, false)
		}

		// Keepalive endpoint to be used to keep the session valid (ex: while playing songs)
		r.Get("/keepalive/*", func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"response":"ok", "id":"keepalive"}`))
		})
	})
	r.Group(func(r chi.Router) {
		r.Use(server.Authenticator(n.ds))
		r.Use(server.JWTRefresher)
		n.R(r, "/user", model.User{}, true, false)
		n.R(r, "/playlist", model.Playlist{}, true, false)
		n.addPlaylistTrackRoute(r)

	})

	r.Group(func(r chi.Router) {
		n.R(r, "/user/new", model.User{}, true, true)
	})

	return r
}

func (n *Router) R(r chi.Router, pathPrefix string, model interface{}, persistable bool, onlyPost bool) {
	constructor := func(ctx context.Context) rest.Repository {
		return n.ds.Resource(ctx, model)
	}
	n.RX(r, pathPrefix, constructor, persistable, onlyPost)
}

func (n *Router) RX(r chi.Router, pathPrefix string, constructor rest.RepositoryConstructor, persistable bool, onlyPost bool) {
	r.Route(pathPrefix, func(r chi.Router) {
		if !onlyPost {
			r.Get("/", rest.GetAll(constructor))
		}

		if persistable {
			r.Post("/", rest.Post(constructor))
		}
		if !onlyPost {
			r.Route("/{id}", func(r chi.Router) {
				r.Use(server.URLParamsMiddleware)
				r.Get("/", rest.Get(constructor))
				if persistable {
					r.Put("/", rest.Put(constructor))
					r.Delete("/", rest.Delete(constructor))
				}
			})
		}
	})
}

func (n *Router) addPlaylistTrackRoute(r chi.Router) {
	r.Route("/playlist/{playlistId}/tracks", func(r chi.Router) {
		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
			getPlaylist(n.ds)(w, r)
		})
		r.With(server.URLParamsMiddleware).Route("/", func(r chi.Router) {
			r.Delete("/", func(w http.ResponseWriter, r *http.Request) {
				deleteFromPlaylist(n.ds)(w, r)
			})
			r.Post("/", func(w http.ResponseWriter, r *http.Request) {
				addToPlaylist(n.ds)(w, r)
			})
		})
		r.Route("/{id}", func(r chi.Router) {
			r.Use(server.URLParamsMiddleware)
			r.Put("/", func(w http.ResponseWriter, r *http.Request) {
				reorderItem(n.ds)(w, r)
			})
			r.Delete("/", func(w http.ResponseWriter, r *http.Request) {
				deleteFromPlaylist(n.ds)(w, r)
			})
		})
	})
}
