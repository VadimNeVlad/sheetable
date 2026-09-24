package controllers

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path"
	"time"

	. "github.com/SheetAble/SheetAble/backend/api/config"
	"github.com/gin-gonic/gin"

	"github.com/jinzhu/gorm"
	"github.com/rs/cors"

	"github.com/gorilla/handlers"
	_ "github.com/jinzhu/gorm/dialects/mysql"    // mysql database driver
	_ "github.com/jinzhu/gorm/dialects/postgres" // postgres database driver
	_ "github.com/jinzhu/gorm/dialects/sqlite"
)

type Server struct {
	DB     *gorm.DB
	Router *gin.Engine
}

const (
	databaseConnectAttempts   = 10
	databaseRetryInitialDelay = time.Second
	databaseRetryMaximumDelay = 10 * time.Second
	gracefulShutdownTimeout   = 10 * time.Second
)

func (server *Server) Initialize() error {

	var err error
	if err = ensureApplicationDataDirectories(Config().ConfigPath); err != nil {
		return fmt.Errorf("prepare application data directories: %w", err)
	}

	// Set Release Mode
	if !Config().Dev {
		gin.SetMode(gin.ReleaseMode)
	}

	DbDriver := Config().Database.Driver
	DbUser := Config().Database.User
	DbPassword := Config().Database.Password
	DbHost := Config().Database.Host
	DbPort := Config().Database.Port
	DbName := Config().Database.Name

	var connection func() (*gorm.DB, error)
	switch DbDriver {
	case "mysql":
		DBURL := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8&parseTime=True&loc=Local", DbUser, DbPassword, DbHost, DbPort, DbName)
		connection = func() (*gorm.DB, error) {
			return gorm.Open(DbDriver, DBURL)
		}
	case "postgres":
		DBURL := fmt.Sprintf("host=%s port=%d user=%s dbname=%s sslmode=disable password=%s", DbHost, DbPort, DbUser, DbName, DbPassword)
		connection = func() (*gorm.DB, error) {
			return gorm.Open(DbDriver, DBURL)
		}
	default:
		databasePath := path.Join(Config().ConfigPath, "database.db")
		connection = func() (*gorm.DB, error) {
			return gorm.Open("sqlite3", databasePath)
		}
	}

	server.DB, err = openDatabaseWithRetry(
		databaseConnectAttempts,
		databaseRetryInitialDelay,
		databaseRetryMaximumDelay,
		connection,
		time.Sleep,
	)
	if err != nil {
		return fmt.Errorf("connect to %s database after %d attempts: %w", DbDriver, databaseConnectAttempts, err)
	}
	if DbDriver == "sqlite" || (DbDriver != "mysql" && DbDriver != "postgres") {
		fmt.Printf("Connected to %s database %s...\n", DbDriver, path.Join(Config().ConfigPath, "database.db"))
	} else {
		fmt.Printf("Connected to %s database...\n", DbDriver)
	}

	// Silence the logger
	server.DB.LogMode(false)

	server.SetupRouter()
	return nil
}

func ensureApplicationDataDirectories(configPath string) error {
	directories := []string{
		configPath,
		path.Join(configPath, "sheets"),
		path.Join(configPath, "sheets", "uploaded-sheets"),
		path.Join(configPath, "sheets", "thumbnails"),
		path.Join(configPath, "composer"),
	}
	for _, directory := range directories {
		if err := os.MkdirAll(directory, 0750); err != nil {
			return fmt.Errorf("create %s: %w", directory, err)
		}
	}
	return nil
}

func openDatabaseWithRetry(attempts int, initialDelay time.Duration, maximumDelay time.Duration, connect func() (*gorm.DB, error), sleep func(time.Duration)) (*gorm.DB, error) {
	if attempts < 1 {
		return nil, fmt.Errorf("database connection attempts must be at least one")
	}

	delay := initialDelay
	var lastError error
	for attempt := 1; attempt <= attempts; attempt++ {
		database, err := connect()
		if err == nil {
			return database, nil
		}
		lastError = err

		if attempt == attempts {
			break
		}
		log.Printf("database connection attempt %d/%d failed; retrying in %s: %v", attempt, attempts, delay, err)
		sleep(delay)
		delay *= 2
		if delay > maximumDelay {
			delay = maximumDelay
		}
	}

	return nil, lastError
}

func (server *Server) Run(ctx context.Context, addr string, dev bool) error {
	fmt.Printf("Listening to port %v\n", addr)
	/*
		cors.Default() setup the middleware with default options being
		all origins accepted with simple methods (GET, POST).
		See documentation below for more options.
	*/
	c := cors.New(cors.Options{
		// Enable Debugging for testing, consider disabling in production
		AllowedHeaders: []string{
			"Origin",
			"X-Requested-With",
			"Content-Type",
			"Accept",
			"Authorization",
		},
		AllowedMethods: []string{
			http.MethodHead,
			http.MethodGet,
			http.MethodPost,
			http.MethodPut,
			http.MethodPatch,
			http.MethodDelete,
		},
		AllowCredentials: true,
	})

	// Check if run in dev mode, so you can enable CORS or not
	srvHandler := handlers.LoggingHandler(os.Stdout, c.Handler(server.Router))

	if !dev {
		srvHandler = handlers.LoggingHandler(os.Stdout, server.Router)
	}

	srv := &http.Server{
		Handler:           srvHandler,
		Addr:              addr,
		WriteTimeout:      15 * time.Second,
		ReadTimeout:       15 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
	}

	serverStopped := make(chan struct{})
	shutdownFinished := make(chan struct{})
	go func() {
		defer close(shutdownFinished)
		select {
		case <-ctx.Done():
			shutdownContext, cancel := context.WithTimeout(context.Background(), gracefulShutdownTimeout)
			defer cancel()
			if err := srv.Shutdown(shutdownContext); err != nil {
				log.Printf("graceful HTTP shutdown failed: %v", err)
			}
		case <-serverStopped:
		}
	}()

	err := srv.ListenAndServe()
	close(serverStopped)
	<-shutdownFinished
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func (server *Server) Close() error {
	if server.DB == nil {
		return nil
	}
	return server.DB.Close()
}
