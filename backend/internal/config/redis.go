package config

import (
	"log"
	"os"
	"strconv"
)

type RedisConfig struct {
	Addr         string
	Password     string
	DB           int
	MaxEndpoints int64
}

const (
	// constants
	maxEndpoints = 100
	// env variables names
	envVarRedisAddr = "REDIS_ADDR"
	envVarRedisPass = "REDIS_PASS"
	envVarRedisDB   = "REDIS_DB"
	// variables for local env
	localRedisAddr = "localhost:6379"
	localRedisPass = ""
	localRedisDB   = 0
)

func GetRedisConfig(local bool) *RedisConfig {
	var addr, pass string
	var db int
	if local {
		addr = localRedisAddr
		pass = localRedisPass
		db = localRedisDB
	} else {
		addr = getRedisAddr()
		pass = getRedisPass()
		db = getRedisDB()
	}

	return &RedisConfig{
		Addr:         addr,
		Password:     pass,
		DB:           db,
		MaxEndpoints: maxEndpoints,
	}
}

func getRedisAddr() string {
	addr := os.Getenv(envVarRedisAddr)
	if addr == "" {
		log.Fatalf("required %s env variable is empty\n", envVarRedisAddr)
	}
	return addr
}

func getRedisPass() string {
	pass := os.Getenv(envVarRedisPass)
	if pass == "" {
		log.Fatalf("required %s env variable is empty\n", envVarRedisPass)
	}
	return pass
}

func getRedisDB() int {
	dbStr := os.Getenv(envVarRedisDB)
	if dbStr == "" {
		log.Fatalf("required %s env variable is empty\n", envVarRedisDB)
	}
	db, err := strconv.Atoi(dbStr)
	if err != nil {
		log.Fatalf("failed to convert %s env variable to int. %s=%v. Err=%v", envVarRedisDB, envVarRedisDB, dbStr, err)
	}
	return db
}
