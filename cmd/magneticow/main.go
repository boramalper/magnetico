package main

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path"
	"regexp"
	"sync"
	"syscall"
	"time"

	"github.com/pkg/errors"

	"github.com/Wessie/appdirs"
	"github.com/dustin/go-humanize"
	"github.com/gorilla/mux"
	"github.com/gorilla/schema"
	"github.com/jessevdk/go-flags"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"golang.org/x/crypto/bcrypt"

	"github.com/boramalper/magnetico/pkg/persistence"
)

var compiledOn string

// Set a Decoder instance as a package global, because it caches
// meta-data about structs, and an instance can be shared safely.
var decoder = schema.NewDecoder()

var templates map[string]*template.Template
var database persistence.Database

var opts struct {
	Addr     string
	Database string
	// Credentials is nil when no-auth cmd-line flag is supplied.
	Credentials        map[string][]byte // TODO: encapsulate credentials and mutex for safety
	CredentialsRWMutex sync.RWMutex
	// CredentialsPath is nil when no-auth is supplied.
	CredentialsPath string
	Verbosity       int
}

func main() {
	loggerLevel := zap.NewAtomicLevel()
	// Logging levels: ("debug", "info", "warn", "error", "dpanic", "panic", and "fatal").
	logger := zap.New(zapcore.NewCore(
		zapcore.NewConsoleEncoder(zap.NewDevelopmentEncoderConfig()),
		zapcore.Lock(os.Stderr),
		loggerLevel,
	))
	defer logger.Sync()
	zap.ReplaceGlobals(logger)

	zap.L().Info("magneticow v0.9.0 has been started.")
	zap.L().Info("Copyright (C) 2017-2019  Mert Bora ALPER <bora@boramalper.org>.")
	zap.L().Info("Dedicated to Cemile Binay, in whose hands I thrived.")
	zap.S().Infof("Compiled on %s", compiledOn)

	if err := parseFlags(); err != nil {
		zap.S().Errorf("error while parsing flags: %s", err.Error())
		return
	}

	switch opts.Verbosity {
	case 0:
		loggerLevel.SetLevel(zap.WarnLevel)
	case 1:
		loggerLevel.SetLevel(zap.InfoLevel)
	default: // Default: i.e. in case of 2 or more.
		// TODO: print the caller (function)'s name and line number!
		loggerLevel.SetLevel(zap.DebugLevel)
	}

	zap.ReplaceGlobals(logger)

	// Reload credentials when you receive SIGHUP
	sighupChan := make(chan os.Signal, 1)
	signal.Notify(sighupChan, syscall.SIGHUP)
	go func() {
		for range sighupChan {
			opts.CredentialsRWMutex.Lock()
			if opts.Credentials == nil {
				zap.L().Warn("Ignoring SIGHUP since `no-auth` was supplied")
				continue
			}

			opts.Credentials = make(map[string][]byte) // Clear opts.Credentials
			opts.CredentialsRWMutex.Unlock()
			if err := loadCred(opts.CredentialsPath); err != nil { // Reload credentials
				zap.L().Warn("couldn't load credentials", zap.Error(err))
			}
		}
	}()

	router := mux.NewRouter()
	router.HandleFunc("/",
		BasicAuth(rootHandler, "magneticow"))

	router.HandleFunc("/api/v0.1/statistics",
		BasicAuth(apiStatistics, "magneticow"))
	router.HandleFunc("/api/v0.1/torrents",
		BasicAuth(apiTorrents, "magneticow"))
	router.HandleFunc("/api/v0.1/torrents/{infohash:[a-f0-9]{40}}",
		BasicAuth(apiTorrent, "magneticow"))
	router.HandleFunc("/api/v0.1/torrents/{infohash:[a-f0-9]{40}}/filelist",
		BasicAuth(apiFilelist, "magneticow"))

	router.HandleFunc("/feed",
		BasicAuth(feedHandler, "magneticow"))
	router.PathPrefix("/static").HandlerFunc(
		BasicAuth(staticHandler, "magneticow"))
	router.HandleFunc("/statistics",
		BasicAuth(statisticsHandler, "magneticow"))
	router.HandleFunc("/torrents",
		BasicAuth(torrentsHandler, "magneticow"))
	router.HandleFunc("/torrents/{infohash:[a-f0-9]{40}}",
		BasicAuth(torrentsInfohashHandler, "magneticow"))

	templateFunctions := template.FuncMap{
		"add": func(augend int, addends int) int {
			return augend + addends
		},

		"subtract": func(minuend int, subtrahend int) int {
			return minuend - subtrahend
		},

		"bytesToHex": func(bytes []byte) string {
			return hex.EncodeToString(bytes)
		},

		"unixTimeToYearMonthDay": func(s int64) string {
			tm := time.Unix(s, 0)
			// > Format and Parse use example-based layouts. Usually you’ll use a constant from time
			// > for these layouts, but you can also supply custom layouts. Layouts must use the
			// > reference time Mon Jan 2 15:04:05 MST 2006 to show the pattern with which to
			// > format/parse a given time/string. The example time must be exactly as shown: the
			// > year 2006, 15 for the hour, Monday for the day of the week, etc.
			// https://gobyexample.com/time-formatting-parsing
			// Why you gotta be so weird Go?
			return tm.Format("02/01/2006")
		},

		"humanizeSize": func(s uint64) string {
			return humanize.IBytes(s)
		},

		"humanizeSizeF": func(s int64) string {
			if s < 0 {
				return ""
			}
			return humanize.IBytes(uint64(s))
		},

		"comma": func(s uint) string {
			return humanize.Comma(int64(s))
		},
	}

	templates = make(map[string]*template.Template)
	templates["feed"] = template.Must(template.New("feed").Funcs(templateFunctions).Parse(string(mustAsset("templates/feed.xml"))))
	templates["homepage"] = template.Must(template.New("homepage").Funcs(templateFunctions).Parse(string(mustAsset("templates/homepage.html"))))

	var err error
	database, err = persistence.MakeDatabase(opts.Database, logger)
	if err != nil {
		zap.L().Fatal("could not access to database", zap.Error(err))
	}

	decoder.IgnoreUnknownKeys(false)
	decoder.ZeroEmpty(true)

	zap.S().Infof("magneticow is ready to serve on %s!", opts.Addr)
	err = http.ListenAndServe(opts.Addr, router)
	if err != nil {
		zap.L().Error("ListenAndServe error", zap.Error(err))
	}
}

// TODO: I think there is a standard lib. function for this
func respondError(w http.ResponseWriter, statusCode int, format string, a ...interface{}) {
	w.WriteHeader(statusCode)
	w.Write([]byte(fmt.Sprintf(format, a...)))
}

func mustAsset(name string) []byte {
	data, err := Asset(name)
	if err != nil {
		zap.L().Panic("Could NOT access the requested resource! THIS IS A BUG, PLEASE REPORT",
			zap.String("name", name), zap.Error(err))
	}
	return data
}

func parseFlags() error {
	var cmdFlags struct {
		Addr     string `short:"a" long:"addr"        description:"Address (host:port) to serve on"  default:":8080"`
		Database string `short:"d" long:"database"    description:"URL of the (magneticod) database"`
		Cred     string `short:"c" long:"credentials" description:"Path to the credentials file"`
		NoAuth   bool   `          long:"no-auth"     description:"Disables authorisation"`

		Verbose []bool `short:"v" long:"verbose" description:"Increases verbosity."`
	}

	if _, err := flags.Parse(&cmdFlags); err != nil {
		return err
	}

	if cmdFlags.Cred != "" && cmdFlags.NoAuth {
		return fmt.Errorf("`credentials` and `no-auth` cannot be supplied together")
	}

	opts.Addr = cmdFlags.Addr

	if cmdFlags.Database == "" {
		opts.Database =
			"sqlite3://" +
				appdirs.UserDataDir("magneticod", "", "", false) +
				"/database.sqlite3" +
				"?_journal_mode=WAL" // https://github.com/mattn/go-sqlite3#connection-string
	} else {
		opts.Database = cmdFlags.Database
	}

	if !cmdFlags.NoAuth {
		// Set opts.CredentialsPath to either the default value (computed by appdirs pkg) or to the one
		// supplied by the user.
		if cmdFlags.Cred == "" {
			opts.CredentialsPath = path.Join(
				appdirs.UserConfigDir("magneticow", "", "", false),
				"credentials",
			)
		} else {
			opts.CredentialsPath = cmdFlags.Cred
		}

		opts.Credentials = make(map[string][]byte)
		if err := loadCred(opts.CredentialsPath); err != nil {
			return err
		}
	}

	opts.Verbosity = len(cmdFlags.Verbose)

	return nil
}

func loadCred(cred string) error {
	file, err := os.Open(cred)
	if err != nil {
		return err
	}

	opts.CredentialsRWMutex.Lock()
	defer opts.CredentialsRWMutex.Unlock()

	reader := bufio.NewReader(file)
	for lineno := 1; true; lineno++ {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return errors.Wrapf(err, "while reading line %d", lineno)
		}

		line = line[:len(line)-1] // strip '\n'

		/* The following regex checks if the line satisfies the following conditions:
		 *
		 * <USERNAME>:<BCRYPT HASH>
		 *
		 * where
		 *     <USERNAME> must start with a small-case a-z character, might contain non-consecutive
		 *   underscores in-between, and consists of small-case a-z characters and digits 0-9.
		 *
		 *     <BCRYPT HASH> is the output of the well-known bcrypt function.
		 */
		re := regexp.MustCompile(`^[a-z](?:_?[a-z0-9])*:\$2[aby]?\$\d{1,2}\$[./A-Za-z0-9]{53}$`)
		if !re.Match(line) {
			return fmt.Errorf("on line %d: format should be: <USERNAME>:<BCRYPT HASH>, instead got: %s", lineno, line)
		}

		tokens := bytes.Split(line, []byte(":"))
		opts.Credentials[string(tokens[0])] = tokens[1]
	}

	return nil
}

// BasicAuth wraps a handler requiring HTTP basic auth for it using the given
// username and password and the specified realm, which shouldn't contain quotes.
//
// Most web browser display a dialog with something like:
//
//    The website says: "<realm>"
//
// Which is really stupid so you may want to set the realm to a message rather than
// an actual realm.
//
// Source: https://stackoverflow.com/a/39591234/4466589
func BasicAuth(handler http.HandlerFunc, realm string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if opts.Credentials == nil { // --no-auth is supplied by the user.
			handler(w, r)
			return
		}

		username, password, ok := r.BasicAuth()
		if !ok { // No credentials provided
			authenticate(w, realm)
			return
		}

		opts.CredentialsRWMutex.RLock()
		hashedPassword, ok := opts.Credentials[username]
		opts.CredentialsRWMutex.RUnlock()
		if !ok { // User not found
			authenticate(w, realm)
			return
		}

		if err := bcrypt.CompareHashAndPassword(hashedPassword, []byte(password)); err != nil { // Wrong password
			authenticate(w, realm)
			return
		}

		handler(w, r)
	}
}

func authenticate(w http.ResponseWriter, realm string) {
	w.Header().Set("WWW-Authenticate", `Basic realm="`+realm+`"`)
	w.WriteHeader(401)
	_, _ = w.Write([]byte("Unauthorised.\n"))
}
