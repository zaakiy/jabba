package jabba

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"github.com/hako/durafmt"
	"github.com/rs/zerolog"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

//Version is the server version
var Version string = "v0.6.1"

//ID is a unique server ID
var ID string = "unknown"

//Runtime struct defines runtime environment wrapper for a config.
type Runtime struct {
	Config
}

//Runner is the Live environment of the server
var Runner *Runtime

var Boot sync.WaitGroup = sync.WaitGroup{}

//BootStrap starts up the server from a ServerConfig
func BootStrap() {
	initLogger()

	config := new(Config).
		read(ConfigFile).
		reApplyResourceSchemes().
		reApplyResourceNames().
		compileRoutePaths().
		sortRoutes().
		addDefaultPolicy().
		setDefaultUpstreamParams().
		setDefaultDownstreamParams()

	Runner = &Runtime{Config: *config}
	Runner.initStats().
		initUserAgent().
		startListening()
}

func (runtime Runtime) startListening() {
	readTimeoutDuration := time.Second * time.Duration(runtime.Connection.Downstream.ReadTimeoutSeconds)
	roundTripTimeoutDuration := time.Second * time.Duration(runtime.Connection.Downstream.RoundTripTimeoutSeconds)
	roundTripTimeoutDurationWithGrace := roundTripTimeoutDuration + (time.Second * 1)
	idleTimeoutDuration := time.Second * time.Duration(runtime.Connection.Downstream.IdleTimeoutSeconds)

	log.Debug().
		Float64("dwnReadTimeoutSeconds", readTimeoutDuration.Seconds()).
		Float64("dwnRoundTripTimeoutSeconds", roundTripTimeoutDuration.Seconds()).
		Float64("dwnIdleConnTimeoutSeconds", idleTimeoutDuration.Seconds()).
		Msg("server derived downstream params")

	server := &http.Server{
		Addr:              ":" + strconv.Itoa(runtime.Connection.Downstream.Port),
		ReadHeaderTimeout: readTimeoutDuration,
		ReadTimeout:       readTimeoutDuration,
		WriteTimeout:      roundTripTimeoutDurationWithGrace,
		IdleTimeout:       idleTimeoutDuration,
		Handler:           runtime.mapPathsToHandler(),
	}

	//signal the WaitGroup that boot is over.
	Boot.Done()

	//this line blocks execution and the server stays up
	var err error

	if runtime.isTLSMode() {
		server.TLSConfig = runtime.tlsConfig()
		log.Info().Msgf("Jabba %s listening in TLS mode on port %d...", Version, runtime.Connection.Downstream.Port)
		err = server.ListenAndServeTLS("", "")
	} else {
		log.Info().Msgf("Jabba %s listening on port %d...", Version, runtime.Connection.Downstream.Port)
		err = server.ListenAndServe()
	}

	if err != nil {
		log.Fatal().Err(err).Msgf("unable to start HTTP server on port %d, exiting...", runtime.Connection.Downstream.Port)
		panic(err.Error())
	}
}

func (runtime Runtime) mapPathsToHandler() http.Handler {
	//TODO: do we need this handler with two handlerfuncs or can we map all requests to one handlerfunc to speed up?
	//if one handlerfunc in the system, it would need to distinguish between /about and other routes.

	handler := http.NewServeMux()
	for _, route := range runtime.Routes {
		if route.Resource == AboutJabba {
			handler.Handle(route.Path, http.HandlerFunc(aboutHandler))
			log.Debug().Msgf("assigned about handler to path %s", route.Path)
		}
	}
	handler.Handle("/", http.HandlerFunc(proxyHandler))
	log.Debug().Msgf("assigned proxy handler to path %s", "/")

	return handler
}

func (runtime Runtime) initUserAgent() Runtime {
	if httpClient == nil {
		httpClient = scaffoldHTTPClient(runtime)
	}
	return runtime
}

func (runtime Runtime) initStats() Runtime {
	go stats(os.Getpid())
	return runtime
}

//TODO: this really needs to be configurable. it adds a lot of options to every response otherwise.
func (proxy *Proxy) writeStandardResponseHeaders() {
	header := proxy.Dwn.Resp.Writer.Header()

	header.Set("Server", fmt.Sprintf("Jabba %s %s", Version, ID))
	header.Set("Cache-control:", "no-store, no-cache, must-revalidate, proxy-revalidate")
	//for TLS response, we set HSTS header see RFC6797
	if Runner.isTLSMode() {
		header.Set("Strict-Transport-Security", "max-age=31536000")
	}
	header.Set("X-xss-protection", "1;mode=block")
	header.Set("X-content-type-options", "nosniff")
	header.Set("X-frame-options", "sameorigin")
	//copy the X-REQUEST-ID from the request
	header.Set(XRequestID, proxy.XRequestID)
}

func (runtime Runtime) tlsConfig() *tls.Config {
	defer func() {
		if err := recover(); err != nil {
			log.Fatal().Msg("unable to parse TLS configuration, check your certificate and/or private key. Jabba is exiting ...")
			os.Exit(-1)
		}
	}()

	//here we create a keypair from the PEM string in the config file
	var cert []byte = []byte(runtime.Connection.Downstream.Cert)
	var key []byte = []byte(runtime.Connection.Downstream.Key)
	kp, _ := tls.X509KeyPair(cert, key)

	//parse the first cert in the config, so we can report on it. we assume certs are in order of cert, intermediate, root.
	kp.Leaf, _ = x509.ParseCertificate(kp.Certificate[0])

	//now create the TLS config.
	config := &tls.Config{
		MinVersion:               tls.VersionTLS12,
		CurvePreferences:         []tls.CurveID{tls.CurveP521, tls.CurveP384, tls.CurveP256},
		PreferServerCipherSuites: true,
		CipherSuites: []uint16{
			//TLS 1.3 good ciphers
			tls.TLS_AES_256_GCM_SHA384,
			tls.TLS_AES_128_GCM_SHA256,
			tls.TLS_CHACHA20_POLY1305_SHA256,
			//TLS 1.2 good ciphers
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			//TLS 1.2 weak ciphers for IE11, Safari 6-8. We keep this for backwards compatibility with older
			//clients, it still gives us an A+ result on: https://www.ssllabs.com/ssltest/analyze.html?d=j8a.io
			tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256,
		},
		Certificates: []tls.Certificate{kp},
	}

	//and tell our users what we found.
	until := time.Until(kp.Leaf.NotAfter)
	monthy := time.Hour * 24 * 30
	var ev *zerolog.Event

	//if the certificate expires in less than 30 days we send this as a log.Warn event instead.
	if until < monthy {
		ev = log.Warn()
	} else {
		ev = log.Debug()
	}
	ev.Msgf("parsed TLS certificate for DNS names %s from issuer %s, expires in %s",
		kp.Leaf.DNSNames,
		kp.Leaf.Issuer.CommonName,
		durafmt.Parse(until).LimitFirstN(3).String(),
	)
	return config
}

func sendStatusCodeAsJSON(proxy *Proxy) {
	proxy.writeStandardResponseHeaders()
	proxy.Dwn.Resp.Writer.Header().Set("Content-Type", "application/json")
	proxy.writeContentEncodingHeader()

	proxy.Dwn.Resp.Writer.WriteHeader(proxy.Dwn.Resp.StatusCode)

	statusCodeResponse := StatusCodeResponse{
		Code:       proxy.Dwn.Resp.StatusCode,
		Message:    proxy.Dwn.Resp.Message,
		XRequestID: proxy.XRequestID,
	}

	if len(proxy.Dwn.Resp.Message) == 0 || proxy.Dwn.Resp.Message == "none" {
		statusCodeResponse.withCode(proxy.Dwn.Resp.StatusCode)
		proxy.Dwn.Resp.Message = statusCodeResponse.Message
	}

	if proxy.Dwn.Resp.SendGzip {
		proxy.Dwn.Resp.Writer.Write(Gzip(statusCodeResponse.AsJSON()))
	} else {
		proxy.Dwn.Resp.Writer.Write(statusCodeResponse.AsJSON())
	}

	logHandledDownstreamRoundtrip(proxy)
}
