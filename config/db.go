package config

import (
	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"os"

	_ "github.com/jackc/pgx/stdlib"
)

var (
	mdb, fdb           *sqlx.DB // mdb,fdb
	defaultDbConfigOpt = DBConfigOption{
		Paths: []string{"./config", "."},
		Name:  "database",
	}
)

type DBConfigOption struct {
	Paths []string
	Name  string
}

//setDBConfigOpt adds default values
func setDBConfigOpt(opt DBConfigOption) DBConfigOption {
	if len(opt.Paths) == 0 {
		opt.Paths = make([]string, len(defaultDbConfigOpt.Paths))
		copy(opt.Paths, defaultDbConfigOpt.Paths)
	}

	if opt.Name == "" {
		opt.Name = defaultDbConfigOpt.Name
	}

	return opt
}

type dbConnConfig struct {
	Env      string
	URL      string
	Dialect  string // dialect defaults to postgres
	Pool     int    // Defaults to 0 "unlimited". See https://golang.org/pkg/database/sql/#DB.SetMaxOpenConns
	IdlePool int    // Defaults to 2. See https://golang.org/pkg/database/sql/#DB.SetMaxIdleConns
	Unsafe   bool   // Defaults to `false`. See https://godoc.org/github.com/jmoiron/sqlx#DB.Unsafe
}

func initDbConfig(opts ...DBConfigOption) (map[string]*dbConnConfig, error) {
	var opt DBConfigOption

	if len(opts) != 0 {
		opt = opts[0]
	}
	opt = setDBConfigOpt(opt)

	dbViper := viper.New()
	dbViper.SetConfigName(opt.Name)

	for _, path := range opt.Paths {
		dbViper.AddConfigPath(path)
	}

	if err := dbViper.ReadInConfig(); err != nil {
		panic(err)
	}

	var conns map[string]*dbConnConfig

	err := dbViper.Unmarshal(&conns)
	if err != nil {
		return nil, err
	}

	for _, v := range conns {
		//	env is set use it
		if v.Env != "" {
			v.URL = viper.GetString(v.Env)
		}

		// set dialect
		if v.Dialect == "" {
			v.Dialect = "pgx"
		}
	}

	return conns, nil
}

func newDB(env string) (*sqlx.DB, error) {
	dbConn, err := initDbConfig()
	if err != nil {
		return nil, err
	}
	conn, ok := dbConn[env]
	if !ok {
		conn, ok = dbConn["development"]
		if !ok {
			return nil, errors.New("can't find connection " + env)
		}
	}

	db, err := sqlx.Connect(conn.Dialect, conn.URL)
	if err != nil {
		return nil, err
	}

	//
	if conn.Pool != 0 {
		db.SetMaxOpenConns(conn.Pool)
	}

	if conn.IdlePool <= 0 {
		db.SetMaxIdleConns(2)
	} else {
		db.SetMaxIdleConns(conn.IdlePool)
	}

	if conn.Unsafe {
		db = db.Unsafe()
	}

	return db, nil
}

// NewDB ...
func NewDB() {
	var err error
	mdb, err = newDB(os.Getenv(Env))
	if err != nil {
		logrus.WithField("context", "postgres_db_init").Panic(err)
	}

	fdb, err = newDB(os.Getenv(Env) + "_follower")
	if err != nil {
		logrus.WithField("context", "postgres_follower_db_init").Panic(err)
	}

	closeFn := func() {
		logrus.Info("closing pg conn")

		//close main db
		err := mdb.Close()
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"context": "close_pg_conn",
				"method":  "config/close",
			}).Error(err)
		}

		//close follower db
		err = fdb.Close()
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"context": "close_pg_conn",
				"method":  "config/close",
			}).Error(err)
		}
	}

	closeFns = append(closeFns, closeFn)
}

// GetDB returns the db instance
func GetDB() *sqlx.DB {
	return mdb
}

func GetFollowerDB() *sqlx.DB {
	return fdb
}
